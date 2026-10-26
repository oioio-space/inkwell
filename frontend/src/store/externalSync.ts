import { next as A } from "@automerge/automerge";
import type { DocHandle } from "@automerge/automerge-repo";
import { type Integration, integrate, type Range, type Skipped, undoIntegration } from "./merge";
import type { Projection, Written } from "./projection";
import { safeGet, safeRemove, safeSet } from "./safeStorage";
import type { ExternalPolicy } from "./settings.svelte";

type Doc = { text: string };

/** What the person is told about a file that changed (or vanished) outside Inkwell. */
export type Notice =
  /** The change was taken into the document (or, in a locked zone, set aside). */
  | {
      kind: "merged";
      id: number;
      name: string;
      lines: number;
      /** The added text, and the places text was removed (empty spans), as spans of the document: for the gutter. */
      ranges: Range[];
      /** Changes set aside because they fall in a locked zone. */
      skipped: Skipped[];
    }
  /** The setting is "ask": nothing was done yet. */
  | { kind: "ask"; name: string }
  | { kind: "deleted"; name: string }
  /** The file is not text any more; Inkwell keeps its version. */
  | { kind: "binary"; name: string }
  | { kind: "invalid"; name: string; line: number | undefined };

/** What the controller needs from the storage stream (see GoStorageAdapter). */
export interface SyncStorage {
  /** The file as UTF-8 text with LF line ends; undefined if it is gone. Rejects if not text. */
  readProjection(name: string): Promise<string | undefined>;
  /** Tells Go the outside edit was dealt with, so Inkwell may write the file again. */
  acknowledge(name: string): Promise<void>;
}

/** How a read that is not text failed (see NotTextError). */
export interface NotTextLike {
  reason: "binary" | "invalid-utf8";
  line?: number | undefined;
}

export interface ExternalSyncOptions {
  handle: DocHandle<Doc>;
  /** The file the document is the projection of. */
  name: string;
  storage: SyncStorage;
  projection: () => Projection | undefined;
  policy: () => ExternalPolicy;
  /** True when Inkwell writes only on Ctrl+S: an integration is not written back at once. */
  manual: () => boolean;
  onNotice: (notice: Notice | null) => void;
  /** Whether `err` says the file is not text. */
  notText?: (err: unknown) => NotTextLike | undefined;
  /**
   * Whether the outside change to this span (as of the last sync with the disk) must not be
   * applied: a locked zone. The change is set aside in the notice instead. There is no locked
   * zone yet (ADR 0022); this is where they plug in.
   */
  skip?: ((name: string, range: Range) => boolean) | undefined;
  /** Quiet time before an outside change is read, so a burst of writes is one integration. */
  debounceMs?: number;
  /** Where the point of agreement with the disk is kept between sessions; none: not kept. */
  persistKey?: string;
}

/** A point where the file and the document agreed: the heads, and a fingerprint of the text there. */
interface Base {
  heads: A.Heads;
  fingerprint: number;
}

/** What the debounce waits after the last outside write. */
export const EXTERNAL_DEBOUNCE_MS = 200;

/** cyrb53: a quick 53-bit fingerprint, to tell whether the text at saved heads is the text saved. */
function fingerprint(s: string): number {
  let h1 = 0xdeadbeef;
  let h2 = 0x41c6ce57;
  for (let i = 0; i < s.length; i++) {
    const ch = s.charCodeAt(i);
    h1 = Math.imul(h1 ^ ch, 2654435761);
    h2 = Math.imul(h2 ^ ch, 1597334677);
  }
  h1 = Math.imul(h1 ^ (h1 >>> 16), 2246822507) ^ Math.imul(h2 ^ (h2 >>> 13), 3266489909);
  h2 = Math.imul(h2 ^ (h2 >>> 16), 2246822507) ^ Math.imul(h1 ^ (h1 >>> 13), 3266489909);
  return 4294967296 * (2097151 & h2) + (h1 >>> 0);
}

