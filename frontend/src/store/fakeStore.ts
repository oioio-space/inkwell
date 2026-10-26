import type { SocketLike } from "../net/WailsNetworkAdapter";
import { validatePath } from "../project/names";
import {
  Eol,
  encodeFileInfo,
  encodeFileList,
  type FileInfo,
  FileKind,
  type ProjectFile,
} from "./files";
import {
  decodeRequest,
  decodeWriteAt,
  type Entry,
  encodeRange,
  encodeResponse,
  Op,
  Status,
  validComponent,
} from "./protocol";
import {
  decodeReferenced,
  decodeTree,
  encodeBlobInfo,
  encodeTreeResult,
  NodeType,
  type TreeEntry,
} from "./tree";

const textEncoder = new TextEncoder();

/** The operations that name a project file by a path. */
const PATH_OPS: ReadonlySet<number> = new Set([
  Op.WriteProjection,
  Op.ReadProjection,
  Op.Acknowledge,
  Op.FileInfo,
  Op.PutBlob,
]);

/** A stand-in for BLAKE3: 32 bytes that depend on the content only. */
export function fakeHash(bytes: Uint8Array): Uint8Array {
  const out = new Uint8Array(32);
  let h = 0x811c9dc5;
  for (let i = 0; i < 32; i++) {
    for (const b of bytes) h = Math.imul(h ^ b, 0x01000193) >>> 0;
    h = Math.imul(h ^ (i + 1), 0x01000193) >>> 0;
    out[i] = h & 0xff;
  }
  return out;
}

/**
 * An in-memory stand-in for the Go side of the `store` stream, for tests: it speaks the wire
 * protocol (answering one request at a time, in order) over a plain map, the way
 * internal/store and internal/project do.
 */
export class FakeGoStore {
  readonly values = new Map<string, Uint8Array>();
  readonly files = new Map<string, Uint8Array>();
  readonly acknowledged: string[] = [];
  /** The sync point recorded with each file's hash, by name. */
  readonly points = new Map<string, Uint8Array>();
  /** Names refused by writeProjection, as Go does for a file edited outside Inkwell. */
  readonly locked = new Set<string>();
  /** Keys (joined by "/") that load as damaged and unrecoverable. */
  readonly corrupt = new Set<string>();
  /** How each file is stored on disk, when not UTF-8 with LF (see FileInfo). */
  readonly info = new Map<string, FileInfo>();
  /** Files that are binary: reading them answers Binary. */
  readonly binary = new Set<string>();
  /** Files with invalid UTF-8, by 1-based line of the first bad byte. */
  readonly invalidText = new Map<string, number>();
  /** Every request received, in order: [op, key joined by "/"]. */
  readonly log: [number, string][] = [];
  /** The last project tree Tree gave, and the hashes Referenced gave. */
  tree: TreeEntry[] = [];
  referenced: Uint8Array[] = [];
  /** How many Tree and Referenced requests came. */
  treeCalls = 0;
  referencedCalls = 0;
  /** How many entries Tree refuses (Go checks the names itself). */
  rejectOnTree = 0;
  /**
   * Like Go, refuse to write a file the tree does not list as text (and main.typ before any
   * tree). Off by default, so tests of the single-file projection need no tree.
   */
  strictOwnership = false;
  /** Files that are on disk but not in the project, with their bytes: what PutBlob reads. */
  readonly disk = new Map<string, Uint8Array>();
  /** The size limit of a blob; PutBlob answers TooLarge past it. */
  blobLimit = 100 * 1024 * 1024;
  #sockets = new Set<FakeSocket>();

  /** Opens a connection, like `Stream("store")`. */
  socket(): SocketLike {
    const s = new FakeSocket(this);
    this.#sockets.add(s);
    return s;
  }

