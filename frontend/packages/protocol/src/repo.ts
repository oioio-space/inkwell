// SPDX-License-Identifier: MPL-2.0
/**
 * automerge-repo message schemas of spec §9.3-§9.6 (cddl/repo.cddl).
 *
 * `decodeRepoMessage` applies the §9.2 validity profile, binds the message to
 * the authenticated sender/receiver and the admitted document set, then checks
 * the schema of its type. An unknown message type is ignored without error
 * (N14); every other violation is an {@link InvalidRepoMessage}.
 */
import { type CBORMap, type CBORValue, decodeCBOR } from "./cbor";

export interface RepoContext {
  /** The peer authenticated by the transport. */
  from: string;
  /** The receiver. */
  me: string;
  /** Documents this receiver currently serves. */
  admitted: readonly string[];
}

/** A message that passed the profile and its §9 schema. */
export interface RepoMessage {
  type: string;
  senderId: string;
  targetId: string;
  projectUrl?: string;
  documentId?: string;
  data?: Uint8Array;
  /** Change hashes of `history-request`, hex encoded. */
  have?: string[];
  max?: number;
  /** Number of changes in a `history-page`. */
  changes?: number;
  last?: boolean;
  sessionId?: string;
  count?: number;
}

export type RepoDecode =
  | { kind: "message"; message: RepoMessage }
  | { kind: "ignored"; type: unknown };

export class InvalidRepoMessage extends Error {
  override name = "InvalidRepoMessage";
}

const PEER = /^[!-~]{1,64}$/;
const DOC = /^[1-9A-HJ-NP-Za-km-z]{1,64}$/;
const URL = /^automerge:[1-9A-HJ-NP-Za-km-z]{1,70}$/;
const SESSION = /^[0-9a-z]{1,16}$/;
const HISTORY_MAX = 4194304;
const KNOWN = new Set([
  "sync",
  "request",
  "doc-unavailable",
  "ephemeral",
  "history-request",
  "history-page",
]);
const SCREENS = new Set(["show", "black", "white"]);
const utf8 = new TextEncoder();

function invalid(reason: string): never {
  throw new InvalidRepoMessage(reason);
}

function isMap(v: CBORValue | undefined): v is CBORMap {
  return v !== null && typeof v === "object" && !Array.isArray(v) && !(v instanceof Uint8Array);
}

function own(v: CBORMap, key: string): CBORValue | undefined {
  return Object.hasOwn(v, key) ? v[key] : undefined;
}

function uint(v: CBORValue | undefined, max: number): v is number {
  return typeof v === "number" && Number.isSafeInteger(v) && v >= 0 && v <= max;
}

function hex(bytes: Uint8Array): string {
  return Array.from(bytes, (x) => x.toString(16).padStart(2, "0")).join("");
}

/** N14: a key whose name starts with "!" is must-understand, at any depth. */
function critical(v: CBORValue): boolean {
  if (Array.isArray(v)) return v.some(critical);
  if (isMap(v)) {
    return Object.keys(v).some((k) => k.startsWith("!")) || Object.values(v).some(critical);
  }
  return false;
}

function channel(key: unknown, value: unknown): boolean {
  if (key !== "name" && key !== "anchor" && key !== "head") return false;
  if (typeof value !== "string") return false;
  return utf8.encode(value).length <= (key === "name" ? 128 : 64);
}

function presence(p: CBORMap): boolean {
  switch (own(p, "type")) {
    case "heartbeat":
    case "goodbye":
      return true;
    case "update":
      return channel(own(p, "channel"), own(p, "value"));
    case "snapshot": {
      const state = own(p, "state");
      if (!isMap(state) || Object.keys(state).length > 3) return false;
      return Object.entries(state).every(([k, v]) => channel(k, v));
    }
    default:
      return false;
  }
}

function present(m: CBORMap): boolean {
  const keys = Object.keys(m).sort().join(",");
  const epoch = (v: CBORValue | undefined) => uint(v, 2147483648);
  const peer = (v: CBORValue | undefined) => typeof v === "string" && PEER.test(v);
  const coord = (v: CBORValue | undefined) => uint(v, 10000);
  switch (own(m, "t")) {
    case "state":
      return (
        keys === "epoch,screen,seq,slide,step,t" &&
        epoch(own(m, "epoch")) &&
        uint(own(m, "seq"), Number.MAX_SAFE_INTEGER) &&
        uint(own(m, "slide"), 100000) &&
        (own(m, "slide") as number) >= 1 &&
        uint(own(m, "step"), 100000) &&
        SCREENS.has(own(m, "screen") as string)
      );
    case "laser":
      return (
        keys === "epoch,t,x,y" &&
        epoch(own(m, "epoch")) &&
        ((own(m, "x") === null && own(m, "y") === null) ||
          (coord(own(m, "x")) && coord(own(m, "y"))))
      );
    case "take":
    case "end":
      return keys === "epoch,t" && epoch(own(m, "epoch"));
    case "hello":
    case "ask":
      return keys === "t";
    case "give":
      return keys === "epoch,t,to" && epoch(own(m, "epoch")) && peer(own(m, "to"));
    case "deny":
      return keys === "t,to" && peer(own(m, "to"));
    default:
      return false;
  }
}

