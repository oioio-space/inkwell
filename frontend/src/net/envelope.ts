/**
 * Binary envelope between the webview and Go over the Wails stream `net`
 * (mirror of internal/bridge/envelope.go and kinds.go; both are checked against
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
    kind === Kind.Suspended
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

/** Parses the payload of Roles, strictly (same rules as Go). */
export function decodeRoles(b: Uint8Array): MemberRole[] {
  if (b.length < 2) throw new MalformedEnvelope("truncated roles");
  const count = ((b[0] as number) << 8) | (b[1] as number);
  if (count > MAX_MEMBERS) throw new MalformedEnvelope("too many members");
  const out: MemberRole[] = [];
  let at = 2;
  for (let n = 0; n < count; n++) {
    const len = b[at] as number | undefined;
    if (len === undefined || at + 1 + len + 2 > b.length) {
      throw new MalformedEnvelope("truncated member entry");
    }
    const peer = String.fromCharCode(...b.subarray(at + 1, at + 1 + len));
    const role = ROLE_NAMES[b[at + 1 + len] as number];
    const flags = b[at + 2 + len] as number;
    if (!validPeer(peer) || role === undefined || (flags & ~1) !== 0) {
      throw new MalformedEnvelope("invalid member entry");
    }
    out.push({ peer, role, suspended: (flags & 1) === 1 });
    at += 3 + len;
  }
  if (at !== b.length) throw new MalformedEnvelope("trailing bytes");
  return out;
}

/** Parses the payload of Suspended: true if this device was suspended, false if let back. */
export function decodeSuspended(b: Uint8Array): boolean {
  if (b.length !== 1 || (b[0] as number) > 1) throw new MalformedEnvelope("bad suspended payload");
  return b[0] === 1;
}
