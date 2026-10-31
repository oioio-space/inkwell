import {
  type DocHeads,
  decodeEnvelope,
  decodeHeads,
  encodeEnvelope,
  encodeHeads,
  encodeRoles,
  Kind,
} from "./envelope";
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

/** What a device may do in the simulated project, as in Go's member list. */
export type Role = "writer" | "reader";

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
      net.announceState(this);
      net.announceMembers();
      for (const other of net.linkedTo(peer)) this.deliver(Kind.PeerUp, other);
    });
  }
  send(data: Uint8Array): void {
    const env = decodeEnvelope(data);
    if (env.kind === Kind.Message) this.net.route(this.peer, env.peer, env.payload.slice());
    else if (env.kind === Kind.Attest) this.net.attest(this.peer, decodeHeads(env.payload));
    // Go drops every other kind from the webview.
  }
  close(): void {
    this.net.detach(this);
  }
  deliver(kind: Kind, peer: string, payload: Uint8Array = new Uint8Array()): void {
    const frame = encodeEnvelope({ kind, peer, payload });
    this.onmessage?.(new MessageEvent("message", { data: frame.buffer }));
  }
}

/** The attestations a device holds: (document, signer) → latest heads. */
type Known = Map<string, { signer: string; heads: DocHeads }>;

/**
 * An in-memory stand-in for "Go router + transport" between several webviews.
 * Links behave like TCP: ordered and reliable while up; a partition drops what is
 * in flight and each side sees PeerDown — the failure mode the real stack has.
 *
 * It also plays the part of Go's write enforcement (internal/app): a device has a
 * role; only a writer's `Attest` is signed, and the attestation is gossiped to the
 * connected devices ahead of the messages sent afterwards (a single ordered queue
 * per link); `revoke` removes a writer, drops its attestations and tells the others
 * with `Revoked`; a revoked device is refused when it links. Knowledge spreads only
 * along links, as in the real thing, so a device that was offline learns of a
 * revocation when it meets someone who knows.
 */
export class SimNet {
  #sockets = new Map<string, SimSocket>();
  #links = new Map<string, number>(); // "a|b" → epoch
  #epoch = 0;
  #inFlight = 0;
  #roles = new Map<string, Role>();
  #known = new Map<string, Known>(); // device → attestations it holds
  #revoked = new Map<string, Map<string, boolean>>(); // device → writers it knows lost write access → removed from the project too
  #sealed = new Set<string>();
  readonly random: () => number;
  /** Devices whose Attest Go refused (readers, sealed lists), in order. */
  readonly refused: string[] = [];

  constructor(seed: number) {
    this.random = rng(seed);
  }

  socket(peer: string): SocketLike {
    const s = new SimSocket(peer, this);
    this.#sockets.set(peer, s);
    return s;
  }

  /** Sets the role of a device; every device is a writer until told otherwise. */
  setRole(peer: string, role: Role): void {
    this.#roles.set(peer, role);
  }

