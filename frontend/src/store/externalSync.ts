import { next as A } from "@automerge/automerge";
import type { DocHandle } from "@automerge/automerge-repo";
import { tooBigToEdit } from "./limits";
import { type Integration, integrate, type Range, type Skipped, undoIntegration } from "./merge";
import type { Projection } from "./projection";
import { safeGet, safeRemove } from "./safeStorage";
import type { ExternalPolicy } from "./settings.svelte";
import { decodeSyncPoint, encodeSyncPoint, mergeHeads } from "./syncPoint";

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
  /**
   * Tells Go the outside edit was dealt with, so Inkwell may write the file again. With `point`,
   * Go records it with the hash of the file as it is now; without, the file has no sync point.
   */
  acknowledge(name: string, point?: string): Promise<void>;
  /** Where the document and the file last agreed, as recorded by Go with the file's hash. */
  readSyncPoint(name: string): Promise<string | undefined>;
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
  /**
   * The file is too big to enter the document (ADR 0028): called with its text, which is only
   * read, and with undefined once the file is small again and taken in as usual.
   */
  onBig?: (text: string | undefined) => void;
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
  /**
   * Where an older version kept the point of agreement (localStorage). Used once if Go has none,
   * then dropped: Go keeps the point with the file's hash now.
   */
  legacyKey?: string;
}

/** What the debounce waits after the last outside write. */
export const EXTERNAL_DEBOUNCE_MS = 200;

/**
 * Brings the edits other programs make to the project file into the document (B1 EXT-*).
 *
 * Go remembers the heads where the document and the file last agreed (a write of Inkwell's, or a
 * previous integration), with the hash of the file there. An outside edit is made as of those
 * heads and merged, so what the person or others typed since stays, whatever window or session
 * typed it. Bursts of writes are one integration; Go has already ignored the
 * echo of Inkwell's own writes.
 */
export class ExternalSync {
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
    options.handle.on("change", this.#onChange);
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
      this.#dropLegacy();
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
        this.#o.onBig?.(undefined); // the file is gone: the document shows again
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
      if (tooBigToEdit(text.length)) {
        // Decided before Automerge sees a byte; the file stays as it is on the disk.
        projection?.hold(true);
        this.#o.onBig?.(text);
        return;
      }
      this.#o.onBig?.(undefined);
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
    // The file holds the outside text until Inkwell writes: the point of agreement is gone (the
    // acknowledgement above recorded none).
    this.#dropLegacy();
    this.#o.onNotice(null);
    if (!this.#o.manual()) await projection?.flush(true);
  }

  async #merge(text: string): Promise<void> {
    const { handle, name, storage } = this.#o;
    const doc = handle.doc();
    if (!doc) return;
    const base = mergeHeads(doc, await this.#readPoint());
    this.#own = true;
    let done: Integration | undefined;
    try {
      done = integrate(handle, base, text, {
        skip: this.#o.skip ? (range) => this.#o.skip?.(name, range) ?? false : undefined,
      });
    } finally {
      this.#own = false;
    }
    const projection = this.#o.projection();
    // The file holds `text` now. Its point of agreement with the document is the integration's own,
    // recorded by Go with the hash of the file as it is at the acknowledgement.
    const heads = done && done.branch.length > 0 ? done.branch : base;
    await storage.acknowledge(
      name,
      encodeSyncPoint(heads, A.view(handle.doc() ?? doc, heads).text),
    );
    this.#dropLegacy();
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

  /** The point Go recorded, else (once) the one an older version kept in localStorage. */
  async #readPoint(): Promise<ReturnType<typeof decodeSyncPoint>> {
    const { name, storage, legacyKey } = this.#o;
    const recorded = decodeSyncPoint(await storage.readSyncPoint(name).catch(() => undefined));
    if (recorded) {
      this.#dropLegacy();
      return recorded;
    }
    return legacyKey ? decodeSyncPoint(safeGet(legacyKey)) : undefined;
  }

  /** The older localStorage copy is obsolete once Go holds the point, or the point is gone. */
  #dropLegacy(): void {
    const key = this.#o.legacyKey;
    if (key) safeRemove(key);
  }
}
