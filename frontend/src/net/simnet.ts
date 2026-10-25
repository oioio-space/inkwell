import { decodeEnvelope, encodeEnvelope, Kind } from "./envelope";
import type { SocketLike } from "./WailsNetworkAdapter";

/** Deterministic PRNG (mulberry32) so a failing seed replays exactly. */
export function rng(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

class SimSocket implements SocketLike {
  binaryType = "arraybuffer";
  onopen: ((ev: Event) => unknown) | null = null;
  onclose: ((ev: Event) => unknown) | null = null;
  onmessage: ((ev: MessageEvent) => unknown) | null = null;
  constructor(
    readonly peer: string,
    private readonly net: SimNet,
  ) {
    queueMicrotask(() => {
      this.onopen?.(new Event("open"));
      for (const other of net.linkedTo(peer)) this.deliver(Kind.PeerUp, other);
    });
  }
  send(data: Uint8Array): void {
    const env = decodeEnvelope(data);
    this.net.route(this.peer, env.peer, env.payload.slice());
  }
  close(): void {
    this.net.detach(this);
  }
  deliver(kind: Kind, peer: string, payload: Uint8Array = new Uint8Array()): void {
    const frame = encodeEnvelope({ kind, peer, payload });
    this.onmessage?.(new MessageEvent("message", { data: frame.buffer }));
  }
}

/**
 * An in-memory stand-in for "Go router + transport" between several webviews.
 * Links behave like TCP: ordered and reliable while up; a partition drops what is
 * in flight and each side sees PeerDown — the failure mode the real stack has.
 */
export class SimNet {
  #sockets = new Map<string, SimSocket>();
  #links = new Map<string, number>(); // "a|b" → epoch
  #epoch = 0;
  #inFlight = 0;
  readonly random: () => number;

  constructor(seed: number) {
    this.random = rng(seed);
  }

  socket(peer: string): SocketLike {
    const s = new SimSocket(peer, this);
    this.#sockets.set(peer, s);
    return s;
  }

  link(a: string, b: string): void {
    const key = SimNet.#key(a, b);
    if (a === b || this.#links.has(key)) return;
    this.#links.set(key, ++this.#epoch);
    this.#sockets.get(a)?.deliver(Kind.PeerUp, b);
    this.#sockets.get(b)?.deliver(Kind.PeerUp, a);
  }

  partition(a: string, b: string): void {
    if (!this.#links.delete(SimNet.#key(a, b))) return;
    this.#sockets.get(a)?.deliver(Kind.PeerDown, b);
    this.#sockets.get(b)?.deliver(Kind.PeerDown, a);
  }

  linkedTo(peer: string): string[] {
    return [...this.#links.keys()]
      .map((k) => k.split("|") as [string, string])
      .filter(([a, b]) => a === peer || b === peer)
      .map(([a, b]) => (a === peer ? b : a));
  }

  route(from: string, to: string, payload: Uint8Array): void {
    const key = SimNet.#key(from, to);
    const epoch = this.#links.get(key);
    if (epoch === undefined) return;
    this.#inFlight++;
    // Async but ordered per link: macrotasks run FIFO.
    setTimeout(() => {
      this.#inFlight--;
      if (this.#links.get(key) === epoch)
        this.#sockets.get(to)?.deliver(Kind.Message, from, payload);
    }, 0);
  }

  detach(s: SimSocket): void {
    if (this.#sockets.get(s.peer) === s) this.#sockets.delete(s.peer);
  }

  /** Resolves once nothing has been in flight for a few event-loop turns. */
  async settle(maxMs = 5000): Promise<void> {
    const deadline = Date.now() + maxMs;
    let quiet = 0;
    while (quiet < 5) {
      if (Date.now() > deadline) throw new Error("network did not settle");
      await new Promise((r) => setTimeout(r, 10));
      quiet = this.#inFlight === 0 ? quiet + 1 : 0;
    }
  }

  static #key(a: string, b: string): string {
    return a < b ? `${a}|${b}` : `${b}|${a}`;
  }
}
