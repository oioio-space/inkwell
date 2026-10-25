/**
 * Wire format of the Wails stream `store` (mirror of internal/bridge/storestream.go; both are
 * checked against internal/bridge/testdata/storestream.json). Integers are big-endian.
 *
 *   request:  id u32 | op u8 | nkey u8 | nkey × (len u8 | component) | data
 *   response: id u32 | status u8 | payload
 *
 * Requests are answered one at a time, in order. Go also pushes a response with id 0 and
 * status External: the payload is a project file edited outside Inkwell.
 *
 * ReadProjection and WriteProjection always carry UTF-8 text with LF line ends; Go converts from
 * and to the file's own encoding, BOM and line ending. ListFiles (no key) and FileInfo (the file
 * path) return the versioned payloads defined below.
 *
 * The operations that name a project file (WriteProjection, ReadProjection, Acknowledge, FileInfo,
 * PutBlob) take its path as the key, one component per folder, up to 32. Go refuses any path that
 * is not portable (ADR 0023); the webview's own validator checks the names before it sends them.
 *
 * Tree and Referenced carry the payloads of ./tree.ts: the project tree the webview materialized
 * from the index, and the hashes the index refers to. PutBlob answers with a blob info payload.
 */

export const Op = {
  Load: 1,
  Save: 2,
  Remove: 3,
  LoadRange: 4,
  RemoveRange: 5,
  WriteProjection: 6,
  ReadProjection: 7,
  Acknowledge: 8,
  ListFiles: 9,
  FileInfo: 10,
  Tree: 11,
  Referenced: 12,
  PutBlob: 13,
} as const;
export type Op = (typeof Op)[keyof typeof Op];

/**
 * Binary and InvalidText answer ReadProjection and FileInfo for a file that is not text: the
 * second carries the 1-based line of the first invalid byte as a u32. EditedOutside answers
 * WriteProjection for a file changed outside Inkwell and not yet acknowledged. TooLarge answers
 * PutBlob for a file over the size limit of a blob: its payload is the limit in bytes, as a u64.
 */
export const Status = {
  OK: 0,
  NotFound: 1,
  Corrupt: 2,
  Error: 3,
  External: 4,
  Binary: 5,
  InvalidText: 6,
  EditedOutside: 7,
  TooLarge: 8,
} as const;
export type Status = (typeof Status)[keyof typeof Status];

export interface Request {
  id: number;
  op: Op;
  key: string[];
  data: Uint8Array;
}

export interface Response {
  id: number;
  status: Status;
  payload: Uint8Array;
}

export interface Entry {
  key: string[];
  data: Uint8Array;
}

export class ProtocolError extends Error {
  override name = "ProtocolError";
}

const MAX_DEPTH = 4;
const MAX_COMPONENT = 128;
/** The operations that name a project file by a path in their key. */
const PATH_OPS: ReadonlySet<number> = new Set([
  Op.WriteProjection,
  Op.ReadProjection,
  Op.Acknowledge,
  Op.FileInfo,
  Op.PutBlob,
]);
const MAX_PATH_DEPTH = 32;
const MAX_PATH_COMPONENT = 255;
const textEncoder = new TextEncoder();
const textDecoder = new TextDecoder("utf-8", { fatal: true });

/** A key component: 1 to 128 of [A-Za-z0-9._-], never "." or "..". */
export function validComponent(c: string): boolean {
  return (
    c.length > 0 && c.length <= MAX_COMPONENT && /^[A-Za-z0-9._-]+$/.test(c) && !/^\.\.?$/.test(c)
  );
}

/** A path key: 1 to 32 components of 1 to 255 UTF-8 bytes, none with a slash. */
function appendPathKey(out: number[], key: string[]): void {
  if (key.length === 0 || key.length > MAX_PATH_DEPTH) {
    throw new ProtocolError(`path of ${key.length} components`);
  }
  out.push(key.length);
  for (const c of key) {
    const bytes = textEncoder.encode(c);
    if (bytes.length === 0 || bytes.length > MAX_PATH_COMPONENT || c.includes("/")) {
      throw new ProtocolError(`invalid path component of ${bytes.length} bytes`);
    }
    out.push(bytes.length, ...bytes);
  }
}

