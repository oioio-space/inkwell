import type { Entry, Materialized } from "./materialize";
import {
  type DeletionMode,
  type IndexDoc,
  type IndexNode,
  imm,
  isHash,
  isId,
  MAX_CREATE_PER_CHANGE,
  MAX_NODES,
  type NodeKind,
  newId,
  plain,
  type Role,
} from "./model";
import {
  collisionKey,
  MAX_DEPTH,
  type Reason,
  sanitizeName,
  splitExt,
  validateName,
} from "./names";

/**
 * The operations on the project index (ADR 0023, B5). Each is a pure planner: from the index,
 * its tree and who acts, it returns either why it cannot be done or a {@link Plan} that writes
 * the change into an Automerge document, in one change. A valid client therefore never writes an
 * invalid name, an impossible move or a deletion its role does not allow (PRJ-15), and the
 * admission rules ({@link ./admit}) refuse the same things when a forged client does.
 */

export interface OpContext {
  /** The device making the change. */
  me: string;
  /** Its person's display name, for the history. */
  name: string;
  role: Role;
  now: number;
  /** The word that marks a restored file whose name was taken ("restauré"). */
  restoredTag?: string;
}

export type OpErrorCode =
  | "forbidden"
  | "invalid_name"
  | "exists"
  | "cycle"
  | "not_found"
  | "main"
  | "depth"
  | "limit"
  | "unchanged"
  | "invalid";

export interface OpError {
  code: OpErrorCode;
  /** For "invalid_name": which rule the name broke. */
  reason?: Reason;
  /** The name concerned, for the message. */
  name?: string;
}

export type OpKind =
  | "create"
  | "rename"
  | "move"
  | "trash"
  | "request"
  | "accept"
  | "refuse"
  | "restore"
  | "purge"
  | "main"
  | "mode"
  | "replace";

/** What a change says about itself; the history reads it back (see {@link ./history}). */
export interface OpMessage {
  k: "file";
  v: 1;
  op: OpKind;
  /** The device and the display name of whoever did it: their own declaration. */
  by: string;
  who: string;
  items?: MessageItem[];
  /** How many items there were, when `items` was cut. */
  n?: number;
  mode?: DeletionMode;
}

export interface MessageItem {
  id: string;
  type?: NodeKind;
  name: string;
  /** For a rename, the previous name; for a move, the previous folder path. */
  from?: string;
  /** For a move, the new folder path. */
  to?: string;
  /** The previous folder id, to move back. */
  fromId?: string | null;
  toId?: string | null;
}

export interface Plan {
  apply: (d: IndexDoc) => void;
  message: OpMessage;
  /** For a creation, the ids of the nodes it makes, in the order of the specs. */
  created?: string[];
}

export type Result = { ok: true; plan: Plan } | { ok: false; error: OpError };

const fail = (code: OpErrorCode, extra: Partial<OpError> = {}): Result => ({
  ok: false,
  error: { code, ...extra },
});

/** The most items a message lists; the rest is only counted. */
const MAX_ITEMS = 40;

const canWrite = (c: OpContext) => c.role === "writer" || c.role === "admin";
const isAdmin = (c: OpContext) => c.role === "admin";

export const deletionMode = (doc: IndexDoc | undefined): DeletionMode =>
  doc?.settings?.deletion === "free" ? "free" : "approval";

function message(
  c: OpContext,
  op: OpKind,
  items?: MessageItem[],
  extra?: Partial<OpMessage>,
): OpMessage {
  const m: OpMessage = { k: "file", v: 1, op, by: c.me, who: c.name.slice(0, 80), ...extra };
  if (items) {
    m.items = items.slice(0, MAX_ITEMS);
    if (items.length > MAX_ITEMS) m.n = items.length;
  }
  return m;
}

function stamp(c: OpContext) {
  // `by` is a value, not text: see imm.
  return { by: imm(c.me), at: c.now };
}

/** The keys of the names taken in a folder, except the node `except`. */
function takenKeys(m: Materialized, parent: string | null, except?: string): Set<string> {
  const keys = new Set<string>();
  for (const e of m.entries) {
    if (e.parentId === parent && e.id !== except) keys.add(collisionKey(e.name));
  }
  return keys;
}

