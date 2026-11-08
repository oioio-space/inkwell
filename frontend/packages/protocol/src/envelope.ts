// SPDX-License-Identifier: MPL-2.0
/**
 * Binary envelope between the webview and Go over the Wails stream `net`
 * (counterpart of protocol/bridge/envelope.go and kinds.go; both are checked against
 * internal/bridge/testdata/envelopes.json):
 *
 *   version u8 = 1 | kind u8 | len(peer) u8 | peer (ASCII) | payload
 *
 * Webview → Go sends Message (peer = recipient) and Attest (peer ignored). Go →
 * webview sends PeerUp, PeerDown, Message (peer = sender), Attested (peer = signer),
 * Revoked (peer = the revoked writer), Sealed (peer = this device), Roles (peer = this
 * device) and Suspended (peer = the admin who suspended or resumed this device).
 *
 * The payload of Attest and Attested is a document's heads:
 *
 *   len(doc) u16 big endian | doc (UTF-8) | heads, 32 bytes each
 *
 * The payload of Roles lists the devices of the project:
 *
 *   count u16 big endian | per device: len(peer) u8 | peer | role u8 (1 reader, 2 writer,
 *   3 admin) | flags u8 (bit 0: suspended)
 *
 * The payload of Suspended is one byte: 1 if this device was suspended, 0 if it was let back.
 *
 * The payload of Links (Go → webview only, peer = this device) is count u16 big endian, then per
 * peer len(peer) u8 | peer | the figures of `Link`, in order (spec Annex A.1); peers sorted, no
 * duplicate, no network address.
 */

export const Kind = {
  PeerUp: 1,
  PeerDown: 2,
  Message: 3,
  Attest: 4,
  Attested: 5,
  Revoked: 6,
  Sealed: 7,
  Roles: 8,
  Suspended: 9,
  Links: 20,
} as const;
export type Kind = (typeof Kind)[keyof typeof Kind];

export interface Envelope {
  kind: Kind;
  peer: string;
  payload: Uint8Array;
}

export class MalformedEnvelope extends Error {
  override name = "MalformedEnvelope";
}

const VERSION = 1;
const HEADER = 3;
const MAX_PEER = 64;
const HASH_BYTES = 32;
/** Limits shared with Go (bridge.MaxDoc, bridge.MaxHeads). */
export const MAX_DOC = 256;
export const MAX_HEADS = 64;
/** Bound of a Roles payload (bridge.MaxMembers). */
export const MAX_MEMBERS = 1024;

function validPeer(peer: string): boolean {
  if (peer.length === 0 || peer.length > MAX_PEER) return false;
  for (let i = 0; i < peer.length; i++) {
    const c = peer.charCodeAt(i);
    if (c <= 0x20 || c > 0x7e) return false;
  }
  return true;
}

const KINDS: ReadonlySet<number> = new Set(Object.values(Kind));

function carriesPayload(kind: Kind): boolean {
  return (
    kind === Kind.Message ||
    kind === Kind.Attest ||
    kind === Kind.Attested ||
    kind === Kind.Roles ||
    kind === Kind.Suspended ||
    kind === Kind.Links
  );
}

function carriesHeads(kind: Kind): boolean {
  return kind === Kind.Attest || kind === Kind.Attested;
}

function check(kind: number, peer: string, payload: Uint8Array): asserts kind is Kind {
  if (!KINDS.has(kind)) throw new MalformedEnvelope(`unknown kind ${kind}`);
  if (!validPeer(peer)) throw new MalformedEnvelope("invalid peer id");
  if (!carriesPayload(kind as Kind) && payload.length > 0) {
    throw new MalformedEnvelope("this kind carries no payload");
  }
  if (carriesHeads(kind as Kind)) decodeHeads(payload);
  if (kind === Kind.Roles) decodeRoles(payload);
  if (kind === Kind.Suspended) decodeSuspended(payload);
  if (kind === Kind.Links) decodeLinks(payload);
}

export function encodeEnvelope(e: Envelope): Uint8Array {
  check(e.kind, e.peer, e.payload);
  const out = new Uint8Array(HEADER + e.peer.length + e.payload.length);
  out[0] = VERSION;
  out[1] = e.kind;
  out[2] = e.peer.length;
  for (let i = 0; i < e.peer.length; i++) out[HEADER + i] = e.peer.charCodeAt(i);
  out.set(e.payload, HEADER + e.peer.length);
  return out;
}

/** Decodes a frame; the returned payload is a view on `b`, not a copy. */
export function decodeEnvelope(b: Uint8Array): Envelope {
  if (b.length < HEADER) throw new MalformedEnvelope("short frame");
  if (b[0] !== VERSION) throw new MalformedEnvelope(`unsupported version ${b[0]}`);
  const kind = b[1] as number;
  const peerLength = b[2] as number;
  if (b.length < HEADER + peerLength) throw new MalformedEnvelope("truncated peer");
  const peer = String.fromCharCode(...b.subarray(HEADER, HEADER + peerLength));
  const payload = b.subarray(HEADER + peerLength);
  check(kind, peer, payload);
  return { kind, peer, payload };
}

