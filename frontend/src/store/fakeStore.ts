import type { SocketLike } from "../net/WailsNetworkAdapter";
import {
  decodeRequest,
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
  /** Names refused by writeProjection, as Go does for a file edited outside Inkwell. */
  readonly locked = new Set<string>();
  /** Keys (joined by "/") that load as damaged and unrecoverable. */
  readonly corrupt = new Set<string>();
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
    if (!req.key.every(validComponent) || (req.key.length === 0 && req.op !== Op.LoadRange)) {
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
          return [Status.Error, textEncoder.encode("project: file edited outside Inkwell")];
        }
        this.files.set(path, req.data.slice());
        return [Status.OK];
      case Op.ReadProjection: {
        const f = this.files.get(path);
        return f ? [Status.OK, f] : [Status.NotFound];
      }
      case Op.Acknowledge:
        this.acknowledged.push(path);
        this.locked.delete(path);
        return [Status.OK];
      default:
        return [Status.Error, textEncoder.encode("unknown op")];
    }
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
