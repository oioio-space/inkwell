import { isValidAutomergeUrl, type Message, type PeerId } from "@automerge/automerge-repo";
import { decode } from "cbor-x";
import { MARKER, parseWire } from "../present/wire";

export type Handshake = {
  type: "arrive" | "welcome";
  senderId: PeerId;
  peerMetadata: { isEphemeral: true };
  projectUrl?: import("@automerge/automerge-repo").AutomergeUrl;
};
export const MAX_PROJECT_URL_LENGTH = 80;
type Context = { from: string; me: string; announced: boolean; has: (id: string) => boolean };
const plain = (v: unknown): v is Record<string, unknown> =>
  v !== null &&
  typeof v === "object" &&
  Object.getPrototypeOf(v) === Object.prototype &&
  !Object.hasOwn(v, "__proto__") &&
  // cbor-x renames the dangerous key when decoding maps.
  !Object.hasOwn(v, "__proto_");
const own = (v: Record<string, unknown>, k: string): unknown =>
  Object.hasOwn(v, k) ? v[k] : undefined;
const id = (v: unknown): v is string => typeof v === "string" && v.length > 0 && v.length <= 64;
function ephemeralPayload(data: Uint8Array): boolean {
  const envelope: unknown = decode(data);
  if (!plain(envelope)) return false;
  if (Object.hasOwn(envelope, MARKER)) {
    const message = own(envelope, MARKER);
    return Object.keys(envelope).length === 1 && plain(message) && parseWire(message) !== undefined;
  }
  const p = own(envelope, "__presence");
  if (!plain(p)) return false;
  const channel = (k: string, v: unknown) =>
    ["name", "anchor", "head"].includes(k) &&
    typeof v === "string" &&
    v.length <= (k === "name" ? 128 : 64);
  switch (own(p, "type")) {
    case "heartbeat":
    case "goodbye":
      return true;
    case "update":
      return typeof own(p, "channel") === "string" && channel(p.channel as string, own(p, "value"));
    case "snapshot": {
      const s = own(p, "state");
      return plain(s) && Object.entries(s).every(([k, v]) => channel(k, v));
    }
    default:
      return false;
  }
}
/** Decode untrusted bytes without retaining extra fields or peer metadata. */
export function parsePeerMessage(payload: Uint8Array, c: Context): Handshake | Message | undefined {
  try {
    const v: unknown = decode(payload);
    if (!plain(v)) return;
    const senderId = own(v, "senderId");
    const type = own(v, "type");
    if (!id(senderId) || senderId !== c.from) return;
    if (type === "arrive" || type === "welcome") {
      if (payload.length > 1024) return;
      const metadata = own(v, "peerMetadata");
      if (metadata !== undefined && !plain(metadata)) return;
      const url = own(v, "projectUrl");
      if (
        url !== undefined &&
        (typeof url !== "string" ||
          url.length > MAX_PROJECT_URL_LENGTH ||
          !isValidAutomergeUrl(url))
      )
        return;
      return {
        type,
        senderId: senderId as PeerId,
        peerMetadata: { isEphemeral: true },
        ...(url ? { projectUrl: url as NonNullable<Handshake["projectUrl"]> } : {}),
      };
    }
    if (!["sync", "request", "ephemeral", "doc-unavailable"].includes(type as string)) return;
    const targetId = own(v, "targetId"),
      documentId = own(v, "documentId");
    if (!c.announced || !id(targetId) || targetId !== c.me || !id(documentId) || !c.has(documentId))
      return;
    const base = { type, senderId, targetId, documentId };
    if (type === "doc-unavailable") return base as Message;
    const data = own(v, "data");
    if (!(data instanceof Uint8Array)) return;
    if (type !== "ephemeral") return { ...base, data } as Message;
    const sessionId = own(v, "sessionId"),
      count = own(v, "count");
    if (
      typeof sessionId !== "string" ||
      !/^[0-9a-z]{1,16}$/.test(sessionId) ||
      !Number.isSafeInteger(count) ||
      (count as number) < 1 ||
      data.length > 1024 ||
      !ephemeralPayload(data)
    )
      return;
    return { ...base, data, sessionId, count } as unknown as Message;
  } catch {
    return;
  }
}
type Bucket = { tokens: number; at: number };
type Peer = {
  ephemeral: Bucket;
  arrive: Bucket;
  session?: string;
  count: number;
  pending?: unknown;
  queue: Message[];
  bytes: number;
};
/** Per authenticated peer budgets and cooldowns outlive link resets. */
export class Admission {
  #peers = new Map<string, Peer>();
  #cooldowns = new Map<string, { until: number; delay: number }>();