/**
 * Brings the edits other programs make to the project file into the document (B1 EXT-*).
 *
 * The document remembers the heads where it and the file last agreed (a write of Inkwell's, or a
 * previous integration). An outside edit is made as of those heads and merged, so what the person
 * or others typed since stays. Bursts of writes are one integration; Go has already ignored the
 * echo of Inkwell's own writes.
 */
export class ExternalSync {
  #base: Base | undefined;
  #timer: ReturnType<typeof setTimeout> | undefined;
  #chain: Promise<void> = Promise.resolve();
  #done: Integration | undefined;
  #nextId = 0;
  #typed = false;
  #own = false;
  #stopped = false;
  /** A banner about a file that is gone or not text is showing. */
  #problem = false;
  readonly #o: ExternalSyncOptions;
  readonly #onChange = (payload: { patchInfo: { source: string } }) => {
    if (!this.#own && payload.patchInfo.source === "change") this.#typed = true;
  };

  constructor(options: ExternalSyncOptions) {
    this.#o = options;
    this.#base = this.#load();
    options.handle.on("change", this.#onChange);
  }

  /** The projection wrote `written`: the file and the document agree at those heads. */
  recordWritten(written: Written): void {
    this.#setBase(written.heads, written.text);
  }

  /** Go says `name` was edited (or removed) outside. */
  notify(name: string): void {
    if (name !== this.#o.name) return; // the other files of the project: see the per-file documents
    clearTimeout(this.#timer);
    this.#timer = setTimeout(
      () => this.#enqueue(() => this.#read()),
      this.#o.debounceMs ?? EXTERNAL_DEBOUNCE_MS,
    );
  }

  /** True when Ctrl+Z should take the last integration back: nothing was typed since. */
  get canKeyUndo(): boolean {
    return this.#done !== undefined && !this.#typed;
  }

  /** Takes the last integration back: only what it added and removed; others' typing stays. */
  undo(): void {
    const done = this.#done;
    if (!done) return;
    this.#done = undefined;
    this.#own = true;
    try {
      undoIntegration(this.#o.handle, done);
    } finally {
      this.#own = false;
    }
    this.#o.onNotice(null);
  }

  /** "Ask" answered with "Integrate". */
  integrateAsked(): Promise<void> {
    return this.#enqueue(() => this.#read(true));
  }

  /** "Ask" answered with "Keep Inkwell's version" (or the same for a file that is no longer text). */
  keepOurs(): Promise<void> {
    return this.#enqueue(() => this.#keep());
  }

  /** The file was deleted and the person wants it back. */
  recreate(): Promise<void> {
    return this.#enqueue(async () => {
      await this.#o.storage.acknowledge(this.#o.name);
      const projection = this.#o.projection();
      projection?.hold(false);
      this.#forgetBase();
      this.#o.onNotice(null);
      await projection?.flush(true);
    });
  }

  stop(): void {
    this.#stopped = true;
    clearTimeout(this.#timer);
    this.#o.handle.off("change", this.#onChange);
  }

  #enqueue(job: () => Promise<void>): Promise<void> {
    const next = this.#chain.then(job).catch((err: unknown) => {
      console.warn("external:", err instanceof Error ? err.message : "failed");
    });
    this.#chain = next;
    return next;
  }

  /** Reads the file and deals with what it says. `asked`: the person chose to integrate. */
  async #read(asked = false): Promise<void> {
    if (this.#stopped) return;
    const { name, storage } = this.#o;
    const projection = this.#o.projection();
    // The file may change again between the read and the acknowledgement: read until it is steady.
    for (let attempt = 0; attempt < 3; attempt++) {
      let text: string | undefined;
      try {
        text = await storage.readProjection(name);
      } catch (err) {
        const bad = this.#o.notText?.(err);
        if (!bad) throw err;
        this.#problem = true;
        this.#o.onNotice(
          bad.reason === "binary"
            ? { kind: "binary", name }
            : { kind: "invalid", name, line: bad.line },
        );
        return;
      }
      if (text === undefined) {
        projection?.hold(true); // the text stays in Inkwell; it is not written back unasked
        this.#problem = true;
        this.#o.onNotice({ kind: "deleted", name });
        return;
      }
      projection?.hold(false);
      if (this.#problem) {
        this.#problem = false; // the file is back, or is text again
        this.#o.onNotice(null);
      }
      const policy = asked ? "import" : this.#o.policy();
      if (policy === "ask") {
        this.#o.onNotice({ kind: "ask", name });
        return;
      }
      if (policy === "keep") {
        await this.#keep();
        return;
      }
      await this.#merge(text);
      const again = await storage.readProjection(name).catch(() => text);
      if (again === text) return;
    }
  }

  /** The outside text is not taken: the file is written from the document again (or, with Ctrl+S, at the next save). */
  async #keep(): Promise<void> {
    await this.#o.storage.acknowledge(this.#o.name);
    const projection = this.#o.projection();
    projection?.hold(false);
    // The file holds the outside text until Inkwell writes: the point of agreement is gone.
    this.#forgetBase();
    this.#o.onNotice(null);
    if (!this.#o.manual()) await projection?.flush(true);
  }

  async #merge(text: string): Promise<void> {
    const { handle, name, storage } = this.#o;
    const doc = handle.doc();
    if (!doc) return;
    const base = this.#baseHeads(doc);
    this.#own = true;
    let done: Integration | undefined;
    try {
      done = integrate(handle, base, text, {
        skip: this.#o.skip ? (range) => this.#o.skip?.(name, range) ?? false : undefined,
      });
    } finally {
      this.#own = false;
    }
    await storage.acknowledge(name);
    const projection = this.#o.projection();
    // The file holds `text` now. Its point of agreement with the document is the integration's own.
    const heads = done && done.branch.length > 0 ? done.branch : base;
    this.#setBase(heads, A.view(handle.doc() ?? doc, heads).text);
    projection?.adopt(text);
    if (done && (done.merged.length > 0 || done.skipped.length > 0)) {
      this.#done = done.merged.length > 0 ? done : undefined;
      this.#typed = false;
      this.#o.onNotice({
        kind: "merged",
        id: ++this.#nextId,
        name,
        lines: done.lines,
        ranges: [...done.inserted, ...done.deleted.map((d) => ({ from: d.at, to: d.at }))],
        skipped: done.skipped,
      });
    }
    if (!this.#o.manual()) await projection?.flush();
  }

  /** The heads to merge from: the saved ones if they still give the text saved, else now. */
  #baseHeads(doc: A.Doc<Doc>): A.Heads {
    const base = this.#base;
    if (base) {
      try {
        if (fingerprint(A.view(doc, base.heads).text) === base.fingerprint) return base.heads;
      } catch {
        // The heads are not in this document (another project's, or damaged): start from now.
      }
    }
    return A.getHeads(doc);
  }

  /** The file no longer agrees with the document: neither this session nor the next may merge from the old point. */
  #forgetBase(): void {
    this.#base = undefined;
    const key = this.#o.persistKey;
    if (key) safeRemove(key);
  }

  #setBase(heads: A.Heads, text: string): void {
    this.#base = { heads, fingerprint: fingerprint(text) };
    const key = this.#o.persistKey;
    if (key) safeSet(key, JSON.stringify(this.#base));
  }

  #load(): Base | undefined {
    const key = this.#o.persistKey;
    const raw = key ? safeGet(key) : null;
    if (!raw) return undefined;
    try {
      const value = JSON.parse(raw) as Partial<Base>;
      if (
        Array.isArray(value.heads) &&
        value.heads.every((h) => typeof h === "string") &&
        typeof value.fingerprint === "number"
      ) {
        return { heads: value.heads, fingerprint: value.fingerprint };
      }
    } catch {
      // damaged: ignored, the next write records a new one
    }
    return undefined;
  }
}