function appendKey(out: number[], key: string[]): void {
  if (key.length > MAX_DEPTH) throw new ProtocolError(`key of ${key.length} components`);
  out.push(key.length);
  for (const c of key) {
    if (!validComponent(c)) throw new ProtocolError(`invalid key component of ${c.length} chars`);
    out.push(c.length, ...textEncoder.encode(c));
  }
}

function u32(value: number): number[] {
  return [(value >>> 24) & 0xff, (value >>> 16) & 0xff, (value >>> 8) & 0xff, value & 0xff];
}

const readU32 = (b: Uint8Array, at: number): number =>
  new DataView(b.buffer, b.byteOffset, b.byteLength).getUint32(at);

export function encodeRequest(r: Request): Uint8Array {
  if (!Number.isInteger(r.id) || r.id <= 0 || r.id > 0xffffffff) {
    throw new ProtocolError("request id out of range");
  }
  const head: number[] = [...u32(r.id), r.op];
  if (PATH_OPS.has(r.op)) appendPathKey(head, r.key);
  else if (r.op === Op.Tree || r.op === Op.Referenced) {
    if (r.key.length !== 0) throw new ProtocolError("Tree and Referenced take no key");
    head.push(0);
  } else appendKey(head, r.key);
  const out = new Uint8Array(head.length + r.data.length);
  out.set(head);
  out.set(r.data, head.length);
  return out;
}

/** Decodes a request. Only Go-side fakes and tests need it. */
export function decodeRequest(b: Uint8Array): Request {
  if (b.length < 6) throw new ProtocolError("short request");
  const key: string[] = [];
  const nkey = b[5] as number;
  let at = 6;
  for (let i = 0; i < nkey; i++) {
    const len = b[at];
    if (len === undefined || at + 1 + len > b.length) throw new ProtocolError("truncated key");
    key.push(textDecoder.decode(b.subarray(at + 1, at + 1 + len)));
    at += 1 + len;
  }
  return { id: readU32(b, 0), op: b[4] as Op, key, data: b.subarray(at) };
}

export function encodeResponse(r: Response): Uint8Array {
  const out = new Uint8Array(5 + r.payload.length);
  out.set([...u32(r.id), r.status]);
  out.set(r.payload, 5);
  return out;
}

/** Decodes a frame from Go; the payload is a view on `b`, not a copy. */
export function decodeResponse(b: Uint8Array): Response {
  if (b.length < 5) throw new ProtocolError("short response");
  const status = b[4] as number;
  if (status > Status.TooLarge) throw new ProtocolError(`unknown status ${status}`);
  return { id: readU32(b, 0), status: status as Status, payload: b.subarray(5) };
}

/** Encodes the payload of a LoadRange response. */
export function encodeRange(entries: Entry[]): Uint8Array {
  const pieces: Uint8Array[] = [Uint8Array.from(u32(entries.length))];
  for (const e of entries) {
    const head: number[] = [];
    appendKey(head, e.key);
    head.push(...u32(e.data.length));
    pieces.push(Uint8Array.from(head), e.data);
  }
  const out = new Uint8Array(pieces.reduce((n, p) => n + p.length, 0));
  let at = 0;
  for (const p of pieces) {
    out.set(p, at);
    at += p.length;
  }
  return out;
}

/** Decodes the payload of a LoadRange response; the data are views on `b`. */
export function decodeRange(b: Uint8Array): Entry[] {
  if (b.length < 4) throw new ProtocolError("truncated range");
  const count = readU32(b, 0);
  const out: Entry[] = [];
  let at = 4;
  for (let i = 0; i < count; i++) {
    const nkey = b[at++];
    if (nkey === undefined || nkey > MAX_DEPTH) throw new ProtocolError("bad range entry");
    const key: string[] = [];
    for (let k = 0; k < nkey; k++) {
      const len = b[at];
      if (len === undefined || at + 1 + len > b.length) throw new ProtocolError("truncated range");
      key.push(textDecoder.decode(b.subarray(at + 1, at + 1 + len)));
      at += 1 + len;
    }
    if (at + 4 > b.length) throw new ProtocolError("truncated range");
    const n = readU32(b, at);
    at += 4;
    if (at + n > b.length) throw new ProtocolError("truncated range entry");
    out.push({ key, data: b.subarray(at, at + n) });
    at += n;
  }
  if (at !== b.length) throw new ProtocolError("trailing bytes in range");
  return out;
}