/** The heads of one document, as the Automerge API spells them (hex change hashes). */
export interface DocHeads {
  doc: string;
  heads: string[];
}

const utf8 = new TextEncoder();
const strictUtf8 = new TextDecoder("utf-8", { fatal: true });

function hexToBytes(hex: string): Uint8Array {
  if (hex.length !== HASH_BYTES * 2 || !/^[0-9a-f]+$/.test(hex)) {
    throw new MalformedEnvelope("head is not a change hash");
  }
  const out = new Uint8Array(HASH_BYTES);
  for (let i = 0; i < HASH_BYTES; i++) out[i] = Number.parseInt(hex.slice(2 * i, 2 * i + 2), 16);
  return out;
}

function bytesToHex(b: Uint8Array): string {
  return Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
}

/** Serializes the payload of Attest and Attested. */
export function encodeHeads(h: DocHeads): Uint8Array {
  const doc = utf8.encode(h.doc);
  if (doc.length === 0 || doc.length > MAX_DOC || h.heads.length > MAX_HEADS) {
    throw new MalformedEnvelope("heads out of limits");
  }
  const out = new Uint8Array(2 + doc.length + HASH_BYTES * h.heads.length);
  out[0] = doc.length >> 8;
  out[1] = doc.length & 0xff;
  out.set(doc, 2);
  h.heads.forEach((x, i) => {
    out.set(hexToBytes(x), 2 + doc.length + HASH_BYTES * i);
  });
  return out;
}

/** Parses the payload of Attest and Attested, strictly (same rules as Go). */
export function decodeHeads(b: Uint8Array): DocHeads {
  if (b.length < 2) throw new MalformedEnvelope("truncated heads");
  const n = ((b[0] as number) << 8) | (b[1] as number);
  if (n === 0 || n > MAX_DOC || 2 + n > b.length) throw new MalformedEnvelope("bad document");
  let doc: string;
  try {
    doc = strictUtf8.decode(b.subarray(2, 2 + n));
  } catch {
    throw new MalformedEnvelope("document is not UTF-8");
  }
  const rest = b.subarray(2 + n);
  if (rest.length % HASH_BYTES !== 0 || rest.length / HASH_BYTES > MAX_HEADS) {
    throw new MalformedEnvelope("bad heads");
  }
  const heads: string[] = [];
  for (let i = 0; i < rest.length; i += HASH_BYTES) {
    heads.push(bytesToHex(rest.subarray(i, i + HASH_BYTES)));
  }
  return { doc, heads };
}

/** The role of a device as Go numbers it (trust.Role). */
export const RoleLevel = { reader: 1, writer: 2, admin: 3 } as const;
export type RoleName = keyof typeof RoleLevel;

/** One device of a Roles payload. */
export interface MemberRole {
  peer: string;
  role: RoleName;
  suspended: boolean;
}

const ROLE_NAMES: readonly (RoleName | undefined)[] = [undefined, "reader", "writer", "admin"];

/** Serializes the payload of Roles. */
export function encodeRoles(members: MemberRole[]): Uint8Array {
  if (members.length > MAX_MEMBERS) throw new MalformedEnvelope("too many members");
  const parts: number[] = [members.length >> 8, members.length & 0xff];
  for (const m of members) {
    if (!validPeer(m.peer)) throw new MalformedEnvelope("invalid peer id");
    parts.push(m.peer.length);
    for (let i = 0; i < m.peer.length; i++) parts.push(m.peer.charCodeAt(i));
    parts.push(RoleLevel[m.role], m.suspended ? 1 : 0);
  }
  return Uint8Array.from(parts);
}

/** A cursor over a payload: reads fields in order and throws at the first overrun. */
class Cursor {
  #at = 0;
  constructor(readonly b: Uint8Array) {}

