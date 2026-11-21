import type { Chunk, StorageAdapterInterface, StorageKey } from "@automerge/automerge-repo";
import type { SocketLike } from "../net/WailsNetworkAdapter";
import {
  decodeFileInfo,
  decodeFileList,
  decodeInvalidLine,
  type FileInfo,
  type ProjectFile,
} from "./files";
import {
  decodeRange,
  decodeResponse,
  encodeRequest,
  encodeWriteAt,
  Op,
  ProtocolError,
  REASON_LEAVING,
  type Response,
  Status,
} from "./protocol";

/** A storage request that Go refused or could not carry out. */
export class StoreError extends Error {
  override name = "StoreError";
}

export class SaveError extends StoreError {
  constructor(
    readonly document: string | undefined,
    readonly failure: unknown,
  ) {
    super(failure instanceof Error ? failure.message : "unconfirmed save");
  }
}

/** A project file that is not text Inkwell can edit: it is binary, or has invalid UTF-8 bytes. */
export class NotTextError extends StoreError {
  override name = "NotTextError";
  constructor(
    readonly reason: "binary" | "invalid-utf8",
    /** For "invalid-utf8", the 1-based line of the first invalid byte. */
    readonly line?: number,
  ) {
    super(reason === "binary" ? "file is not text" : `invalid UTF-8 at line ${line}`);
  }
}

/** A write refused because the project file was edited outside Inkwell and not yet acknowledged. */
export class ExternalEditError extends StoreError {
  override name = "ExternalEditError";
  constructor() {
    super("file edited outside Inkwell");
  }
}

interface Pending {
  resolve: (r: Response) => void;
  reject: (e: Error) => void;
  /** Whether the request writes: one lost with its stream is reported as not saved. */
  write: boolean;
}

/** The requests that write the storage or a project file. */
const WRITES: ReadonlySet<Op> = new Set([
  Op.Save,
  Op.Remove,
  Op.RemoveRange,
  Op.WriteProjection,
  Op.WriteProjectionAt,
  Op.SetSyncPoint,
  Op.PutBlob,
]);

const textDecoder = new TextDecoder();
const textEncoder = new TextEncoder();

/**
 * The automerge-repo storage adapter backed by Go (ADR 0020): chunks live in the project folder,
 * written atomically and checksummed, reached through the Wails stream `store`. Go answers
 * requests one at a time, in order, so a load issued after a save sees it.
 *
 * The same stream carries the project's `.typ` files: {@link writeProjection},
 * {@link readProjection}, and the notices of files edited outside Inkwell ({@link onExternal}).
 */
export class GoStorageAdapter implements StorageAdapterInterface {
  #open: () => SocketLike;
  #conn: Promise<SocketLike> | undefined;
  #pending = new Map<number, Pending>();
  #writes = new Set<Promise<Response>>();
  #external = new Set<(rel: string) => void>();
  #leaving = new Set<() => void>();
  #nextId = 0;

  constructor(open: () => SocketLike) {
    this.#open = open;
    // Connect now, not at the first request: Go pushes the pending external-edit notices on attach.
    this.#connect().catch(() => {});
  }

  async load(key: StorageKey): Promise<Uint8Array | undefined> {
    const r = await this.#call(Op.Load, key);
    if (r.status === Status.OK) return r.payload;
    if (r.status === Status.NotFound) return undefined;
    if (r.status === Status.Corrupt) {
      // Go quarantined the damaged value and had no backup: the repo carries on without it, and
      // its peers send the missing changes again.
      console.warn("store: a damaged value was quarantined:", key.join("/"));
      return undefined;
    }
    throw failure(r);
  }

