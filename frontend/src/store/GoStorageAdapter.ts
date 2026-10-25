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
  Op,
  ProtocolError,
  type Response,
  Status,
} from "./protocol";

/** A storage request that Go refused or could not carry out. */
export class StoreError extends Error {
  override name = "StoreError";
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

interface Pending {
  resolve: (r: Response) => void;
  reject: (e: Error) => void;
}

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
  #external = new Set<(rel: string) => void>();
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
    expectOk(await this.#call(Op.Save, key, data));
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

  /** Writes a project file (a projection of the document). Rejects if it was edited outside. */
  async writeProjection(name: string, text: string): Promise<void> {
    expectOk(await this.#call(Op.WriteProjection, [name], textEncoder.encode(text)));
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

  /** Tells Go the outside edit of `name` was dealt with, so Inkwell may write it again. */
  async acknowledge(name: string): Promise<void> {
    expectOk(await this.#call(Op.Acknowledge, [name]));
  }

  /** Calls `cb` with the name of each project file edited outside Inkwell. Returns an unsubscribe. */
  onExternal(cb: (name: string) => void): () => void {
    this.#external.add(cb);
    return () => this.#external.delete(cb);
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
        this.#failAll(new StoreError("store stream closed"));
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
    pending.resolve(r);
  }

  #failAll(err: Error): void {
    const all = [...this.#pending.values()];
    this.#pending.clear();
    for (const p of all) p.reject(err);
  }

  async #call(op: Op, key: string[], data: Uint8Array = new Uint8Array()): Promise<Response> {
    const socket = await this.#connect();
    this.#nextId = (this.#nextId % 0xffffffff) + 1; // never 0: that id is Go's pushes
    const id = this.#nextId;
    const reply = new Promise<Response>((resolve, reject) => {
      this.#pending.set(id, { resolve, reject });
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

function expectOk(r: Response): void {
  if (r.status !== Status.OK) throw failure(r);
}

function failure(r: Response): StoreError {
  if (r.status === Status.Binary) return new NotTextError("binary");
  if (r.status === Status.InvalidText) {
    return new NotTextError("invalid-utf8", decodeInvalidLine(r.payload));
  }
  if (r.status === Status.Error) return new StoreError(textDecoder.decode(r.payload));
  return new StoreError(`unexpected status ${r.status}`);
}
