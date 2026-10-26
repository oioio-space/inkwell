import type { AutomergeUrl, DocHandle, Repo } from "@automerge/automerge-repo";
import { t } from "../i18n";
import type { ProjectFile } from "../store/files";
import { type Projection, startProjection } from "../store/projection";
import type { BlobInfo, TreeEntry } from "../store/tree";
import { type ImportPlan, type IncomingFile, planImport } from "./import";
import { type Materialized, materialize } from "./materialize";
import type { IndexDoc, Role } from "./model";
import { validatePath } from "./names";
import type { NodeSpec } from "./ops";
import { type Outcome, ProjectIndex } from "./ProjectIndex";
import { TreeSync } from "./sync";

type TextDoc = { text: string };

/** What the runtime needs of the Go side: the `store` stream, as GoStorageAdapter offers it. */
export interface RuntimeStorage {
  writeProjection(path: string, text: string, point?: string): Promise<void>;
  setTree(entries: TreeEntry[]): Promise<number>;
  setReferenced(hashes: Uint8Array[]): Promise<void>;
  putBlob(path: string, bytes?: Uint8Array): Promise<BlobInfo>;
  listFiles(): Promise<ProjectFile[]>;
}

/** The blob side of the project as the Go service reports it (Projects.BlobProgress). */
export interface BlobStatus {
  Refused?: string[] | null;
  Files: number;
  Stored: number;
  Bytes: number;
  StoredBytes: number;
  Limit: number;
  NeedsConfirmation: boolean;
}

export interface RuntimeDeps {
  repo: Repo;
  storage: RuntimeStorage;
  me(): { id: string; name: string };
  blobStatus?(): Promise<BlobStatus>;
  confirmBlobs?(): Promise<void>;
  now?(): number;
}

/** How far the files of the project have reached this device. */
export interface Progress {
  /** Files complete here: documents loaded and blobs verified. */
  done: number;
  total: number;
  /** Bytes complete / in all, where known. */
  doneBytes: number;
  bytes: number;
  /** The project is over its quota and waits for the person to agree to download the rest. */
  needsConfirmation: boolean;
  limit: number;
  /** Paths of blobs over the size limit, which are not received. */
  refused: string[];
}

const EMPTY_PROGRESS: Progress = {
  done: 0,
  total: 0,
  doneBytes: 0,
  bytes: 0,
  needsConfirmation: false,
  limit: 0,
  refused: [],
};

/** Documents fetched at once (ADR 0023: four in parallel, the main file first). */
const PARALLEL_DOCS = 4;
/** How long before a document that no one could give us is asked for again. */
const RETRY_DOC_MS = 3000;
/** The files kept open, each with its own editor and undo history. */
export const MAX_OPEN = 12;

/**
 * The files of the open project (ADR 0023, B4, B5): the index, the text documents it names,
 * their projection to the folder, the blobs, and which file is shown. One per open project.
 *
 * - The index is the root document. Each text file is its own document, loaded four at a time,
 *   main first; the progress counts the complete ones and the verified blobs.
 * - Every change of the tree is told to Go (`Tree`, `Referenced`); a text file is written to the
 *   folder only at the path Go has acknowledged, so a rename never writes to a path Go does not
 *   know and nothing is written before the tree that lists it.
 * - The file shown is held by its id, so a rename does not close it, and a deletion by someone
 *   else leaves it open with a banner, its typing safe in the document (PRJ-05, PRJ-06).
 */
export class ProjectRuntime {
  readonly index: ProjectIndex;
  /** The visible tree, the trash and the requests: changes with the index. */
  tree = $state.raw<Materialized>(materialize(undefined));
  role = $state<Role>("writer");
  /** The file shown, by id; null before the project is open. */
  currentId = $state<string | null>(null);
  /** The ids of the files opened, the latest last: each keeps an editor (and its undo history). */
  openIds = $state.raw<string[]>([]);
  progress = $state.raw<Progress>(EMPTY_PROGRESS);
  /** How many entries of the tree Go refused to write (names that clash on disk). */
  treeRejected = $state(0);
  /** Files in the project folder that are not in the project: to add, if the person wants. */
  untracked = $state.raw<ProjectFile[]>([]);
  /** Bumped when a document becomes ready, for what depends on `handleOf`. */
  ready = $state(0);