export function decodeRepoMessage(bytes: Uint8Array, ctx: RepoContext): RepoDecode {
  const value = decodeCBOR(bytes);
  if (!isMap(value)) invalid("not a map");
  if (critical(value)) invalid("unknown must-understand key");
  const type = typeof own(value, "type") === "string" ? (own(value, "type") as string) : undefined;
  const senderId = own(value, "senderId");
  const targetId = own(value, "targetId");
  if (typeof senderId !== "string" || senderId !== ctx.from) {
    invalid("senderId is not the authenticated sender");
  }
  if (typeof targetId !== "string" || targetId !== ctx.me) {
    invalid("targetId is not the receiver");
  }
  const out: RepoMessage = { type: type ?? "", senderId, targetId };

  if (type === "arrive" || type === "welcome") {
    if (bytes.length > 1024) invalid("handshake > 1024 bytes");
    const metadata = own(value, "peerMetadata");
    if (metadata !== undefined && !isMap(metadata)) invalid("peerMetadata");
    const url = own(value, "projectUrl");
    if (url !== undefined) {
      if (typeof url !== "string" || !URL.test(url) || url.length > 80) invalid("projectUrl");
      out.type = type;
      out.projectUrl = url;
    }
    return { kind: "message", message: out };
  }
  if (type === "remote-subscription-change" || type === "remote-heads-changed") {
    invalid("type not admitted");
  }
  if (type === undefined || !KNOWN.has(type)) return { kind: "ignored", type };
  out.type = type;

  const documentId = own(value, "documentId");
  if (typeof documentId !== "string" || !DOC.test(documentId)) invalid("documentId");
  if (!ctx.admitted.includes(documentId)) invalid("document not in the admitted set");
  out.documentId = documentId;
  if (type === "doc-unavailable") return { kind: "message", message: out };

  if (type === "history-request") {
    const have = own(value, "have");
    const max = own(value, "max");
    if (
      !Array.isArray(have) ||
      have.length > 256 ||
      !have.every((h) => h instanceof Uint8Array && h.length === 32)
    ) {
      invalid("have");
    }
    if (max !== HISTORY_MAX) invalid("max must be r = 4194304");
    out.have = (have as Uint8Array[]).map(hex);
    out.max = HISTORY_MAX;
    return { kind: "message", message: out };
  }

  if (type === "history-page") {
    const changes = own(value, "changes");
    const last = own(value, "last");
    if (
      !Array.isArray(changes) ||
      changes.length > 256 ||
      !changes.every((x) => x instanceof Uint8Array && x.length >= 1)
    ) {
      invalid("changes");
    }
    if (typeof last !== "boolean") invalid("last");
    if (changes.length === 0 && !last) {
      invalid("empty page must be the last (terminal answer)");
    }
    out.changes = changes.length;
    out.last = last;
    return { kind: "message", message: out };
  }

  const data = own(value, "data");
  if (!(data instanceof Uint8Array)) invalid("data is not a byte string");
  out.data = data;
  if (type !== "ephemeral") return { kind: "message", message: out };

  const sessionId = own(value, "sessionId");
  const count = own(value, "count");
  if (typeof sessionId !== "string" || !SESSION.test(sessionId)) invalid("sessionId");
  if (!uint(count, Number.MAX_SAFE_INTEGER) || count < 1) invalid("count");
  if (data.length > 1024) invalid("ephemeral data > 1024 bytes");

  const envelope = decodeCBOR(data);
  if (!isMap(envelope)) invalid("ephemeral payload");
  if (critical(envelope)) invalid("unknown must-understand key (inner payload)");
  if (Object.keys(envelope).length !== 1) invalid("envelope is a closed map with one key");
  if (Object.hasOwn(envelope, "__present")) {
    const p = envelope.__present;
    if (!isMap(p) || !present(p)) invalid("__present");
  } else {
    const p = own(envelope, "__presence");
    if (!isMap(p) || !presence(p)) invalid("__presence");
  }
  out.sessionId = sessionId;
  out.count = count;
  return { kind: "message", message: out };
}
