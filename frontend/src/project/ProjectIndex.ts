import { next as A } from "@automerge/automerge";
import type { AutomergeUrl, DocHandle } from "@automerge/automerge-repo";
import { FileHistory, type FileOpEntry } from "./history";
import { type Entry, type Materialized, materialize } from "./materialize";
import { migration } from "./migrate";
import { type DeletionMode, type IndexDoc, plain, type Role } from "./model";
import { splitExt } from "./names";
import type { OpContext, OpError, Result } from "./ops";
import * as ops from "./ops";

/** What the index needs from the app around it. */
export interface IndexHost {
  /** This device and its person's name. */
  me(): { id: string; name: string };
  /** What this device may do, from the member list. */
  role(): Role;
  /** Makes a new text document with `text` and gives its URL. */
  createText(text: string): AutomergeUrl;
  /** The text of a text document, if it is loaded here. */
  textOf(url: string): string | undefined;
  /** The words that mark a restored file and a copy ("restauré", "copie"). */
  tags?(): { restored: string; copy: string };
  now?(): number;
}

export type Outcome = { ok: true; ids: string[] } | { ok: false; error: OpError };

/**
 * The project index as the app uses it (ADR 0023): the root document, its materialized tree
 * kept up to date, and the operations of B5, each one change of the document, attributed.
 *
 * Reading is open to everyone; writing is checked here against the person's role, and again by
 * every replica at admission ({@link ./admit}).
 */