  take(n: number): Uint8Array {
    if (this.#at + n > this.b.length) throw new MalformedEnvelope("truncated");
    this.#at += n;
    return this.b.subarray(this.#at - n, this.#at);
  }

  u8(): number {
    return this.take(1)[0] as number;
  }

  u16(): number {
    const [h, l] = this.take(2) as unknown as [number, number];
    return (h << 8) | l;
  }

  u32(): number {
    const [a, b, c, d] = this.take(4) as unknown as [number, number, number, number];
    return ((a << 24) | (b << 16) | (c << 8) | d) >>> 0;
  }

  /** Reads an entry count and refuses one above `max`. */
  count(max: number, what: string): number {
    const n = this.u16();
    if (n > max) throw new MalformedEnvelope(`too many ${what}`);
    return n;
  }

  /** Reads a peer as len u8 | peer, and refuses an invalid one. */
  peer(): string {
    const peer = String.fromCharCode(...this.take(this.u8()));
    if (!validPeer(peer)) throw new MalformedEnvelope("invalid peer id");
    return peer;
  }

  done(): void {
    if (this.#at !== this.b.length) throw new MalformedEnvelope("trailing bytes");
  }
}

/** Parses the payload of Roles, strictly (same rules as Go). */
export function decodeRoles(b: Uint8Array): MemberRole[] {
  const c = new Cursor(b);
  const out: MemberRole[] = [];
  for (let n = c.count(MAX_MEMBERS, "members"); n > 0; n--) {
    const peer = c.peer();
    const role = ROLE_NAMES[c.u8()];
    const flags = c.u8();
    if (role === undefined || (flags & ~1) !== 0)
      throw new MalformedEnvelope("invalid member entry");
    out.push({ peer, role, suspended: (flags & 1) === 1 });
  }
  c.done();
  return out;
}

/** Parses the payload of Suspended: true if this device was suspended, false if let back. */
export function decodeSuspended(b: Uint8Array): boolean {
  if (b.length !== 1 || (b[0] as number) > 1) throw new MalformedEnvelope("bad suspended payload");
  return b[0] === 1;
}

/** Bound of a Links payload (bridge.MaxLinks). */
export const MAX_LINKS = 1024;
/** lossPermille of a link with too few packets to tell. */
export const LOSS_UNKNOWN = 0xffff;

export type LinkPath = "unknown" | "lan" | "direct" | "relayed";
export type LinkQuality = "unknown" | "good" | "fair" | "poor";
export type LossScope = "none" | "path" | "firstHop";

/** One peer of a Links payload, with the wire's units (instants are ages in ms). */
export interface PeerLink {
  peer: string;
  path: LinkPath;
  quality: LinkQuality;
  pathAgeMs: number;
  upgrades: number;
  fallbacks: number;
  rttMs: number;
  rttVarMs: number;
  rttAgeMs: number;
  lossPermille: number;
  lossScope: LossScope;
  upMs: number;
  downs10m: number;
  downsTotal: number;
  lastDownMs: number;
  resyncs10m: number;
  resyncs: number;
  pendingMsgs: number;
  pendingBytes: number;
  oldestPendingMs: number;
  sendFailures: number;
  resets: number;
}

const PATHS: readonly LinkPath[] = ["unknown", "lan", "direct", "relayed"];
const QUALITIES: readonly LinkQuality[] = ["unknown", "good", "fair", "poor"];
const SCOPES: readonly LossScope[] = ["none", "path", "firstHop"];

/** The numeric fields of a link after its peer, in wire order, with their width in bytes. */
const FIGURES = [
  ["path", 1],
  ["quality", 1],
  ["pathAgeMs", 4],
  ["upgrades", 2],
  ["fallbacks", 2],
  ["rttMs", 4],
  ["rttVarMs", 4],
  ["rttAgeMs", 4],
  ["lossPermille", 2],
  ["lossScope", 1],
  ["upMs", 4],
  ["downs10m", 2],
  ["downsTotal", 4],
  ["lastDownMs", 4],
  ["resyncs10m", 2],
  ["resyncs", 4],
  ["pendingMsgs", 4],
  ["pendingBytes", 4],
  ["oldestPendingMs", 4],
  ["sendFailures", 4],
  ["resets", 4],
] as const;
type Figure = (typeof FIGURES)[number][0];

/** Size of a Link entry without its peer, derived from the field list. */
export const LINK_FIXED = FIGURES.reduce((n, [, width]) => n + width, 0);

/** Parses the payload of Links, strictly (same rules as Go). */
export function decodeLinks(b: Uint8Array): PeerLink[] {
  const c = new Cursor(b);
  const out: PeerLink[] = [];
  for (let n = c.count(MAX_LINKS, "links"); n > 0; n--) {
    const peer = c.peer();
    const f = {} as Record<Figure, number>;
    for (const [name, width] of FIGURES)
      f[name] = width === 1 ? c.u8() : width === 2 ? c.u16() : c.u32();
    const { path, quality, lossScope, ...rest } = f;
    const link = {
      ...rest,
      peer,
      path: PATHS[path],
      quality: QUALITIES[quality],
      lossScope: SCOPES[lossScope],
    };
    if (link.path === undefined || link.quality === undefined || link.lossScope === undefined) {
      throw new MalformedEnvelope("invalid link entry");
    }
    if (out.length > 0 && (out[out.length - 1] as PeerLink).peer >= peer) {
      throw new MalformedEnvelope("links not sorted");
    }
    out.push(link as PeerLink);
  }
  c.done();
  return out;
}
