import { next as A } from "@automerge/automerge";
import { generateAutomergeUrl } from "@automerge/automerge-repo";
import { materialize } from "./materialize";
import { migration } from "./migrate";
import { type IndexDoc, plain, type Role } from "./model";
import type { OpContext, Plan, Result } from "./ops";

/** Test helpers: people with roles, an index document, and plans applied to it. */
export interface Person {
  id: string;
  name: string;
  role: Role;
}

export const alice: Person = { id: "alice", name: "Alice", role: "admin" };
export const admin2: Person = { id: "dora", name: "Dora", role: "admin" };
export const bob: Person = { id: "bob", name: "Bob", role: "writer" };
export const carol: Person = { id: "carol", name: "Carol", role: "reader" };

export const ctx = (p: Person, now = 1_000): OpContext => ({
  me: p.id,
  name: p.name,
  role: p.role,
  now,
  restoredTag: "restauré",
});

export type Index = A.Doc<IndexDoc>;

/** A migrated project: the root document with its `main.typ` node. */
export function newProject(rootUrl = generateAutomergeUrl()): Index {
  const doc = A.from<IndexDoc>({ text: "" });
  const m = migration(doc, rootUrl, alice.id, 0);
  if (!m) throw new Error("already migrated");
  return A.change(doc, m);
}

/** Applies a plan, as a client does: one change, its message attached. */
export function apply(doc: Index, r: Result): Index {
  if (!r.ok) throw new Error(`refused: ${JSON.stringify(r.error)}`);
  return run(doc, r.plan);
}

export function run(doc: Index, plan: Plan): Index {
  return A.change(doc, { message: JSON.stringify(plan.message), time: 1 }, plan.apply);
}

/** Merges two replicas both ways; returns the one that has everything. */
export function sync(a: Index, b: Index): [Index, Index] {
  const merged = A.merge(A.clone(a), A.clone(b));
  return [merged, A.clone(merged)];
}

export const view = (doc: Index) => materialize(plain(doc));
export const paths = (doc: Index) => view(doc).entries.map((e) => e.path);

/** The id of the visible entry at `path`. */
export function idOf(doc: Index, path: string): string {
  const e = view(doc).byPath.get(path);
  if (!e) throw new Error(`no ${path} in ${paths(doc).join(", ")}`);
  return e.id;
}

export const textUrl = () => generateAutomergeUrl();
export const HASH = `blake3:${"ab".repeat(32)}`;
