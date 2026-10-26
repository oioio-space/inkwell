import {
  type Decision,
  type DeletionRequest,
  type IndexDoc,
  type IndexNode,
  isHash,
  isId,
  MAX_NODES,
  type NodeKind,
  type Trashed,
} from "./model";
import {
  collisionKey,
  MAX_DEPTH,
  MAX_NAME_BYTES,
  sanitizeName,
  splitExt,
  validateName,
  validatePath,
} from "./names";

/** One file or folder of the visible tree. */
export interface Entry {
  id: string;
  /** Relative to the project folder, with forward slashes. */
  path: string;
  /** The name shown: the node's own, or one suffixed because a homonym took it (PRJ-07). */
  name: string;
  type: NodeKind;
  parentId: string | null;
  url?: string;
  hash?: string;
  size: number;
  main: boolean;
  /** The file type chosen for it, if one was (ADR 0027). */
  ftype?: string;
  by: string;
  at: number;
  /** The node's own name, when a homonym made materialization change it. */
  renamedFrom?: string;
  /** Attached to the root because its parent is missing or not a folder. */
  orphan?: boolean;
  /** Attached to the root to break a cycle of moves (PRJ-16). */
  cycle?: boolean;
  /** A writer asked to delete it; an admin has not answered. */
  deletion?: DeletionRequest;
  /** An admin refused the last request. */
  refused?: Decision;
}

/** A file or folder in the trash. */
export interface TrashItem {
  id: string;
  type: NodeKind;
  name: string;
  /** Where it was, by the names of its folders. */
  path: string;
  by: string;
  at: number;
  /** The request an admin accepted, if the deletion came from one. */
  req?: string;
  /** How many nodes lie under it (a folder). */
  contents: number;
}

export type WarningKind = "invalid" | "capped" | "too_deep" | "bad_path";

export interface Warning {
  kind: WarningKind;
  id?: string;
  count?: number;
}

/** The tree the index describes, the same on every replica that holds the same index. */
export interface Materialized {
  /** The visible nodes, by path. */
  entries: Entry[];
  byId: ReadonlyMap<string, Entry>;
  byPath: ReadonlyMap<string, Entry>;
  trash: TrashItem[];
  /** Every node in the trash, including those that lie under a folder that is in it too. */
  trashed: ReadonlySet<string>;
  /** Deletion requests waiting for an admin. */
  pending: Entry[];
  /** Every blob hash the index refers to, the trash and the packages included, sorted. */
  refs: string[];
  warnings: Warning[];
  mainId: string | undefined;
  /** Ids of the node and everything under it, trashed or not, in no particular order. */
  subtree(id: string): string[];
  /** The id of the folder at `path`, null for the root, undefined if there is none. */
  folderAt(path: string): string | null | undefined;
}

export interface MaterializeOptions {
  maxNodes?: number;
  maxDepth?: number;
}

const KINDS: ReadonlySet<string> = new Set(["dir", "text", "blob"]);

function validNode(id: string, n: unknown): n is IndexNode {
  if (!isId(id) || n === null || typeof n !== "object") return false;
  const node = n as Partial<IndexNode>;
  if (typeof node.type !== "string" || !KINDS.has(node.type)) return false;
  if (typeof node.name !== "string" || validateName(node.name) !== undefined) return false;
  if (node.parent !== null && node.parent !== undefined && typeof node.parent !== "string") {
    return false;
  }
  if (node.type === "text") {
    return typeof node.url === "string" && node.url.length > 0 && node.url.length <= 255;
  }
  if (node.type === "blob") {
    return (
      isHash(node.hash) &&
      (node.size === undefined || (Number.isSafeInteger(node.size) && node.size >= 0))
    );
  }
  return true;
}

/** True when the trash mark counts: a refusal of the request it accepted cancels it. */
function effectiveTrash(id: string, n: IndexNode, doc: IndexDoc, mainId: string | undefined) {
  const t = n.trashed;
  if (!t || typeof t !== "object" || id === mainId) return undefined;
  const d = doc.decisions?.[id];
  if (d && t.req !== undefined && d.req === t.req) return undefined;
  return t;
}

const hex6 = (id: string) => id.slice(0, 6);

/** `notes (Bob).md` from `notes.md`, cut so that the whole fits a name. */
function suffixed(name: string, tag: string): string {
  const { stem, ext } = splitExt(name);
  const tail = ` (${tag})${ext}`;
  const room = MAX_NAME_BYTES - new TextEncoder().encode(tail).length;
  let s = stem;
  while (s.length > 0 && new TextEncoder().encode(s).length > room) s = s.slice(0, -1);
  return `${s}${tail}`;
}