  announceMembers(): void {
    for (const [id, socket] of this.#sockets) {
      socket.deliver(
        Kind.Roles,
        id,
        encodeRoles(
          [...this.#sockets.keys()]
            .filter((peer) => !this.#revokedFor(id).get(peer))
            .map((peer) => ({ peer, role: this.#roles.get(peer) ?? "writer", suspended: false })),
        ),
      );
    }
  }

  link(a: string, b: string): void {
    const key = SimNet.#key(a, b);
    if (a === b || this.#links.has(key)) return;
    this.#learnRevocations(a, b);
    if (this.#revokedFor(a).get(b) || this.#revokedFor(b).get(a)) return; // refused at the gate
    this.announceMembers();
    this.#links.set(key, ++this.#epoch);
    this.#sockets.get(a)?.deliver(Kind.PeerUp, b);
    this.#sockets.get(b)?.deliver(Kind.PeerUp, a);
    this.#exchangeAttestations(a, b);
  }

  partition(a: string, b: string): void {
    if (!this.#links.delete(SimNet.#key(a, b))) return;
    this.#sockets.get(a)?.deliver(Kind.PeerDown, b);
    this.#sockets.get(b)?.deliver(Kind.PeerDown, a);
  }

  /**
   * An admin, `by`, removes `peer` from the project. Everyone `by` is connected to
   * learns it at once, the others when they meet someone who knows. The removed
   * device is cut off from those who know, and believes it is a writer until it
   * is told.
   */
  remove(peer: string, by: string): void {
    this.#revokeWrite(peer, by, true);
    for (const other of this.linkedTo(peer)) {
      if (this.#revokedFor(other).get(peer)) this.partition(peer, other);
    }
  }

  /**
   * An admin, `by`, makes `peer` a reader. Its attestations are void but it stays
   * connected, and can still send what readers send.
   */
  demote(peer: string, by: string): void {
    this.setRole(peer, "reader");
    this.#revokeWrite(peer, by, false);
  }

  /** Go reports that `peer`'s member list is full: it signs no more. */
  seal(peer: string): void {
    this.#sealed.add(peer);
    this.#sockets.get(peer)?.deliver(Kind.Sealed, peer);
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

  /**
   * Delivers a message to `to` as if `from` had sent it, past the gate. It stands
   * for a frame that was already on its way when the member was removed.
   */
  inject(to: string, from: string, payload: Uint8Array): void {
    this.#inFlight++;
    setTimeout(() => {
      this.#inFlight--;
      this.#sockets.get(to)?.deliver(Kind.Message, from, payload);
    }, 0);
  }

  /** Go signs the heads of a writer and gossips them; it refuses anyone else. */
  attest(from: string, h: DocHeads): void {
    if (this.#roles.get(from) === "reader" || this.#sealed.has(from)) {
      this.refused.push(from);
      return;
    }
    for (const member of this.#component(from)) this.#hold(member, from, h);
  }

  /** What a webview that just attached is told: the attestations its device holds. */
  announceState(s: SimSocket): void {
    for (const { signer, heads } of this.#knownBy(s.peer).values()) {
      s.deliver(Kind.Attested, signer, encodeHeads(heads));
    }
    if (this.#sealed.has(s.peer)) s.deliver(Kind.Sealed, s.peer);
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

  // — Go's side —

  #knownBy(peer: string): Known {
    let k = this.#known.get(peer);
    if (!k) {
      k = new Map();
      this.#known.set(peer, k);
    }
    return k;
  }

  #revokedFor(peer: string): Map<string, boolean> {
    let r = this.#revoked.get(peer);
    if (!r) {
      r = new Map();
      this.#revoked.set(peer, r);
    }
    return r;
  }

  #revokeWrite(peer: string, by: string, removed: boolean): void {
    for (const member of this.#component(by)) {
      if (member !== peer || !removed) this.#learnRevoked(member, peer, removed);
    }
  }

  /** The devices reachable from `peer` over links, itself included. */
  #component(peer: string): string[] {
    const seen = new Set([peer]);
    for (const p of seen) for (const q of this.linkedTo(p)) seen.add(q);
    return [...seen];
  }

  /** `holder` stores the attestation and its webview is told, in order with the messages. */
  #hold(holder: string, signer: string, heads: DocHeads): void {
    if (this.#revokedFor(holder).has(signer)) return;
    const known = this.#knownBy(holder);
    const key = `${heads.doc}|${signer}`;
    const have = known.get(key)?.heads.heads.join();
    if (have === heads.heads.join()) return;
    known.set(key, { signer, heads });
    this.#inFlight++;
    setTimeout(() => {
      this.#inFlight--;
      this.#sockets.get(holder)?.deliver(Kind.Attested, signer, encodeHeads(heads));
    }, 0);
  }

  #learnRevoked(member: string, peer: string, removed: boolean): void {
    const revoked = this.#revokedFor(member);
    if (revoked.has(peer)) return;
    revoked.set(peer, removed);
    const known = this.#knownBy(member);
    for (const [k, v] of known) if (v.signer === peer) known.delete(k);
    this.#sockets.get(member)?.deliver(Kind.Revoked, peer);
  }

  /** Two devices that meet share what they know of the revocations. */
  #learnRevocations(a: string, b: string): void {
    for (const [x, removed] of [...this.#revokedFor(b)]) {
      if (x !== a || !removed) this.#learnRevoked(a, x, removed);
    }
    for (const [x, removed] of [...this.#revokedFor(a)]) {
      if (x !== b || !removed) this.#learnRevoked(b, x, removed);
    }
  }

  /** On connection each side gossips the attestations it holds (Go's sendAttestations). */
  #exchangeAttestations(a: string, b: string): void {
    const fromA = [...this.#knownBy(a).values()];
    const fromB = [...this.#knownBy(b).values()];
    for (const { signer, heads } of fromA)
      for (const m of this.#component(b)) this.#hold(m, signer, heads);
    for (const { signer, heads } of fromB)
      for (const m of this.#component(a)) this.#hold(m, signer, heads);
  }

  static #key(a: string, b: string): string {
    return a < b ? `${a}|${b}` : `${b}|${a}`;
  }
}
