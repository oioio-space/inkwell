import type { SocketLike } from "../net/WailsNetworkAdapter";
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

const textEncoder = new TextEncoder();

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
    const keyless = req.op === Op.LoadRange || req.op === Op.ListFiles;
    if (!req.key.every(validComponent) || (req.key.length === 0 && !keyless)) {
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

  #notText(name: string): [Status, Uint8Array?] | undefined {
    if (this.binary.has(name)) return [Status.Binary];
    const line = this.invalidText.get(name);
    if (line === undefined) return undefined;
    return [Status.InvalidText, Uint8Array.from([0, 0, (line >> 8) & 0xff, line & 0xff])];
  }

  #list(): ProjectFile[] {
    return [...this.files]
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