/** The path of a folder (null is the root, ""), or undefined if it is not a visible folder. */
function folderPath(m: Materialized, id: string | null): string | undefined {
  if (id === null) return "";
  const e = m.byId.get(id);
  return e?.type === "dir" ? e.path : undefined;
}

const levels = (path: string) => (path === "" ? 0 : path.split("/").length);

/**
 * `name`, or `name (2)` with the extension kept, `name (3)`... — the first one that is free in
 * the folder. For a file that arrives (a copy, an import) and must not replace another.
 */
export function uniqueName(m: Materialized, parent: string | null, name: string): string {
  const taken = takenKeys(m, parent);
  if (!taken.has(collisionKey(name))) return name;
  const { stem, ext } = splitExt(name);
  for (let n = 2; ; n++) {
    const candidate = `${stem} (${n})${ext}`;
    if (validateName(candidate) === undefined && !taken.has(collisionKey(candidate)))
      return candidate;
  }
}

/** What to create: a node under an existing folder, or under another node of the same batch. */
export interface NodeSpec {
  type: NodeKind;
  name: string;
  /** An existing folder (null for the root), or the position in the batch of a folder to create. */
  parent: string | null | { spec: number };
  url?: string;
  hash?: string;
  size?: number;
}

/**
 * Creates files and folders in one change (a new file, an import, a duplicate). The names are
 * checked as typed: sanitizing is the caller's choice.
 */
export function create(
  doc: IndexDoc,
  m: Materialized,
  c: OpContext,
  specs: NodeSpec[],
  op: OpKind = "create",
): Result {
  if (!canWrite(c)) return fail("forbidden");
  if (specs.length === 0) return fail("unchanged");
  if (specs.length > MAX_CREATE_PER_CHANGE) return fail("limit");
  if (Object.keys(doc.nodes ?? {}).length + specs.length > MAX_NODES) return fail("limit");
  const ids = specs.map(() => newId());
  const taken = new Map<string | null, Set<string>>();
  const keysOf = (parent: string | null) => {
    let s = taken.get(parent);
    if (!s) {
      s = takenKeys(m, parent);
      taken.set(parent, s);
    }
    return s;
  };
  const depthOf = new Map<number, number>();
  const nodes: [string, IndexNode][] = [];
  for (const [i, spec] of specs.entries()) {
    const reason = validateName(spec.name);
    if (reason) return fail("invalid_name", { reason, name: spec.name });
    let parent: string | null;
    let depth: number;
    if (typeof spec.parent === "object" && spec.parent !== null) {
      const at = spec.parent.spec;
      const target = specs[at];
      if (at >= i || target?.type !== "dir") return fail("invalid");
      parent = ids[at] as string;
      depth = (depthOf.get(at) as number) + 1;
    } else {
      parent = spec.parent;
      const path = folderPath(m, parent);
      if (path === undefined) return fail("not_found");
      depth = levels(path) + 1;
    }
    if (depth > MAX_DEPTH) return fail("depth", { name: spec.name });
    depthOf.set(i, depth);
    const keys = keysOf(parent);
    const key = collisionKey(spec.name);
    if (keys.has(key)) return fail("exists", { name: spec.name });
    keys.add(key);
    const node: IndexNode = {
      type: imm(spec.type) as NodeKind,
      name: imm(spec.name),
      parent: parent === null ? null : imm(parent),
      ...stamp(c),
      ...(c.name.trim() === "" ? {} : { who: imm(sanitizeName(c.name.trim().slice(0, 40))) }),
    };
    if (spec.type === "text") {
      if (!spec.url) return fail("invalid");
      node.url = imm(spec.url);
    } else if (spec.type === "blob") {
      if (!isHash(spec.hash) || !Number.isSafeInteger(spec.size ?? 0)) return fail("invalid");
      node.hash = imm(spec.hash);
      node.size = spec.size ?? 0;
    }
    nodes.push([ids[i] as string, node]);
  }
  const items: MessageItem[] = specs.map((s, i) => ({
    id: ids[i] as string,
    type: s.type,
    name: s.name,
  }));
  return {
    ok: true,
    plan: {
      message: message(c, op, items),
      created: ids,
      apply(d) {
        d.nodes ??= {};
        for (const [id, n] of nodes) d.nodes[id] = n;
      },
    },
  };
}