  /** Pushes the "edited outside Inkwell" notice to every open connection. */
  pushExternal(name: string): void {
    for (const s of this.#sockets) s.deliver(0, Status.External, textEncoder.encode(name));
  }

  /** Closes every connection from the Go side. */
  dropConnections(): void {
    for (const s of this.#sockets) s.close();
    this.#sockets.clear();
  }

  forget(s: FakeSocket): void {
    this.#sockets.delete(s);
  }

  /** Answers one request frame with a status and a payload. */
  handle(frame: Uint8Array): [Status, Uint8Array] {
    const [status, payload = new Uint8Array()] = this.#answer(frame);
    return [status, payload];
  }

  #answer(frame: Uint8Array): [Status, Uint8Array?] {
    const req = decodeRequest(frame);
    const path = req.key.join("/");
    this.log.push([req.op, path]);
    const keyless =
      req.op === Op.LoadRange ||
      req.op === Op.ListFiles ||
      req.op === Op.Tree ||
      req.op === Op.Referenced;
    const byPath = PATH_OPS.has(req.op);
    if (
      (!byPath && !req.key.every(validComponent)) ||
      (byPath && validatePath(path) !== undefined) ||
      (req.key.length === 0 && !keyless)
    ) {
      return [Status.Error, textEncoder.encode("invalid key")];
    }
    switch (req.op) {
      case Op.Load: {
        if (this.corrupt.has(path)) return [Status.Corrupt];
        const v = this.values.get(path);
        return v ? [Status.OK, v] : [Status.NotFound];
      }
      case Op.Save:
        this.values.set(path, req.data.slice());
        return [Status.OK];
      case Op.Remove:
        this.values.delete(path);
        return [Status.OK];
      case Op.LoadRange:
        return [Status.OK, encodeRange(this.#range(req.key))];
      case Op.RemoveRange:
        for (const e of this.#range(req.key)) this.values.delete(e.key.join("/"));
        return [Status.OK];
      case Op.WriteProjection:
        if (this.strictOwnership && !this.#owned(path)) {
          return [Status.Error, textEncoder.encode("not owned")];
        }
        if (this.locked.has(path)) {
          return [Status.EditedOutside];
        }
        this.files.set(path, req.data.slice());
        this.points.delete(path);
        return [Status.OK];
      case Op.WriteProjectionAt: {
        if (this.locked.has(path)) return [Status.EditedOutside];
        const { point, text } = decodeWriteAt(req.data);
        this.files.set(path, text.slice());
        this.points.set(path, point.slice());
        return [Status.OK];
      }
      case Op.SetSyncPoint:
        if (this.locked.has(path)) return [Status.EditedOutside];
        this.points.set(path, req.data.slice());
        return [Status.OK];
      case Op.ReadSyncPoint: {
        const point = this.points.get(path);
        return point?.length ? [Status.OK, point] : [Status.NotFound];
      }
      case Op.Tree:
        this.treeCalls++;
        this.tree = decodeTree(req.data);
        return [Status.OK, encodeTreeResult(this.rejectOnTree)];
      case Op.Referenced:
        this.referencedCalls++;
        this.referenced = decodeReferenced(req.data);
        return [Status.OK];
      case Op.PutBlob: {
        // With data, the bytes come from the webview and must not replace a file that is there.
        if (req.data.length > 0) {
          if (this.disk.has(path) || this.files.has(path)) {
            return [Status.Error, textEncoder.encode("file exists")];
          }
          this.disk.set(path, req.data.slice());
        }
        const bytes = this.disk.get(path);
        if (!bytes) return [Status.NotFound];
        if (bytes.length > this.blobLimit) {
          const limit = new Uint8Array(8);
          new DataView(limit.buffer).setBigUint64(0, BigInt(this.blobLimit));
          return [Status.TooLarge, limit];
        }
        this.files.set(path, bytes);
        return [Status.OK, encodeBlobInfo({ hash: fakeHash(bytes), size: bytes.length })];
      }
      case Op.ReadProjection: {
        const f = this.files.get(path);
        return this.#notText(path) ?? (f ? [Status.OK, f] : [Status.NotFound]);
      }
      case Op.FileInfo:
        if (!this.files.has(path)) return [Status.NotFound];
        return (
          this.#notText(path) ?? [
            Status.OK,
            encodeFileInfo(
              this.info.get(path) ?? { encoding: "utf-8", eol: Eol.LF, bom: false, mixed: false },
            ),
          ]
        );
      case Op.ListFiles:
        return [Status.OK, encodeFileList(this.#list())];
      case Op.Acknowledge:
        this.acknowledged.push(path);
        if (req.data.length > 0) this.points.set(path, req.data.slice());
        else this.points.delete(path);
        this.locked.delete(path);
        return [Status.OK];
      default:
        return [Status.Error, textEncoder.encode("unknown op")];
    }
  }

  /** True when the tree lists `path` as a text file (before any tree, only main.typ). */
  #owned(path: string): boolean {
    if (this.treeCalls === 0) return path === "main.typ";
    return this.tree.some((e) => e.type === NodeType.text && e.path === path);
  }

  #notText(name: string): [Status, Uint8Array?] | undefined {
    if (this.binary.has(name)) return [Status.Binary];
    const line = this.invalidText.get(name);
    if (line === undefined) return undefined;
    return [Status.InvalidText, Uint8Array.from([0, 0, (line >> 8) & 0xff, line & 0xff])];
  }

  #list(): ProjectFile[] {
    return [...new Map([...this.disk, ...this.files])]
      .sort(([a], [b]) => (a < b ? -1 : 1))
      .map(([path, data]) => ({
        path,
        kind: path.endsWith(".typ") ? FileKind.typst : FileKind.text,
        size: data.length,
        main: path === "main.typ",
      }));
  }

  #range(prefix: string[]): Entry[] {
    const out: Entry[] = [];
    for (const [path, data] of [...this.values].sort(([a], [b]) => (a < b ? -1 : 1))) {
      const key = path.split("/");
      if (prefix.every((c, i) => key[i] === c)) out.push({ key, data });
    }
    return out;
  }
}

class FakeSocket implements SocketLike {
  binaryType = "arraybuffer";
  onopen: ((ev: Event) => unknown) | null = null;
  onclose: ((ev: Event) => unknown) | null = null;
  onmessage: ((ev: MessageEvent) => unknown) | null = null;
  #closed = false;

  constructor(private readonly store: FakeGoStore) {
    queueMicrotask(() => this.onopen?.(new Event("open")));
  }

  send(data: Uint8Array): void {
    if (this.#closed) throw new Error("socket closed");
    const frame = data.slice();
    queueMicrotask(() => {
      if (this.#closed) return;
      const id = new DataView(frame.buffer).getUint32(0);
      const [status, payload] = this.store.handle(frame);
      this.deliver(id, status, payload);
    });
  }

  close(): void {
    if (this.#closed) return;
    this.#closed = true;
    this.store.forget(this);
    queueMicrotask(() => this.onclose?.(new Event("close")));
  }

  deliver(id: number, status: Status, payload: Uint8Array): void {
    if (this.#closed) return;
    const frame = encodeResponse({ id, status, payload });
    this.onmessage?.(new MessageEvent("message", { data: frame.buffer }));
  }
}
