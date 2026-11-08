import { isValidAutomergeUrl, type Message, type PeerId } from "@automerge/automerge-repo";
import { type CBORValue, decodeCBOR, encodeCBOR, InvalidCBOR } from "@inkwell/protocol";
import { MARKER, parseWire } from "../present/wire";

export type Handshake = {
  type: "arrive" | "welcome";
  senderId: PeerId;
  targetId: PeerId;
  peerMetadata: { isEphemeral: true };
  projectUrl?: import("@automerge/automerge-repo").AutomergeUrl;
};
export const MAX_PROJECT_URL_LENGTH = 80;
/** Longest project-relative path, in UTF-8 bytes (project.MaxPathLen on the Go side). */
export const MAX_FILE_PATH_BYTES = 1024;
/**
 * Characters a presence name or file may not hold: controls (Cc: C0, DEL, C1), format characters
 * (Cf: the bidirectional embeddings, overrides and isolates U+202A–U+202E and U+2066–U+2069, the
 * marks U+200E/U+200F, zero-width characters, U+FEFF…) and lone surrogates (Cs, which CBOR cannot
 * carry). They are invisible or reorder text, so one person or file could pass for another in the
 * people list. Property escapes keep the check portable (no ES2024 isWellFormed).
 */
const HIDDEN_CHAR = /[\p{Cc}\p{Cf}\p{Cs}]/u;
/** Longest presence name, in UTF-16 units. */
export const MAX_PRESENCE_NAME = 128;
/** Whether a peer-announced display name is safe to show: short, without hidden characters. */
export function validPresenceName(v: unknown): v is string {
  return typeof v === "string" && v.length <= MAX_PRESENCE_NAME && !HIDDEN_CHAR.test(v);
}
/**
 * Whether a peer-announced file is a plausible project-relative path: Unicode in NFC, at most
 * 1024 bytes, no hidden character (see HIDDEN_CHAR) and no backslash (a separator on Windows), no
 * leading, trailing or doubled "/", and no "." or ".." segment. It is only ever a display and
 * filter key, never a filesystem path. The sender applies the same test before announcing a file.
 */
export function validFilePath(v: unknown): v is string {
  if (typeof v !== "string" || v.length === 0 || v.length > MAX_FILE_PATH_BYTES) return false;
  if (HIDDEN_CHAR.test(v) || v.includes("\\") || v.normalize("NFC") !== v) return false;
  if (new TextEncoder().encode(v).length > MAX_FILE_PATH_BYTES) return false;
  return v.split("/").every((part) => part !== "" && part !== "." && part !== "..");
}
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
/** Whether a presence channel (a key of the person's state) carries an acceptable value. */
function presenceValue(k: string, v: unknown): boolean {
  switch (k) {
    case "name":
      return validPresenceName(v);
    case "file":
      return validFilePath(v);
    case "anchor":
    case "head":
      return typeof v === "string" && v.length <= 64;
    default:
      return false;
  }
}
/**
 * The ephemeral payload to forward: data itself when it is valid, a re-encoded snapshot without its
 * invalid keys (one bad field must not hide the rest of a person's state), or undefined.
 */
function admitEphemeral(data: Uint8Array): Uint8Array | undefined {
  const envelope: unknown = decodeCBOR(data);
  if (!plain(envelope)) return;
  if (Object.hasOwn(envelope, MARKER)) {
    const message = own(envelope, MARKER);
    const ok =
      Object.keys(envelope).length === 1 && plain(message) && parseWire(message) !== undefined;
    return ok ? data : undefined;
  }
  const p = own(envelope, "__presence");
  if (!plain(p)) return;
  switch (own(p, "type")) {
    case "heartbeat":
    case "goodbye":
      return data;
    case "update": {
      const channel = own(p, "channel");
      return typeof channel === "string" && presenceValue(channel, own(p, "value"))
        ? data
        : undefined;
    }
    case "snapshot": {
      const s = own(p, "state");
      if (!plain(s)) return;
      const kept = Object.entries(s).filter(([k, v]) => presenceValue(k, v));
      if (kept.length === Object.keys(s).length) return data;
      // Only the four known keys survive the filter, so no prototype key is rebuilt.
      try {
        return encodeCBOR({
          ...envelope,
          __presence: { ...p, state: Object.fromEntries(kept) },
        } as CBORValue);
      } catch {
        return;
      }
    }
    default:
      return;
  }
}
/** Decode untrusted bytes without retaining extra fields or peer metadata. */
export function parsePeerMessage(payload: Uint8Array, c: Context): Handshake | Message | undefined {
  try {
    let v: unknown;
    try {
      v = decodeCBOR(payload);
    } catch (e) {
      if (e instanceof InvalidCBOR) return;
      throw e;
    }
    if (!plain(v)) return;
    const senderId = own(v, "senderId");
    const type = own(v, "type");
    if (!id(senderId) || senderId !== c.from) return;
    const targetId = own(v, "targetId");
    if (!id(targetId) || targetId !== c.me) return;
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
        targetId: targetId as PeerId,
        peerMetadata: { isEphemeral: true },
        ...(url ? { projectUrl: url as NonNullable<Handshake["projectUrl"]> } : {}),
      };
    }
    if (!["sync", "request", "ephemeral", "doc-unavailable"].includes(type as string)) return;
    const documentId = own(v, "documentId");
    if (!c.announced || !id(documentId) || !c.has(documentId)) return;
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
      data.length > 1024
    )
      return;
    const admitted = admitEphemeral(data);
    if (!admitted) return;
    return { ...base, data: admitted, sessionId, count } as unknown as Message;
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
