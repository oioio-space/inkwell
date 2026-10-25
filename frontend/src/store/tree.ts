/**
 * Payloads of the `store` stream's Tree, Referenced and PutBlob operations (mirror of
 * internal/bridge/tree.go; both are checked against internal/bridge/testdata/storestream.json).
 * Integers are big-endian. Each starts with a version byte, as the file payloads do.
 *
 *   tree:       version u8 | count u32 | count × (type u8 | flags u8 | size u64 | len u16 | path | rlen u8 | ref)
 *   tree result: version u8 | rejected u32
 *   referenced: version u8 | count u32 | count × hash (32 bytes)
 *   blob info:  version u8 | hash (32 bytes) | size u64
 *
 * A tree entry's type is 0 for a folder, 1 for a text file and 2 for a blob; flags bit 0 is "main
 * document"; ref is the document URL of a text file, the 32 bytes of the hash of a blob, and empty
 * for a folder. Go checks every path with its own validator before it touches the disk and counts
 * the entries it refuses in the tree result.
 */
import { ProtocolError } from "./protocol";

const VERSION = 1;
const HASH_LEN = 32;
const ENTRY_HEAD = 12;
/** The most entries a tree may have (ADR 0023). */
export const MAX_NODES = 10000;
/** The longest project path, in UTF-8 bytes. */
export const MAX_PATH_BYTES = 1024;
const MAX_URL = 255;

const textEncoder = new TextEncoder();
const textDecoder = new TextDecoder("utf-8", { fatal: true });

/** Types of tree entry, in the order of the Go type project.NodeType. */
export const NodeType = { dir: 0, text: 1, blob: 2 } as const;
export type NodeType = (typeof NodeType)[keyof typeof NodeType];

/** A node of the project tree, as the webview materialized it from the index. */
export interface TreeEntry {
  /** Relative to the project folder, with forward slashes. */
  path: string;
  type: NodeType;
  /** The document URL of a text file. */
  url?: string;
  /** The BLAKE3 hash of a blob, 32 bytes. */
  hash?: Uint8Array;
  /** The length the index gives a blob. */
  size: number;
  /** The project's main document. */
  main: boolean;
}

function concat(pieces: Uint8Array[]): Uint8Array {
  const out = new Uint8Array(pieces.reduce((n, p) => n + p.length, 0));
  let at = 0;
  for (const p of pieces) {
    out.set(p, at);
    at += p.length;
  }
  return out;
}

function decodeUtf8(b: Uint8Array): string {
  try {
    return textDecoder.decode(b);
  } catch {
    throw new ProtocolError("not UTF-8");
  }
}

function reference(e: TreeEntry): Uint8Array {
  switch (e.type) {
    case NodeType.text: {
      const url = textEncoder.encode(e.url ?? "");
      if (url.length === 0 || url.length > MAX_URL) throw new ProtocolError("bad document URL");
      return url;
    }
    case NodeType.blob:
      if (e.hash?.length !== HASH_LEN) throw new ProtocolError("bad blob hash");
      return e.hash;
    default:
      return new Uint8Array();
  }
}

/** Encodes the data of a Tree request. */
export function encodeTree(entries: TreeEntry[]): Uint8Array {
  if (entries.length > MAX_NODES) throw new ProtocolError(`tree of ${entries.length} entries`);
  const first = new Uint8Array(5);
  first[0] = VERSION;
  new DataView(first.buffer).setUint32(1, entries.length);
  const pieces: Uint8Array[] = [first];
  for (const e of entries) {
    const path = textEncoder.encode(e.path);
    if (path.length === 0 || path.length > MAX_PATH_BYTES) throw new ProtocolError("bad tree path");
    if (!Number.isSafeInteger(e.size) || e.size < 0) throw new ProtocolError("bad tree size");
    const ref = reference(e);
    const head = new Uint8Array(ENTRY_HEAD);
    const view = new DataView(head.buffer);
    head[0] = e.type;
    head[1] = e.main ? 1 : 0;
    view.setBigUint64(2, BigInt(e.size));
    view.setUint16(10, path.length);
    pieces.push(head, path, Uint8Array.of(ref.length), ref);
  }
  return concat(pieces);
}