/** The visible entry `id`, if the person may change files. */
function target(m: Materialized, c: OpContext, id: string): Entry | OpError {
  if (!canWrite(c)) return { code: "forbidden" };
  const e = m.byId.get(id);
  return e ?? { code: "not_found" };
}

const isError = (x: Entry | OpError): x is OpError => "code" in x;

export function rename(m: Materialized, c: OpContext, id: string, name: string): Result {
  const e = target(m, c, id);
  if (isError(e)) return { ok: false, error: e };
  const reason = validateName(name);
  if (reason) return fail("invalid_name", { reason, name });
  if (takenKeys(m, e.parentId, id).has(collisionKey(name))) return fail("exists", { name });
  if (name === e.name && e.renamedFrom === undefined) return fail("unchanged");
  return {
    ok: true,
    plan: {
      message: message(c, "rename", [{ id, type: e.type, name, from: e.name }]),
      apply(d) {
        (d.nodes?.[id] as IndexNode).name = imm(name);
      },
    },
  };
}

export function move(m: Materialized, c: OpContext, ids: string[], parent: string | null): Result {
  if (!canWrite(c)) return fail("forbidden");
  const dest = folderPath(m, parent);
  if (dest === undefined) return fail("not_found");
  const keys = takenKeys(m, parent);
  const items: MessageItem[] = [];
  for (const id of ids) {
    const e = m.byId.get(id);
    if (!e) return fail("not_found");
    if (e.parentId === parent) continue;
    if (parent !== null && m.subtree(id).includes(parent)) return fail("cycle", { name: e.name });
    const height = Math.max(
      0,
      ...m.entries
        .filter((x) => x.path.startsWith(`${e.path}/`))
        .map((x) => levels(x.path) - levels(e.path)),
    );
    if (levels(dest) + 1 + height > MAX_DEPTH) return fail("depth", { name: e.name });
    const key = collisionKey(e.renamedFrom ?? e.name);
    if (keys.has(key)) return fail("exists", { name: e.name });
    keys.add(key);
    items.push({
      id,
      type: e.type,
      name: e.name,
      from: folderPath(m, e.parentId) ?? "",
      to: dest,
      fromId: e.parentId,
      toId: parent,
    });
  }
  if (items.length === 0) return fail("unchanged");
  return {
    ok: true,
    plan: {
      message: message(c, "move", items),
      apply(d) {
        for (const it of items) {
          (d.nodes?.[it.id] as IndexNode).parent = parent === null ? null : imm(parent);
        }
      },
    },
  };
}

/**
 * Deletes files. An admin, or anyone when the project lets files go freely, puts them in the
 * trash; a writer under the default rule asks instead (FM-05, FM-15). The main file cannot go
 * (FM-18).
 */
export function remove(doc: IndexDoc, m: Materialized, c: OpContext, ids: string[]): Result {
  if (!canWrite(c)) return fail("forbidden");
  const direct = isAdmin(c) || deletionMode(plain(doc)) === "free";
  const trashed: MessageItem[] = [];
  const asked: MessageItem[] = [];
  for (const id of ids) {
    const e = m.byId.get(id);
    if (!e) return fail("not_found");
    if (id === m.mainId) return fail("main", { name: e.name });
    const item: MessageItem = { id, type: e.type, name: e.name };
    if (direct) trashed.push(item);
    else if (e.deletion === undefined) asked.push(item);
  }
  if (trashed.length + asked.length === 0) return fail("unchanged");
  const reqs = new Map(asked.map((i) => [i.id, newId()]));
  return {
    ok: true,
    plan: {
      message: message(c, direct ? "trash" : "request", direct ? trashed : asked),
      apply(d) {
        for (const it of trashed) {
          const req = m.byId.get(it.id)?.deletion?.req;
          (d.nodes?.[it.id] as IndexNode).trashed =
            req === undefined ? stamp(c) : { ...stamp(c), req: imm(req) };
        }
        for (const it of asked) {
          (d.nodes?.[it.id] as IndexNode).deletion = {
            ...stamp(c),
            req: imm(reqs.get(it.id) as string),
          };
        }
      },
    },
  };
}