  #deps: RuntimeDeps;
  #sync: TreeSync;
  #docs = new Map<string, DocHandle<TextDoc>>();
  #loading = new Set<string>();
  #queue: string[] = [];
  #active = 0;
  #projections = new Map<string, Projection>();
  /** The path Go last acknowledged for each text file: where its projection writes. */
  #acked = new Map<string, string>();
  #lastPath = "main.typ";
  /** Every path the tree has had since the project was opened: files Inkwell wrote itself. */
  #left = new Set<string>();
  #timers = new Set<ReturnType<typeof setTimeout>>();
  #poll: ReturnType<typeof setInterval> | undefined;
  #stopped = false;
  #off: () => void;
  /** Called when a file is edited outside and the person must decide (path of the file). */
  onProjectionError: (path: string, err: unknown) => void = () => {};
  onProjectionWritten: () => void = () => {};

  constructor(handle: DocHandle<IndexDoc>, deps: RuntimeDeps) {
    this.#deps = deps;
    this.index = new ProjectIndex(handle, {
      me: () => deps.me(),
      role: () => this.role,
      createText: (text) => this.createText(text),
      textOf: (url) => this.#textByUrl(url),
      tags: () => ({ restored: t("files.tag.restored"), copy: t("files.tag.copy") }),
      ...(deps.now ? { now: deps.now } : {}),
    });
    this.#sync = new TreeSync(deps.storage, {
      onResult: (rejected, sent) => this.#acknowledged(rejected, sent),
      onError: (err) => console.warn("tree:", err instanceof Error ? err.message : "failed"),
    });
    this.#off = this.index.onChange((tree) => this.#changed(tree));
  }

  /** Makes the index current: migrates a project that has only `text`, then follows it. */
  start(): void {
    this.index.ensureMigrated();
    this.#changed(this.index.tree);
    const main = this.tree.mainId ?? this.tree.entries.find((e) => e.type === "text")?.id;
    if (main && this.currentId === null) this.openFile(main);
    void this.refreshProgress();
    this.#poll = setInterval(() => {
      if (!this.#stopped && this.progress.done < this.progress.total) void this.refreshProgress();
    }, 1000);
  }

  stop(): void {
    this.#stopped = true;
    clearInterval(this.#poll);
    for (const timer of this.#timers) clearTimeout(timer);
    for (const p of this.#projections.values()) p.stop();
    this.#projections.clear();
    this.#sync.stop();
    this.#off();
    this.index.stop();
  }

  /** Tells the runtime the person's role changed (from the member list). */
  setRole(role: Role): void {
    if (this.role === role) return;
    this.role = role;
    this.index.ensureMigrated();
    this.index.refresh(true);
  }

  // — Files shown —

  /** The path of the file shown; of a file that was trashed, the path it had. */
  get currentPath(): string {
    const e = this.currentId ? this.tree.byId.get(this.currentId) : undefined;
    if (e) this.#lastPath = e.path;
    return this.#lastPath;
  }

  /** What the file shown is, if it is in the tree. */
  get current() {
    return this.currentId ? this.tree.byId.get(this.currentId) : undefined;
  }

  /** Who put the shown file in the trash, if it is there (PRJ-06, FM-09). */
  get trashedCurrent() {
    const id = this.currentId;
    if (!id || this.tree.byId.has(id)) return undefined;
    return this.tree.trash.find((x) => x.id === id) ?? this.#hiddenTrash(id);
  }

  #hiddenTrash(id: string) {
    if (!this.tree.trashed.has(id)) return undefined;
    const n = this.index.doc.nodes?.[id];
    if (!n?.trashed) return undefined;
    return {
      id,
      type: n.type,
      name: n.name,
      path: n.name,
      by: n.trashed.by,
      at: n.trashed.at,
      contents: 0,
    };
  }

  /** The handle of the document of a text file, once loaded. */
  handleOf(id: string): DocHandle<TextDoc> | undefined {
    return this.#docs.get(id);
  }

  /** Shows a file. A text file's document is loaded first; a blob shows its card. */
  openFile(id: string): void {
    const e = this.tree.byId.get(id);
    if (!e && !this.tree.trashed.has(id)) return;
    this.currentId = id;
    if (e) this.#lastPath = e.path;
    this.openIds = [...this.openIds.filter((x) => x !== id), id].slice(-MAX_OPEN);
    this.#want(id, true);
  }

  openPath(path: string): void {
    const e = this.tree.byPath.get(path);
    if (e && e.type !== "dir") this.openFile(e.id);
  }

  /** A file the person closes (or that no longer exists): its editor goes. */
  #forget(id: string): void {
    this.openIds = this.openIds.filter((x) => x !== id);
    if (this.currentId === id) {
      this.currentId = null;
      const next = this.tree.mainId;
      if (next) this.openFile(next);
    }
  }

  // — Documents —

  /** A new text document, for a file being created. */
  createText(text: string): AutomergeUrl {
    const handle = this.#deps.repo.create<TextDoc>({ text });
    return handle.url;
  }

  #textByUrl(url: string): string | undefined {
    for (const [id, h] of this.#docs) {
      if (h.url === url && this.tree.byId.has(id)) return h.doc()?.text;
    }
    return undefined;
  }

  /** Puts a document in the line to be loaded (first, if wanted now). */
  #want(id: string, first = false): void {
    if (this.#docs.has(id) || this.#loading.has(id)) return;
    this.#loading.add(id);
    if (first) this.#queue.unshift(id);
    else this.#queue.push(id);
    this.#pump();
  }

  #pump(): void {
    while (this.#active < PARALLEL_DOCS && this.#queue.length > 0 && !this.#stopped) {
      const id = this.#queue.shift() as string;
      this.#active++;
      void this.#load(id).finally(() => {
        this.#active--;
        this.#pump();
      });
    }
  }

  async #load(id: string): Promise<void> {
    const entry = this.tree.byId.get(id);
    const url = entry?.url;
    if (!entry || !url) {
      this.#loading.delete(id);
      return;
    }
    try {
      const handle =
        url === this.index.handle.url
          ? (this.index.handle as unknown as DocHandle<TextDoc>)
          : await this.#deps.repo.find<TextDoc>(url as AutomergeUrl);
      if (this.#stopped) return;
      this.#docs.set(id, handle);
      this.#loading.delete(id);
      this.ready++;
      this.#progressChanged();
      this.#project(id);
    } catch {
      // Nobody has it yet (or the link dropped): ask again soon, without blocking the others.
      this.#loading.delete(id);
      const timer = setTimeout(() => {
        this.#timers.delete(timer);
        if (this.tree.byId.has(id)) this.#want(id);
      }, RETRY_DOC_MS);
      this.#timers.add(timer);
    }
  }

  // — Projection to the folder —

  /** Starts writing the file of `id` to the folder, once its document and Go's tree are there. */
  #project(id: string): void {
    const handle = this.#docs.get(id);
    // The main file's text is the root document: the workspace writes it (with its sync point).
    if (!handle || handle === (this.index.handle as unknown) || this.#projections.has(id)) return;
    if (!this.#acked.has(id)) return;
    this.#projections.set(
      id,
      startProjection(handle, this.#deps.storage, {
        name: () => this.#acked.get(id) ?? this.tree.byId.get(id)?.path ?? "",
        onError: (err) => this.onProjectionError(this.#acked.get(id) ?? "", err),
        onWritten: () => this.onProjectionWritten(),
      }),
    );
  }

  /** The projection of a path, to write it again (after an outside edit was dealt with). */
  projectionOf(path: string): Projection | undefined {
    for (const [id, p] of this.#projections) if (this.#acked.get(id) === path) return p;
    return undefined;
  }

  /** The document of the file at `path`. */
  handleAt(path: string): DocHandle<TextDoc> | undefined {
    const e = this.tree.byPath.get(path);
    return e ? this.#docs.get(e.id) : undefined;
  }

  // — Following the index —

  #changed(tree: Materialized): void {
    this.tree = tree;
    for (const e of tree.entries) this.#left.add(e.path);
    this.#sync.update(tree);
    for (const e of tree.entries) if (e.type === "text") this.#want(e.id, e.main);
    // A file that is gone for good (purged) cannot stay open.
    for (const id of this.openIds) {
      if (!tree.byId.has(id) && !tree.trashed.has(id)) this.#forget(id);
    }
    if (
      this.currentId !== null &&
      !tree.byId.has(this.currentId) &&
      !tree.trashed.has(this.currentId)
    ) {
      this.#forget(this.currentId);
    }
    this.#progressChanged();
    void this.refreshUntracked();
  }

  /** Go has the tree: text files may be written at their new paths. */
  #acknowledged(rejected: number, sent: Materialized): void {
    this.treeRejected = rejected;
    const known = new Map<string, string>();
    for (const e of sent.entries) if (e.type === "text") known.set(e.id, e.path);
    this.#acked = known;
    // A file that left the tree (trashed) is not written until it is back.
    for (const [id, p] of this.#projections) {
      if (known.has(id)) continue;
      p.stop();
      this.#projections.delete(id);
    }
    for (const id of this.#docs.keys()) this.#project(id);
    this.#progressChanged();
  }

  // — Progress —

  /** Asks Go how many blobs are in, and counts the documents here. */
  async refreshProgress(): Promise<void> {
    const status = await this.#deps.blobStatus?.().catch(() => undefined);
    this.#progressChanged(status);
  }

  #status: BlobStatus | undefined;

  #progressChanged(status = this.#status): void {
    this.#status = status;
    let docs = 0;
    let docsDone = 0;
    let doneBytes = 0;
    for (const e of this.tree.entries) {
      if (e.type !== "text") continue;
      docs++;
      const h = this.#docs.get(e.id);
      if (h) {
        docsDone++;
        doneBytes += h.doc()?.text.length ?? 0;
      }
    }
    const blobs = this.tree.entries.filter((e) => e.type === "blob");
    const stored = status ? Math.min(status.Stored, blobs.length) : blobs.length;
    const next: Progress = {
      done: docsDone + stored,
      total: docs + blobs.length,
      doneBytes: doneBytes + (status?.StoredBytes ?? blobs.reduce((n, e) => n + e.size, 0)),
      bytes: doneBytes + blobs.reduce((n, e) => n + e.size, 0),
      needsConfirmation: status?.NeedsConfirmation ?? false,
      limit: status?.Limit ?? 0,
      refused: status?.Refused ?? [],
    };
    const p = this.progress;
    if (
      p.done === next.done &&
      p.total === next.total &&
      p.doneBytes === next.doneBytes &&
      p.needsConfirmation === next.needsConfirmation &&
      p.refused.join() === next.refused.join()
    ) {
      return;
    }
    this.progress = next;
  }

  /** The person agrees to download a project over its quota. */
  async confirmDownload(): Promise<void> {
    await this.#deps.confirmBlobs?.();
    await this.refreshProgress();
  }

  // — The folder —

  /** Looks for files in the project folder that the project does not have. */
  async refreshUntracked(): Promise<void> {
    try {
      const files = await this.#deps.storage.listFiles();
      // Go never deletes a file: what left the tree (the trash, a purge) stays in the folder,
      // and is not "a file the project does not have".
      const known = new Set(this.tree.entries.map((e) => e.path));
      const trashed = this.tree.trash.map((x) => x.path);
      const fresh = files.filter(
        (f) =>
          !known.has(f.path) &&
          !this.#left.has(f.path) &&
          !trashed.some((x) => f.path === x || f.path.startsWith(`${x}/`)) &&
          validatePath(f.path) === undefined,
      );
      const same =
        fresh.length === this.untracked.length &&
        fresh.every(
          (f, i) => f.path === this.untracked[i]?.path && f.size === this.untracked[i]?.size,
        );
      if (!same) this.untracked = fresh;
    } catch {
      // The folder cannot be listed now; the next change or poll will try again.
    }
  }

  /**
   * Adds a file that is in the project folder to the project, as a blob: an explicit act of the
   * person (ADR 0023), never done by itself. The folders above it are made if needed.
   */
  async addFromDisk(path: string): Promise<Outcome> {
    const info = await this.#deps.storage.putBlob(path);
    const parts = path.split("/");
    const name = parts.pop() as string;
    const specs: NodeSpec[] = [];
    let parent: NodeSpec["parent"] = null;
    let where = "";
    for (const part of parts) {
      where = where === "" ? part : `${where}/${part}`;
      const existing = this.tree.byPath.get(where);
      if (existing?.type === "dir" && specs.length === 0) {
        parent = existing.id;
        continue;
      }
      specs.push({ type: "dir", name: part, parent });
      parent = { spec: specs.length - 1 };
    }
    specs.push({
      type: "blob",
      name,
      parent,
      hash: `blake3:${Array.from(info.hash, (b) => b.toString(16).padStart(2, "0")).join("")}`,
      size: info.size,
    });
    const out = this.index.createMany(specs);
    if (out.ok) void this.refreshUntracked();
    return out;
  }

  /** A blob replaced on disk: the same file in the project, with its new hash (PRJ-13). */
  async updateFromDisk(id: string): Promise<Outcome> {
    const e = this.tree.byId.get(id);
    if (e?.type !== "blob") return { ok: false, error: { code: "not_found" } };
    const info = await this.#deps.storage.putBlob(e.path);
    return this.index.replaceBlob(id, {
      hash: `blake3:${Array.from(info.hash, (b) => b.toString(16).padStart(2, "0")).join("")}`,
      size: info.size,
    });
  }

  /** Brings dropped or picked files into the project (FM-02). */
  async importFiles(files: IncomingFile[], parent: string | null): Promise<ImportResult> {
    const plan = await planImport(files, parent, this.tree, {
      createText: (text) => this.createText(text),
      putBlob: (path, bytes) => this.#deps.storage.putBlob(path, bytes),
    });
    if (plan.specs.length === 0) return { plan, outcome: { ok: true, ids: [] } };
    return { plan, outcome: this.index.createMany(plan.specs) };
  }
}

export interface ImportResult {
  plan: ImportPlan;
  outcome: Outcome;
}