/** Decodes the data of a Tree request. Only Go-side fakes and tests need it. */
export function decodeTree(b: Uint8Array): TreeEntry[] {
  if (b.length < 5 || b[0] !== VERSION) throw new ProtocolError("bad tree header");
  const view = new DataView(b.buffer, b.byteOffset, b.byteLength);
  const count = view.getUint32(1);
  if (count > MAX_NODES) throw new ProtocolError("too many tree entries");
  const out: TreeEntry[] = [];
  let at = 5;
  for (let i = 0; i < count; i++) {
    if (at + ENTRY_HEAD > b.length) throw new ProtocolError("truncated tree entry");
    const type = view.getUint8(at);
    const flags = view.getUint8(at + 1);
    const size = view.getBigUint64(at + 2);
    const n = view.getUint16(at + 10);
    at += ENTRY_HEAD;
    if (
      type > NodeType.blob ||
      flags > 1 ||
      size > BigInt(Number.MAX_SAFE_INTEGER) ||
      n === 0 ||
      n > MAX_PATH_BYTES ||
      at + n + 1 > b.length
    ) {
      throw new ProtocolError("bad tree entry");
    }
    const path = decodeUtf8(b.subarray(at, at + n));
    const rlen = view.getUint8(at + n);
    at += n + 1;
    if (at + rlen > b.length) throw new ProtocolError("truncated tree reference");
    const ref = b.subarray(at, at + rlen);
    at += rlen;
    const entry: TreeEntry = {
      path,
      type: type as NodeType,
      size: Number(size),
      main: flags === 1,
    };
    if (type === NodeType.text && rlen > 0) entry.url = decodeUtf8(ref);
    else if (type === NodeType.blob && rlen === HASH_LEN) entry.hash = ref.slice();
    else if (!(type === NodeType.dir && rlen === 0)) {
      throw new ProtocolError("bad tree reference");
    }
    out.push(entry);
  }
  if (at !== b.length) throw new ProtocolError("trailing bytes in tree");
  return out;
}

/** Encodes the payload of a Tree response: the number of entries Go refused. */
export function encodeTreeResult(rejected: number): Uint8Array {
  const out = new Uint8Array(5);
  out[0] = VERSION;
  new DataView(out.buffer).setUint32(1, rejected);
  return out;
}

/** Decodes the payload of a Tree response: the number of entries Go refused. */
export function decodeTreeResult(b: Uint8Array): number {
  if (b.length !== 5 || b[0] !== VERSION) throw new ProtocolError("bad tree result");
  return new DataView(b.buffer, b.byteOffset, b.byteLength).getUint32(1);
}

/** Encodes the data of a Referenced request: the hashes the index refers to, 32 bytes each. */
export function encodeReferenced(hashes: Uint8Array[]): Uint8Array {
  const out = new Uint8Array(5 + hashes.length * HASH_LEN);
  out[0] = VERSION;
  new DataView(out.buffer).setUint32(1, hashes.length);
  hashes.forEach((h, i) => {
    if (h.length !== HASH_LEN) throw new ProtocolError("bad hash");
    out.set(h, 5 + i * HASH_LEN);
  });
  return out;
}

/** Decodes the data of a Referenced request. */
export function decodeReferenced(b: Uint8Array): Uint8Array[] {
  if (b.length < 5 || b[0] !== VERSION) throw new ProtocolError("bad referenced header");
  const count = new DataView(b.buffer, b.byteOffset, b.byteLength).getUint32(1);
  if (b.length - 5 !== count * HASH_LEN) throw new ProtocolError("bad referenced length");
  return Array.from({ length: count }, (_, i) => b.slice(5 + i * HASH_LEN, 5 + (i + 1) * HASH_LEN));
}

/** The hash and size of a stored blob, the payload of a PutBlob response. */
export interface BlobInfo {
  hash: Uint8Array;
  size: number;
}

/** Encodes the payload of a PutBlob response. */
export function encodeBlobInfo(i: BlobInfo): Uint8Array {
  if (i.hash.length !== HASH_LEN || !Number.isSafeInteger(i.size) || i.size < 0) {
    throw new ProtocolError("bad blob info");
  }
  const out = new Uint8Array(1 + HASH_LEN + 8);
  out[0] = VERSION;
  out.set(i.hash, 1);
  new DataView(out.buffer).setBigUint64(1 + HASH_LEN, BigInt(i.size));
  return out;
}

/** Decodes the payload of a PutBlob response. */
export function decodeBlobInfo(b: Uint8Array): BlobInfo {
  if (b.length !== 1 + HASH_LEN + 8 || b[0] !== VERSION) throw new ProtocolError("bad blob info");
  const view = new DataView(b.buffer, b.byteOffset, b.byteLength);
  const size = view.getBigUint64(1 + HASH_LEN);
  if (size > BigInt(Number.MAX_SAFE_INTEGER)) throw new ProtocolError("bad blob size");
  return { hash: b.slice(1, 1 + HASH_LEN), size: Number(size) };
}

/** The size limit of a blob, in bytes, from the payload of a TooLarge response. */
export function decodeTooLarge(payload: Uint8Array): number {
  if (payload.length !== 8) throw new ProtocolError("bad too-large payload");
  const limit = new DataView(payload.buffer, payload.byteOffset, 8).getBigUint64(0);
  if (limit > BigInt(Number.MAX_SAFE_INTEGER)) throw new ProtocolError("bad too-large limit");
  return Number(limit);
}