function freeName(name: string, id: string, taken: Set<string>, who?: string) {
  const tags = [who ? sanitizeName(who) : undefined, hex6(id), id.slice(0, 12), id].filter(
    (x): x is string => x !== undefined,
  );
  for (const tag of tags) {
    const candidate = suffixed(name, tag);
    if (validateName(candidate) === undefined && !taken.has(collisionKey(candidate))) {
      return candidate;
    }
  }
  return id; // 32 hex digits: valid, and no other node has this id
}

/**
 * Builds the visible tree from the index. It is a pure function of the document, so every
 * replica with the same index shows the same tree:
 *
 * - a node whose parent is missing or is not a folder is attached to the root;
 * - of a cycle of moves, the node of smallest id goes to the root (PRJ-16);
 * - of homonyms (same name once NFC and case are folded), the smallest id keeps the name and the
 *   others are suffixed with their author's name (PRJ-07, PRJ-17);
 * - past 10 000 nodes (by id) or 32 levels, nodes are ignored and reported.
 */
export function materialize(
  doc: IndexDoc | undefined,
  opts: MaterializeOptions = {},
): Materialized {
  const maxNodes = opts.maxNodes ?? MAX_NODES;
  const maxDepth = opts.maxDepth ?? MAX_DEPTH;
  const warnings: Warning[] = [];
  const raw = doc?.nodes ?? {};
  const ids: string[] = [];
  for (const id of Object.keys(raw).sort()) {
    if (validNode(id, raw[id])) ids.push(id);
    else warnings.push({ kind: "invalid", id });
  }
  // Over the cap nothing that exists is dropped (a file must never vanish because of the
  // number of its neighbours): admission refuses the excess, the interface refuses new files
  // and this says so.
  if (ids.length > maxNodes) warnings.push({ kind: "capped", count: ids.length - maxNodes });
  const nodes = new Map(ids.map((id) => [id, raw[id] as IndexNode]));

  // What is in the trash, by its own mark: a folder in the trash carries its whole subtree
  // (the mark of each node), and a node without a mark stays visible wherever it is attached.
  const trashMarks = new Map<string, Trashed>();
  for (const [id, n] of nodes) {
    const t = effectiveTrash(id, n, doc as IndexDoc, undefined);
    if (t) trashMarks.set(id, t);
  }

  // Parents: the root for a missing or non-folder parent, or a trashed folder (for a node that
  // is not itself in the trash).
  const parent = new Map<string, string | null>();
  const orphan = new Set<string>();
  const cycle = new Set<string>();
  for (const [id, n] of nodes) {
    const p = n.parent ?? null;
    if (p === null) parent.set(id, null);
    else if (
      p !== id &&
      nodes.get(p)?.type === "dir" &&
      (trashMarks.has(id) || !trashMarks.has(p))
    ) {
      parent.set(id, p);
    } else {
      parent.set(id, null);
      orphan.add(id);
    }
  }
  // Cycles: walk up from every node; a walk that meets itself found one.
  const state = new Map<string, 1 | 2>();
  for (const id of ids) {
    if (state.has(id)) continue;
    const walk: string[] = [];
    let cur: string | null = id;
    while (cur !== null && !state.has(cur)) {
      state.set(cur, 1);
      walk.push(cur);
      cur = parent.get(cur) ?? null;
    }
    if (cur !== null && state.get(cur) === 1) {
      const members = walk.slice(walk.indexOf(cur));
      const first = members.reduce((a, b) => (a < b ? a : b));
      parent.set(first, null);
      cycle.add(first);
    }
    for (const w of walk) state.set(w, 2);
  }

  const children = new Map<string | null, string[]>();
  for (const id of ids) {
    const p = parent.get(id) ?? null;
    const list = children.get(p);
    if (list) list.push(id);
    else children.set(p, [id]);
  }

  // The main node: the one the index names, else a main.typ at the root.
  const wanted = doc?.main;
  let mainId: string | undefined =
    typeof wanted === "string" && nodes.has(wanted) ? wanted : undefined;
  if (mainId === undefined) {
    mainId = (children.get(null) ?? []).find((id) => {
      const n = nodes.get(id) as IndexNode;
      return n.type === "text" && n.name === "main.typ";
    });
  }

  if (mainId !== undefined) trashMarks.delete(mainId); // the main file is never in the trash

  const entries: Entry[] = [];
  const trash: TrashItem[] = [];
  const subtreeOf = (id: string): string[] => {
    const out: string[] = [];
    const todo = [id];
    for (let cur = todo.pop(); cur !== undefined; cur = todo.pop()) {
      out.push(cur);
      todo.push(...(children.get(cur) ?? []));
    }
    return out;
  };

  const rawPath = (id: string, shown: Map<string, string>): string => {
    const parts: string[] = [];
    for (let cur: string | null = id; cur !== null; cur = parent.get(cur) ?? null) {
      parts.push(shown.get(cur) ?? (nodes.get(cur) as IndexNode).name);
    }
    return parts.reverse().join("/");
  };
  const shownName = new Map<string, string>();

  const visit = (parentId: string | null, prefix: string, depth: number): void => {
    const kids = children.get(parentId) ?? [];
    const live: string[] = [];
    for (const id of kids) if (!trashMarks.has(id)) live.push(id);
    // Names: the smallest id of each group keeps its own.
    const taken = new Set<string>();
    const names = new Map<string, string>();
    const losers: string[] = [];
    for (const id of live) {
      const n = nodes.get(id) as IndexNode;
      const key = collisionKey(n.name);
      if (taken.has(key)) losers.push(id);
      else {
        taken.add(key);
        names.set(id, n.name);
      }
    }
    for (const id of losers) {
      const n = nodes.get(id) as IndexNode;
      // The author's own name is in the node, so that every replica builds the same name.
      const name = freeName(n.name, id, taken, n.who);
      taken.add(collisionKey(name));
      names.set(id, name);
    }
    for (const id of live) {
      const n = nodes.get(id) as IndexNode;
      const name = names.get(id) as string;
      const path = prefix === "" ? name : `${prefix}/${name}`;
      if (depth > maxDepth) {
        warnings.push({ kind: "too_deep", id });
        continue;
      }
      if (validatePath(path) !== undefined) {
        warnings.push({ kind: "bad_path", id });
        continue;
      }
      shownName.set(id, name);
      const d = doc?.decisions?.[id];
      const req = n.deletion;
      const pending = req && typeof req === "object" && !(d && d.req === req.req) ? req : undefined;
      const entry: Entry = {
        id,
        path,
        name,
        type: n.type,
        parentId,
        size: n.type === "blob" ? (n.size ?? 0) : 0,
        main: id === mainId,
        by: n.by,
        at: n.at,
      };
      if (typeof n.ftype === "string") entry.ftype = n.ftype;
      if (n.type === "text") entry.url = n.url as string;
      if (n.type === "blob") entry.hash = n.hash as string;
      if (name !== n.name) entry.renamedFrom = n.name;
      if (orphan.has(id)) entry.orphan = true;
      if (cycle.has(id)) entry.cycle = true;
      if (pending) entry.deletion = pending;
      else if (d && req && d.req === req.req) entry.refused = d;
      entries.push(entry);
      if (n.type === "dir") visit(id, path, depth + 1);
    }
  };

  const trashItem = (id: string, n: IndexNode, t: Trashed): TrashItem => ({
    id,
    type: n.type,
    name: n.name,
    path: "",
    by: t.by,
    at: t.at,
    ...(t.req === undefined ? {} : { req: t.req }),
    contents: subtreeOf(id).length - 1,
  });

  visit(null, "", 1);
  // The trash lists what was thrown away: the nodes whose folder is not in it too.
  for (const [id, t] of trashMarks) {
    const above = parent.get(id) ?? null;
    if (above === null || !trashMarks.has(above))
      trash.push(trashItem(id, nodes.get(id) as IndexNode, t));
  }
  for (const item of trash) item.path = rawPath(item.id, shownName);
  trash.sort((a, b) => a.at - b.at || (a.id < b.id ? -1 : 1));
  entries.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0));

  const refs = new Set<string>();
  for (const n of nodes.values()) if (n.type === "blob" && n.hash) refs.add(n.hash);
  for (const p of Object.values(doc?.packages ?? {})) if (isHash(p?.hash)) refs.add(p.hash);

  const byId = new Map(entries.map((e) => [e.id, e]));
  const byPath = new Map(entries.map((e) => [e.path, e]));
  return {
    entries,
    byId,
    byPath,
    trash,
    trashed: new Set(trashMarks.keys()),
    pending: entries.filter((e) => e.deletion !== undefined),
    refs: [...refs].sort(),
    warnings,
    mainId: mainId !== undefined && byId.has(mainId) ? mainId : undefined,
    subtree: subtreeOf,
    folderAt(path) {
      if (path === "") return null;
      const e = byPath.get(path);
      return e?.type === "dir" ? e.id : undefined;
    },
  };
}