  #members = new Set<string>();
  /** Replace the signed member device set; departure also erases cooldowns. */
  setMembers(members: Iterable<string>): void {
    this.#members = new Set(members);
    for (const id of this.#peers.keys()) {
      if (!this.#members.has(id)) {
        this.#peers.delete(id);
        this.#cooldowns.delete(id);
      }
    }
  }

  #count = 0;
  constructor(
    private now: () => number = Date.now,
    private sessionId = "admission",
  ) {}
  #peer(id: string): Peer | undefined {
    if (!this.#members.has(id)) return;
    let p = this.#peers.get(id);
    if (!p) {
      p = {
        ephemeral: { tokens: 128, at: this.now() },
        arrive: { tokens: 3, at: this.now() },
        count: 0,
        queue: [],
        bytes: 0,
      };
      this.#peers.set(id, p);
    }
    return p;
  }
  #take(b: Bucket, max: number, rate: number): boolean {
    const n = this.now();
    b.tokens = Math.min(max, b.tokens + (n - b.at) * rate);
    b.at = n;
    if (b.tokens < 1) return false;
    b.tokens--;
    return true;
  }
  retryAfter(id: string): number {
    return Math.max(0, (this.#cooldowns.get(id)?.until ?? 0) - this.now());
  }
  blocked(id: string): boolean {
    return (this.#cooldowns.get(id)?.until ?? 0) > this.now();
  }
  forget(id: string): void {
    const p = this.#peers.get(id);
    if (!p) return;
    p.pending = undefined;
    this.reset(id);
  }
  clearQueues(): void {
    for (const p of this.#peers.values()) {
      p.queue = [];
      p.bytes = 0;
    }
  }
  ephemeral(id: string, msg: Message): Message | undefined {
    const p = this.#peer(id);
    if (!p) return;
    const m = msg as Message & { sessionId: string; count: number };
    if (p.session === m.sessionId && m.count <= p.count) return;
    if (!this.#take(p.ephemeral, 128, 0.064)) return;
    p.session = m.sessionId;
    p.count = m.count;
    return { ...msg, sessionId: this.sessionId, count: ++this.#count } as unknown as Message;
  }
  arrive(id: string, hs: unknown): boolean {
    if (this.blocked(id)) return false;
    const p = this.#peer(id);
    if (!p) return false;
    if (!this.#take(p.arrive, 3, 1 / 20000)) {
      p.pending = hs;
      return false;
    }
    p.pending = undefined;
    return true;
  }
  pending(id: string): unknown {
    const p = this.#peers.get(id);
    if (!p?.pending || this.blocked(id) || !this.#take(p.arrive, 3, 1 / 20000)) return;
    const hs = p.pending;
    p.pending = undefined;
    return hs;
  }
  reset(id: string): void {
    const p = this.#peers.get(id);
    if (!p) return;
    delete p.session;
    p.count = 0;
    p.queue = [];
    p.bytes = 0;
  }
  hold(id: string, msg: Message): boolean {
    if (this.blocked(id)) return false;
    const p = this.#peer(id);
    if (!p) return false;
    const bytes = msg.data?.byteLength ?? 0;
    if (
      p.queue.filter((m) => m.documentId === msg.documentId).length >= 4 ||
      (p.queue.length > 0 && p.bytes + bytes > 8 * 1024 * 1024) ||
      bytes > 64 * 1024 * 1024
    ) {
      p.queue = [];
      p.bytes = 0;
      const delay = Math.min(600000, (this.#cooldowns.get(id)?.delay ?? 15000) * 2);
      // Every cooldown belongs to a member and is pruned on departure.
      this.#cooldowns.set(id, { until: this.now() + delay, delay });
      return false;
    }
    p.queue.push(msg);
    p.bytes += bytes;
    return true;
  }
  held(id: string): number {
    return this.#peers.get(id)?.queue.length ?? 0;
  }
  drain(id: string, ready: (doc: string) => boolean): Message[] {
    const p = this.#peers.get(id);
    if (!p) return [];
    const out: Message[] = [];
    p.queue = p.queue.filter((m) => {
      if (m.documentId && ready(m.documentId)) {
        out.push(m);
        p.bytes -= m.data?.byteLength ?? 0;
        return false;
      }
      return true;
    });
    return out;
  }
}