  async save(key: StorageKey, data: Uint8Array): Promise<void> {
    try {
      expectOk(await this.#call(Op.Save, key, data));
    } catch (error) {
      throw new SaveError(key[0], error);
    }
  }

  /** Store replies still owed to writes started before an explicit Repo.flush. */
  async pendingWrites(): Promise<void> {
    await Promise.all([...this.#writes].map((write) => write.then(expectOk)));
  }

  async remove(key: StorageKey): Promise<void> {
    expectOk(await this.#call(Op.Remove, key));
  }

  async loadRange(keyPrefix: StorageKey): Promise<Chunk[]> {
    const r = await this.#call(Op.LoadRange, keyPrefix);
    expectOk(r);
    return decodeRange(r.payload);
  }

  async removeRange(keyPrefix: StorageKey): Promise<void> {
    expectOk(await this.#call(Op.RemoveRange, keyPrefix));
  }

  /**
   * Writes a project file (a projection of the document). Rejects if it was edited outside. With
   * `point`, Go records it with the hash of what it wrote, in one record: the point where the
   * file and the document agree (see ./syncPoint.ts).
   */
  async writeProjection(name: string, text: string, point?: string): Promise<void> {
    if (point === undefined) {
      expectOk(await this.#call(Op.WriteProjection, [name], textEncoder.encode(text)));
      return;
    }
    const data = encodeWriteAt(textEncoder.encode(point), textEncoder.encode(text));
    expectOk(await this.#call(Op.WriteProjectionAt, [name], data));
  }

  /**
   * Records `point` for a file Inkwell has not written (it opened it in the Ctrl+S mode with the
   * document equal to it). Rejects with {@link ExternalEditError} if the file changed since.
   */
  async setSyncPoint(name: string, point: string): Promise<void> {
    expectOk(await this.#call(Op.SetSyncPoint, [name], textEncoder.encode(point)));
  }

  /** The point recorded with the file's hash by the last write or acknowledgement; undefined if none. */
  async readSyncPoint(name: string): Promise<string | undefined> {
    const r = await this.#call(Op.ReadSyncPoint, [name]);
    if (r.status === Status.NotFound) return undefined;
    expectOk(r);
    return textDecoder.decode(r.payload);
  }

  /**
   * Reads a project file as it is on disk, as UTF-8 text with LF line ends whatever its own
   * encoding; undefined if it does not exist. Rejects with {@link NotTextError} for a file that
   * is not text.
   */
  async readProjection(name: string): Promise<string | undefined> {
    const r = await this.#call(Op.ReadProjection, [name]);
    if (r.status === Status.NotFound) return undefined;
    expectOk(r);
    return textDecoder.decode(r.payload);
  }

  /** Lists the project's files (not Inkwell's own): folders first, then natural order. */
  async listFiles(): Promise<ProjectFile[]> {
    const r = await this.#call(Op.ListFiles, []);
    expectOk(r);
    return decodeFileList(r.payload);
  }

  /**
   * How a project file is stored on disk (encoding, BOM, line ending), for the banner and the
   * conversion offer; undefined if it does not exist. Rejects with {@link NotTextError}.
   */
  async fileInfo(name: string): Promise<FileInfo | undefined> {
    const r = await this.#call(Op.FileInfo, [name]);
    if (r.status === Status.NotFound) return undefined;
    expectOk(r);
    return decodeFileInfo(r.payload);
  }

  /**
   * Tells Go the outside edit of `name` was dealt with, so Inkwell may write it again. With
   * `point`, Go records it with the hash of the file as it is now; without, the file has none.
   */
  async acknowledge(name: string, point?: string): Promise<void> {
    expectOk(
      await this.#call(Op.Acknowledge, [name], point ? textEncoder.encode(point) : undefined),
    );
  }

  /** Calls `cb` with the name of each project file edited outside Inkwell. Returns an unsubscribe. */
  onExternal(cb: (name: string) => void): () => void {
    this.#external.add(cb);
    return () => this.#external.delete(cb);
  }

  /**
   * Calls `cb` each time Go refuses a request because the session is leaving (a project change was
   * requested, rule N-16), and each time Go ends the stream with a write still unanswered (its
   * drain may close the stream before the refusal is out): what was sent is not saved, as far as
   * the page knows. Returns an unsubscribe.
   */
  onLeaving(cb: () => void): () => void {
    this.#leaving.add(cb);
    return () => this.#leaving.delete(cb);
  }

  /** Closes the stream; requests still waiting fail. */
  close(): void {
    const conn = this.#conn;
    this.#conn = undefined;
    conn?.then((socket) => socket.close()).catch(() => {});
    this.#failAll(new StoreError("store closed"));
  }

  #connect(): Promise<SocketLike> {
    if (this.#conn) return this.#conn;
    const socket = this.#open();
    socket.binaryType = "arraybuffer";
    const conn = new Promise<SocketLike>((resolve, reject) => {
      socket.onopen = () => resolve(socket);
      socket.onclose = () => {
        if (this.#conn === conn) this.#conn = undefined; // the next request reconnects
        const lostWrite = [...this.#pending.values()].some((p) => p.write);
        this.#failAll(new StoreError("store stream closed"));
        if (lostWrite) for (const cb of this.#leaving) cb();
        reject(new StoreError("store stream closed"));
      };
    });
    socket.onmessage = (ev) => this.#receive(new Uint8Array(ev.data as ArrayBuffer));
    this.#conn = conn;
    return conn;
  }

  #receive(frame: Uint8Array): void {
    let r: Response;
    try {
      r = decodeResponse(frame);
    } catch (err) {
      console.warn(
        "store: dropped frame:",
        err instanceof ProtocolError ? err.message : "undecodable",
      );
      return;
    }
    if (r.id === 0) {
      if (r.status === Status.External) {
        const name = textDecoder.decode(r.payload);
        for (const cb of this.#external) cb(name);
      }
      return;
    }
    const pending = this.#pending.get(r.id);
    if (!pending) return;
    this.#pending.delete(r.id);
    if (isLeaving(r)) for (const cb of this.#leaving) cb();
    pending.resolve(r);
  }

  #failAll(err: Error): void {
    const all = [...this.#pending.values()];
    this.#pending.clear();
    for (const p of all) p.reject(err);
  }

  #call(op: Op, key: string[], data: Uint8Array = new Uint8Array()): Promise<Response> {
    const reply = this.#request(op, key, data);
    if (WRITES.has(op)) {
      this.#writes.add(reply);
      const remove = () => this.#writes.delete(reply);
      void reply.then(remove, remove);
    }
    return reply;
  }

  async #request(op: Op, key: string[], data: Uint8Array): Promise<Response> {
    const socket = await this.#connect();
    this.#nextId = (this.#nextId % 0xffffffff) + 1; // never 0: that id is Go's pushes
    const id = this.#nextId;
    const reply = new Promise<Response>((resolve, reject) => {
      this.#pending.set(id, { resolve, reject, write: WRITES.has(op) });
    });
    try {
      socket.send(encodeRequest({ id, op, key, data }));
    } catch (err) {
      this.#pending.delete(id);
      throw err;
    }
    return reply;
  }
}

function isLeaving(r: Response): boolean {
  return r.status === Status.Error && textDecoder.decode(r.payload) === REASON_LEAVING;
}

function expectOk(r: Response): void {
  if (r.status !== Status.OK) throw failure(r);
}

function failure(r: Response): StoreError {
  if (r.status === Status.Binary) return new NotTextError("binary");
  if (r.status === Status.InvalidText) {
    return new NotTextError("invalid-utf8", decodeInvalidLine(r.payload));
  }
  if (r.status === Status.EditedOutside) return new ExternalEditError();
  if (r.status === Status.Error) return new StoreError(textDecoder.decode(r.payload));
  return new StoreError(`unexpected status ${r.status}`);
}