export class ProjectIndex {
  readonly handle: DocHandle<IndexDoc>;
  #host: IndexHost;
  #doc: IndexDoc = {};
  #tree: Materialized = materialize(undefined);
  #heads = "";
  #listeners = new Set<(tree: Materialized) => void>();
  #history = new FileHistory();
  // Typing in the main file changes this document too, at every key: the tree only when something
  // other than `text` did.
  #onChange = ({ patches }: { patches: readonly { path: readonly unknown[] }[] }) => {
    if (patches.length > 0 && patches.every((p) => p.path[0] === "text")) return;
    this.refresh();
  };

  constructor(handle: DocHandle<IndexDoc>, host: IndexHost) {
    this.handle = handle;
    this.#host = host;
    handle.on("change", this.#onChange);
    this.refresh();
  }

  stop(): void {
    this.handle.off("change", this.#onChange);
    this.#listeners.clear();
  }

  /** The index as plain data (the text of the main file left out). */
  get doc(): IndexDoc {
    return this.#doc;
  }

  /** The visible tree, the trash and the requests. */
  get tree(): Materialized {
    return this.#tree;
  }

  /** The rule for deleting files. */
  get mode(): DeletionMode {
    return ops.deletionMode(this.#doc);
  }

  get role(): Role {
    return this.#host.role();
  }

  /** Calls `cb` after every change of the tree; returns the way to stop. */
  onChange(cb: (tree: Materialized) => void): () => void {
    this.#listeners.add(cb);
    return () => this.#listeners.delete(cb);
  }

  /** Recomputes the tree when the document changed (or `force`). */
  refresh(force = false): void {
    const doc = this.handle.doc();
    if (!doc) return;
    const heads = A.getHeads(doc).join();
    if (!force && heads === this.#heads) return;
    this.#heads = heads;
    const index = plain(doc);
    // A project not yet migrated is shown as the index it will become.
    this.#doc = index.nodes === undefined ? this.#virtual(index) : index;
    this.#tree = materialize(this.#doc);
    for (const cb of this.#listeners) cb(this.#tree);
  }

  #virtual(index: IndexDoc): IndexDoc {
    const draft: IndexDoc = { ...index };
    migration(draft, this.handle.url, "-", 0)?.(draft);
    return plain(draft);
  }

  /**
   * Gives a project that has only `text` its index, if this person may write. Returns whether it
   * wrote. Two people doing it at once write the same thing.
   */
  ensureMigrated(): boolean {
    const doc = this.handle.doc();
    if (!doc || this.role === "reader") return false;
    const m = migration(plain(doc), this.handle.url, this.#host.me().id, this.#now());
    if (!m) return false;
    this.handle.change(m);
    this.refresh();
    return true;
  }

  #now(): number {
    return this.#host.now?.() ?? Date.now();
  }

  #ctx(): OpContext {
    const me = this.#host.me();
    return {
      me: me.id,
      name: me.name,
      role: this.role,
      now: this.#now(),
      restoredTag: this.#host.tags?.().restored ?? "restored",
    };
  }

  /** Applies a plan as one change of the document. */
  #run(r: Result): Outcome {
    if (!r.ok) return r;
    try {
      this.handle.change(r.plan.apply, {
        message: JSON.stringify(r.plan.message),
        time: Math.floor(this.#now() / 1000),
      });
    } catch {
      return { ok: false, error: { code: "invalid" } };
    }
    this.refresh();
    return { ok: true, ids: r.plan.created ?? [] };
  }

  // — Creating —

  /** A new empty text file (or one with `text`) in the folder `parent` (null: the root). */
  createFile(parent: string | null, name: string, text = ""): Outcome {
    const url = this.#host.createText(text);
    return this.#run(
      ops.create(this.#doc, this.#tree, this.#ctx(), [{ type: "text", name, parent, url }]),
    );
  }

  createFolder(parent: string | null, name: string): Outcome {
    return this.#run(
      ops.create(this.#doc, this.#tree, this.#ctx(), [{ type: "dir", name, parent }]),
    );
  }

  /** Files and folders made together, one change (an import). */
  createMany(specs: ops.NodeSpec[]): Outcome {
    return this.#run(ops.create(this.#doc, this.#tree, this.#ctx(), specs));
  }

  /** A copy of each file or folder (with what is under it), named "x (copie).ext". */
  duplicate(ids: string[]): Outcome {
    const tag = this.#host.tags?.().copy ?? "copy";
    const specs: ops.NodeSpec[] = [];
    const kids = new Map<string | null, Entry[]>();
    for (const e of this.#tree.entries) {
      const list = kids.get(e.parentId);
      if (list) list.push(e);
      else kids.set(e.parentId, [e]);
    }
    const copy = (e: Entry, parent: ops.NodeSpec["parent"], name: string): void => {
      const at = specs.length;
      const spec: ops.NodeSpec = { type: e.type, name, parent };
      if (e.type === "text") spec.url = this.#host.createText(this.#host.textOf(e.url ?? "") ?? "");
      if (e.type === "blob") {
        spec.hash = e.hash as string;
        spec.size = e.size;
      }
      specs.push(spec);
      if (e.type === "dir") {
        for (const child of kids.get(e.id) ?? []) copy(child, { spec: at }, child.name);
      }
    };
    for (const id of ids) {
      const e = this.#tree.byId.get(id);
      if (!e) return { ok: false, error: { code: "not_found" } };
      const { stem, ext } = splitExt(e.name);
      const name = ops.uniqueName(this.#tree, e.parentId, `${stem} (${tag})${ext}`);
      copy(e, e.parentId, name);
    }
    return this.#run(ops.create(this.#doc, this.#tree, this.#ctx(), specs, "create"));
  }

  // — Changing —

  rename(id: string, name: string): Outcome {
    return this.#run(ops.rename(this.#tree, this.#ctx(), id, name));
  }

  move(ids: string[], parent: string | null): Outcome {
    return this.#run(ops.move(this.#tree, this.#ctx(), ids, parent));
  }

  /** Deletes: to the trash for an admin or when files go freely, else a request (FM-05). */
  remove(ids: string[]): Outcome {
    return this.#run(ops.remove(this.#doc, this.#tree, this.#ctx(), ids));
  }

  accept(ids: string[]): Outcome {
    return this.#run(ops.accept(this.#tree, this.#ctx(), ids));
  }

  refuse(ids: string[]): Outcome {
    return this.#run(ops.refuse(this.#tree, this.#ctx(), ids));
  }

  restore(ids: string[]): Outcome {
    return this.#run(ops.restore(this.#doc, this.#tree, this.#ctx(), ids));
  }

  purge(ids: string[]): Outcome {
    return this.#run(ops.purge(this.#doc, this.#tree, this.#ctx(), ids));
  }

  setMain(id: string): Outcome {
    return this.#run(ops.setMain(this.#tree, this.#ctx(), id));
  }

  setMode(mode: DeletionMode): Outcome {
    return this.#run(ops.setDeletionMode(this.#doc, this.#ctx(), mode));
  }

  /** A binary file changed on disk: same node, new hash (PRJ-13). */
  replaceBlob(id: string, blob: { hash: string; size: number }): Outcome {
    return this.#run(ops.replaceBlob(this.#tree, this.#ctx(), id, blob));
  }

  // — History —

  /**
   * The operations on the files, oldest first, with who did them (as they declared) and a
   * sentence in the language of the interface. `since` is a time in milliseconds on the
   * authors' clocks: only later operations come back (0 or none: all).
   */
  fileOps(since = 0): FileOpEntry[] {
    const doc = this.handle.doc();
    if (doc) this.#history.update(doc);
    return this.#history.since(since);
  }

  /** True when {@link revert} knows the exact inverse of this operation. */
  canRevert(entry: FileOpEntry): boolean {
    switch (entry.op) {
      case "rename":
        return entry.items[0]?.from !== undefined;
      case "move":
        return entry.items.every((i) => i.fromId !== undefined);
      case "trash":
      case "accept":
      case "restore":
      case "request":
      case "create":
      case "mode":
        return true;
      default:
        return false;
    }
  }

  /** Undoes one operation by its exact inverse, in one change (FM-19). */
  revert(entry: FileOpEntry): Outcome {
    const ids = entry.items.map((i) => i.id);
    switch (entry.op) {
      case "rename": {
        const item = entry.items[0];
        if (!item || item.from === undefined) break;
        return this.rename(item.id, item.from);
      }
      case "move": {
        const first = entry.items[0];
        if (!first || first.fromId === undefined) break;
        return this.move(ids, first.fromId);
      }
      case "trash":
      case "accept":
        return this.restore(ids);
      case "restore":
      case "create":
        return this.remove(ids);
      case "request":
        return this.refuse(ids);
      case "mode":
        return this.setMode(entry.mode === "free" ? "approval" : "free");
    }
    return { ok: false, error: { code: "invalid" } };
  }
}