/** An admin accepts the deletion request of a file: it goes to the trash. */
export function accept(m: Materialized, c: OpContext, ids: string[]): Result {
  if (!isAdmin(c)) return fail("forbidden");
  const items: MessageItem[] = [];
  for (const id of ids) {
    const e = m.byId.get(id);
    if (!e?.deletion) return fail("not_found");
    items.push({ id, type: e.type, name: e.name });
  }
  if (items.length === 0) return fail("unchanged");
  const reqs = new Map(ids.map((id) => [id, (m.byId.get(id) as Entry).deletion?.req as string]));
  return {
    ok: true,
    plan: {
      message: message(c, "accept", items),
      apply(d) {
        for (const it of items) {
          (d.nodes?.[it.id] as IndexNode).trashed = {
            ...stamp(c),
            req: imm(reqs.get(it.id) as string),
          };
        }
      },
    },
  };
}

/** An admin refuses the request: the file stays. A refusal wins over a concurrent acceptance. */
export function refuse(m: Materialized, c: OpContext, ids: string[]): Result {
  if (!isAdmin(c)) return fail("forbidden");
  const items: MessageItem[] = [];
  const reqs = new Map<string, string>();
  for (const id of ids) {
    const e = m.byId.get(id);
    if (!e?.deletion) return fail("not_found");
    items.push({ id, type: e.type, name: e.name });
    reqs.set(id, e.deletion.req);
  }
  if (items.length === 0) return fail("unchanged");
  return {
    ok: true,
    plan: {
      message: message(c, "refuse", items),
      apply(d) {
        d.decisions ??= {};
        for (const it of items) {
          d.decisions[it.id] = {
            ...stamp(c),
            req: imm(reqs.get(it.id) as string),
            verdict: imm("refused") as "refused",
          };
        }
      },
    },
  };
}

/** A name that is free among `keys`, tagged "(restauré)" when the name itself is taken. */
function restoredName(name: string, keys: Set<string>, tag: string): string {
  if (!keys.has(collisionKey(name))) return name;
  const { stem, ext } = splitExt(name);
  for (let n = 1; ; n++) {
    const candidate = `${stem} (${tag}${n === 1 ? "" : ` ${n}`})${ext}`;
    if (validateName(candidate) === undefined && !keys.has(collisionKey(candidate)))
      return candidate;
  }
}

/**
 * Takes files out of the trash. A folder that was deleted too comes back with them (FM-11), and a
 * name taken in the meantime is not overwritten: the file returns as `notes (restauré).md`
 * (FM-10). Under the default rule only an admin restores, as only an admin deletes.
 */
export function restore(doc: IndexDoc, m: Materialized, c: OpContext, ids: string[]): Result {
  if (!canWrite(c)) return fail("forbidden");
  const index = plain(doc);
  if (deletionMode(index) === "approval" && !isAdmin(c)) return fail("forbidden");
  const nodes = index.nodes ?? {};
  const trashedIds = m.trashed;
  const hiddenAbove = (id: string): string[] => {
    // The trashed ancestors of a node, from the top down, found in the raw tree.
    const chain: string[] = [];
    const seen = new Set<string>();
    for (
      let cur = nodes[id]?.parent ?? null;
      cur && !seen.has(cur);
      cur = nodes[cur]?.parent ?? null
    ) {
      seen.add(cur);
      if (trashedIds.has(cur)) chain.unshift(cur);
    }
    return chain;
  };
  const order: string[] = [];
  for (const id of ids) {
    if (!trashedIds.has(id)) return fail("not_found");
    for (const x of [...hiddenAbove(id), id]) if (!order.includes(x)) order.push(x);
  }
  const tag = c.restoredTag ?? "restored";
  const renames = new Map<string, string>();
  const keysByParent = new Map<string | null, Set<string>>();
  const items: MessageItem[] = [];
  for (const id of order) {
    const n = nodes[id] as IndexNode;
    const p = n.parent ?? null;
    let keys = keysByParent.get(p);
    if (!keys) {
      keys = takenKeys(m, nodes[p ?? ""]?.type === "dir" ? p : null);
      keysByParent.set(p, keys);
    }
    const name = restoredName(n.name, keys, tag);
    keys.add(collisionKey(name));
    if (name !== n.name) renames.set(id, name);
    items.push({ id, type: n.type, name });
  }
  return {
    ok: true,
    plan: {
      message: message(c, "restore", items),
      apply(d) {
        for (const it of items) {
          const n = d.nodes?.[it.id] as IndexNode;
          delete n.trashed;
          delete n.deletion;
          const renamed = renames.get(it.id);
          if (renamed !== undefined) n.name = imm(renamed);
        }
      },
    },
  };
}

