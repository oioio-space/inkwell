/**
 * Payloads of the `store` stream's ListFiles and FileInfo operations (mirror of
 * internal/bridge/files.go; both are checked against internal/bridge/testdata/storestream.json).
 * Integers are big-endian. Both start with a version byte, so a webview and a Go side of
 * different ages fail cleanly instead of misreading each other.
 *
 *   file list: version u8 | count u32 | count × (kind u8 | flags u8 | size u64 | len u16 | path)
 *   file info: version u8 | flags u8 | eol u8 | len u8 | encoding
 */
import { ProtocolError } from "./protocol";

const FILES_VERSION = 1;
const MAX_PATH = 0xffff;
const ENTRY_HEAD = 12;

const textEncoder = new TextEncoder();
const textDecoder = new TextDecoder("utf-8", { fatal: true });

/** Kinds of project file, in the order of the Go type project.Kind. */
export const FileKind = {
  other: 0,
  typst: 1,
  markdown: 2,
  text: 3,
  csv: 4,
  bib: 5,
  image: 6,
  pdf: 7,
  data: 8,
} as const;
export type FileKind = (typeof FileKind)[keyof typeof FileKind];

/** An entry of the project's file list. */
export interface ProjectFile {
  /** Relative to the project folder, with forward slashes. */
  path: string;
  kind: FileKind;
  size: number;
  /** The project's main document. */
  main: boolean;
}

/** Line endings Go writes back. */
export const Eol = { LF: 0, CRLF: 1, CR: 2 } as const;
export type Eol = (typeof Eol)[keyof typeof Eol];

/** How a text file is stored on disk; the editor itself always sees UTF-8 with LF. */
export interface FileInfo {
  /** "utf-8", "utf-16le", "utf-16be" or "windows-1252". */
  encoding: string;
  /** The line ending written back (the most frequent one if `mixed`). */
  eol: Eol;
  bom: boolean;
  /** The file mixed line endings, which saving unifies. */
  mixed: boolean;
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

/** Encodes the payload of a ListFiles response. */
export function encodeFileList(files: ProjectFile[]): Uint8Array {
  const first = new Uint8Array(5);
  first[0] = FILES_VERSION;
  new DataView(first.buffer).setUint32(1, files.length);
  const pieces: Uint8Array[] = [first];
  for (const f of files) {
    const path = textEncoder.encode(f.path);
    if (path.length === 0 || path.length > MAX_PATH) throw new ProtocolError("bad file path");
    const head = new Uint8Array(ENTRY_HEAD);
    const view = new DataView(head.buffer);
    head[0] = f.kind;
    head[1] = f.main ? 1 : 0;
    view.setBigUint64(2, BigInt(f.size));
    view.setUint16(10, path.length);
    pieces.push(head, path);
  }
  return concat(pieces);
}

/** Decodes the payload of a ListFiles response. */
export function decodeFileList(b: Uint8Array): ProjectFile[] {
  if (b.length < 5 || b[0] !== FILES_VERSION) throw new ProtocolError("bad file list header");
  const count = new DataView(b.buffer, b.byteOffset, b.byteLength).getUint32(1);
  const out: ProjectFile[] = [];
  let at = 5;
  for (let i = 0; i < count; i++) {
    if (at + ENTRY_HEAD > b.length) throw new ProtocolError("truncated file entry");
    const view = new DataView(b.buffer, b.byteOffset + at, ENTRY_HEAD);
    const kind = view.getUint8(0);
    const flags = view.getUint8(1);
    const size = view.getBigUint64(2);
    const n = view.getUint16(10);
    at += ENTRY_HEAD;
    if (
      kind > FileKind.data ||
      flags > 1 ||
      size > BigInt(Number.MAX_SAFE_INTEGER) ||
      n === 0 ||
      at + n > b.length
    ) {
      throw new ProtocolError("bad file entry");
    }
    out.push({
      path: decodePath(b.subarray(at, at + n)),
      kind: kind as FileKind,
      size: Number(size),
      main: flags === 1,
    });
    at += n;
  }
  if (at !== b.length) throw new ProtocolError("trailing bytes in file list");
  return out;
}

function decodePath(b: Uint8Array): string {
  try {
    return textDecoder.decode(b);
  } catch {
    throw new ProtocolError("file path is not UTF-8");
  }
}

/** Encodes the payload of a FileInfo response. */
export function encodeFileInfo(i: FileInfo): Uint8Array {
  const name = textEncoder.encode(i.encoding);
  if (name.length > 0xff) throw new ProtocolError("encoding name too long");
  const flags = (i.bom ? 1 : 0) | (i.mixed ? 2 : 0);
  return concat([Uint8Array.from([FILES_VERSION, flags, i.eol, name.length]), name]);
}

/** Decodes the payload of a FileInfo response. */
export function decodeFileInfo(b: Uint8Array): FileInfo {
  const flags = b[1] ?? 0;
  const eol = b[2] ?? 0;
  if (
    b.length < 4 ||
    b[0] !== FILES_VERSION ||
    flags > 3 ||
    eol > Eol.CR ||
    b.length !== 4 + (b[3] ?? 0)
  ) {
    throw new ProtocolError("bad file info");
  }
  return {
    encoding: decodePath(b.subarray(4)),
    eol: eol as Eol,
    bom: (flags & 1) !== 0,
    mixed: (flags & 2) !== 0,
  };
}

/** The line of the first invalid byte, from the payload of an InvalidText response. */
export function decodeInvalidLine(payload: Uint8Array): number {
  if (payload.length !== 4) throw new ProtocolError("bad invalid-text payload");
  return new DataView(payload.buffer, payload.byteOffset, 4).getUint32(0);
}
