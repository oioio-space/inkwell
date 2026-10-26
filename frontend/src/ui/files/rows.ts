import type { Entry } from "../../project/materialize";

/** A line of the file tree, with what an ARIA tree needs to describe it. */
export interface Row {
  entry: Entry;
  /** 0 at the root. */
  depth: number;
  /** Position among its siblings, from 1, and how many there are (aria-posinset, aria-setsize). */
  pos: number;
  size: number;
  /** For a folder: whether its content is shown. */
  expanded: boolean;
  /** For a folder: whether it holds anything. */
  hasChildren: boolean;
}

const natural = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

/** Folders first, then files, each in natural order ("2.typ" before "10.typ"). */
function compare(a: Entry, b: Entry): number {
  const folders = (a.type === "dir" ? 0 : 1) - (b.type === "dir" ? 0 : 1);
  return folders || natural.compare(a.name, b.name) || (a.id < b.id ? -1 : 1);
}

/** The rows of the tree in display order, skipping the content of the folders in `collapsed`. */
export function buildRows(entries: readonly Entry[], collapsed: ReadonlySet<string>): Row[] {
  const kids = new Map<string | null, Entry[]>();
  for (const e of entries) {
    const list = kids.get(e.parentId);
    if (list) list.push(e);
    else kids.set(e.parentId, [e]);
  }
  for (const list of kids.values()) list.sort(compare);
  const rows: Row[] = [];
  const walk = (parent: string | null, depth: number): void => {
    const list = kids.get(parent) ?? [];
    list.forEach((entry, i) => {
      const hasChildren = (kids.get(entry.id)?.length ?? 0) > 0;
      const expanded = entry.type === "dir" && !collapsed.has(entry.id);
      rows.push({ entry, depth, pos: i + 1, size: list.length, expanded, hasChildren });
      if (entry.type === "dir" && expanded) walk(entry.id, depth + 1);
    });
  };
  walk(null, 0);
  return rows;
}

/** The ids from `a` to `b` in the rows, inclusive, in display order (a range selection). */
export function range(rows: readonly Row[], a: string, b: string): string[] {
  const i = rows.findIndex((r) => r.entry.id === a);
  const j = rows.findIndex((r) => r.entry.id === b);
  if (i < 0 || j < 0) return j >= 0 ? [b] : [];
  const [from, to] = i < j ? [i, j] : [j, i];
  return rows.slice(from, to + 1).map((r) => r.entry.id);
}

/**
 * Of a selection, what an action should apply to: a folder and the files in it count once, since
 * acting on the folder acts on what it holds.
 */
export function topmost(rows: readonly Row[], ids: Iterable<string>): string[] {
  const chosen = new Set(ids);
  const byId = new Map(rows.map((r) => [r.entry.id, r.entry]));
  const underChosen = (id: string): boolean => {
    for (let p = byId.get(id)?.parentId ?? null; p !== null; p = byId.get(p)?.parentId ?? null) {
      if (chosen.has(p)) return true;
    }
    return false;
  };
  return rows
    .filter((r) => chosen.has(r.entry.id) && !underChosen(r.entry.id))
    .map((r) => r.entry.id);
}