/** Empties the trash of the given files for good (admins only). The history keeps the texts. */
export function purge(doc: IndexDoc, m: Materialized, c: OpContext, ids: string[]): Result {
  if (!isAdmin(c)) return fail("forbidden");
  const index = plain(doc);
  const gone = new Set<string>();
  const items: MessageItem[] = [];
  for (const id of ids) {
    const n = index.nodes?.[id];
    if (!n || !m.trashed.has(id)) return fail("not_found");
    if (id === m.mainId || id === index.main) return fail("main");
    items.push({ id, type: n.type, name: n.name });
    for (const x of m.subtree(id)) gone.add(x);
  }
  if (items.length === 0) return fail("unchanged");
  return {
    ok: true,
    plan: {
      message: message(c, "purge", items),
      apply(d) {
        for (const id of gone) {
          delete d.nodes?.[id];
          delete d.decisions?.[id];
        }
      },
    },
  };
}

/** Makes a text file the project's main document (admins only, FM-18). */
export function setMain(m: Materialized, c: OpContext, id: string): Result {
  if (!isAdmin(c)) return fail("forbidden");
  const e = m.byId.get(id);
  if (e?.type !== "text") return fail("not_found");
  if (m.mainId === id) return fail("unchanged");
  return {
    ok: true,
    plan: {
      message: message(c, "main", [{ id, type: e.type, name: e.name }]),
      apply(d) {
        d.main = imm(id);
      },
    },
  };
}

/** Changes the rule for deleting files (admins only, FM-16). */
export function setDeletionMode(doc: IndexDoc, c: OpContext, mode: DeletionMode): Result {
  if (!isAdmin(c)) return fail("forbidden");
  const index = plain(doc);
  if (deletionMode(index) === mode && index.settings) return fail("unchanged");
  return {
    ok: true,
    plan: {
      message: message(c, "mode", undefined, { mode }),
      apply(d) {
        d.settings = { deletion: imm(mode) as DeletionMode };
      },
    },
  };
}

/** A file on disk changed (an image replaced): the node keeps its identity, the hash is new (PRJ-13). */
export function replaceBlob(
  m: Materialized,
  c: OpContext,
  id: string,
  blob: { hash: string; size: number },
): Result {
  const e = target(m, c, id);
  if (isError(e)) return { ok: false, error: e };
  if (
    e.type !== "blob" ||
    !isHash(blob.hash) ||
    !Number.isSafeInteger(blob.size) ||
    blob.size < 0
  ) {
    return fail("invalid");
  }
  if (e.hash === blob.hash) return fail("unchanged");
  return {
    ok: true,
    plan: {
      message: message(c, "replace", [{ id, type: "blob", name: e.name }]),
      apply(d) {
        const n = d.nodes?.[id] as IndexNode;
        n.hash = imm(blob.hash);
        n.size = blob.size;
      },
    },
  };
}

/** True when `id` looks like something the index can hold; for callers that take ids from the UI. */
export const knownId = (m: Materialized, id: string): boolean => isId(id) && m.byId.has(id);
