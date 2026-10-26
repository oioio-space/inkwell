import { encodeReferenced, encodeTree, NodeType, type TreeEntry } from "../store/tree";
import type { Materialized } from "./materialize";
import { hashBytes, isHash } from "./model";

/** What the sync needs of the Go side: the two operations of the `store` stream. */
export interface TreeSink {
  setTree(entries: TreeEntry[]): Promise<number>;
  setReferenced(hashes: Uint8Array[]): Promise<void>;
}

/** The tree Go is given, from the materialized index: folders, text files, blobs. */
export function toTreeEntries(m: Materialized): TreeEntry[] {
  const out: TreeEntry[] = [];
  for (const e of m.entries) {
    if (e.type === "dir") {
      out.push({ path: e.path, type: NodeType.dir, size: 0, main: false });
    } else if (e.type === "text" && e.url) {
      out.push({ path: e.path, type: NodeType.text, url: e.url, size: 0, main: e.main });
    } else if (e.type === "blob" && isHash(e.hash)) {
      out.push({
        path: e.path,
        type: NodeType.blob,
        hash: hashBytes(e.hash),
        size: e.size,
        main: false,
      });
    }
  }
  return out;
}

export interface TreeSyncOptions {
  /** Quiet time after a change before the tree is sent; a burst of changes is one request. */
  delayMs?: number;
  /** Called after each Tree request with how many entries Go refused and the tree it was given. */
  onResult?: (rejected: number, sent: Materialized) => void;
  /** Called when a request failed; the next change tries again. */
  onError?: (err: unknown) => void;
}

/**
 * Keeps Go told of the project tree and of the hashes the index refers to (ADR 0023): on every
 * change of the materialized index, after a short quiet time, one `Tree` and one `Referenced`
 * request, one at a time, and none when nothing changed since the last one. Go projects the
 * tree to the folder, brings in the blobs, and serves only what is referenced.
 */
export class TreeSync {
  #sink: TreeSink;
  #opts: TreeSyncOptions;
  #timer: ReturnType<typeof setTimeout> | undefined;
  #latest: Materialized | undefined;
  #running: Promise<void> | undefined;
  #sentTree = "";
  #sentRefs = "";
  #stopped = false;

  constructor(sink: TreeSink, opts: TreeSyncOptions = {}) {
    this.#sink = sink;
    this.#opts = opts;
  }

  /** Schedules sending `m`. */
  update(m: Materialized): void {
    if (this.#stopped) return;
    this.#latest = m;
    clearTimeout(this.#timer);
    this.#timer = setTimeout(() => void this.flush(), this.#opts.delayMs ?? 60);
  }

  /** Sends the latest tree now, if it differs from the last one sent. */
  flush(): Promise<void> {
    clearTimeout(this.#timer);
    const run = async (): Promise<void> => {
      await this.#running?.catch(() => {});
      const m = this.#latest;
      if (!m || this.#stopped) return;
      try {
        const entries = toTreeEntries(m);
        const treeKey = key(encodeTree(entries));
        if (treeKey !== this.#sentTree) {
          const rejected = await this.#sink.setTree(entries);
          this.#sentTree = treeKey;
          this.#opts.onResult?.(rejected, m);
        }
        const hashes = m.refs.map(hashBytes);
        const refsKey = key(encodeReferenced(hashes));
        if (refsKey !== this.#sentRefs) {
          await this.#sink.setReferenced(hashes);
          this.#sentRefs = refsKey;
        }
      } catch (err) {
        this.#opts.onError?.(err);
      }
    };
    const next = run();
    this.#running = next;
    return next;
  }

  /** Forgets what was sent, so the next flush sends everything again (the store reconnected). */
  reset(): void {
    this.#sentTree = "";
    this.#sentRefs = "";
  }

  stop(): void {
    this.#stopped = true;
    clearTimeout(this.#timer);
  }
}

/** A string that stands for the bytes, to tell two payloads apart. */
function key(b: Uint8Array): string {
  let s = "";
  for (let i = 0; i < b.length; i += 0x8000) {
    s += String.fromCharCode(...b.subarray(i, i + 0x8000));
  }
  return s;
}
