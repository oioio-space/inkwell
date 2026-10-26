import { describe, expect, it } from "vitest";
import type { Entry } from "../../project/materialize";
import { fuzzy, score } from "./fuzzy";
import { buildRows, range, topmost } from "./rows";

let n = 0;
const entry = (path: string, type: Entry["type"] = "text"): Entry => {
  const parts = path.split("/");
  return {
    id: `id${String(n++).padStart(3, "0")}`,
    path,
    name: parts.at(-1) as string,
    type,
    parentId: null,
    size: 0,
    main: false,
    by: "x",
    at: 0,
  };
};

/** Entries with their parents linked by path. */
function tree(paths: string[]): Entry[] {
  const entries = paths.map((p) => entry(p.replace(/\/$/, ""), p.endsWith("/") ? "dir" : "text"));
  for (const e of entries) {
    const slash = e.path.lastIndexOf("/");
    if (slash >= 0) e.parentId = entries.find((x) => x.path === e.path.slice(0, slash))?.id ?? null;
  }
  return entries;
}

describe("rows of the tree", () => {
  it("UI-08 folders first, natural order, with position and size among siblings", () => {
    const rows = buildRows(tree(["b.md", "a/", "a/10.typ", "a/2.typ", "A2.md"]), new Set());
    expect(rows.map((r) => r.entry.path)).toEqual(["a", "a/2.typ", "a/10.typ", "A2.md", "b.md"]);
    expect(rows.map((r) => [r.depth, r.pos, r.size])).toEqual([
      [0, 1, 3],
      [1, 1, 2],
      [1, 2, 2],
      [0, 2, 3],
      [0, 3, 3],
    ]);
  });

  it("skips the content of a folded folder and says whether a folder has anything", () => {
    const entries = tree(["a/", "a/x.md", "empty/"]);
    const a = entries[0]?.id as string;
    const rows = buildRows(entries, new Set([a]));
    expect(rows.map((r) => r.entry.path)).toEqual(["a", "empty"]);
    expect(rows.map((r) => [r.expanded, r.hasChildren])).toEqual([
      [false, true],
      [true, false],
    ]);
  });

  it("ranges and the topmost of a selection", () => {
    const entries = tree(["d/", "d/a.md", "d/b.md", "z.md"]);
    const rows = buildRows(entries, new Set());
    const id = (p: string) => entries.find((e) => e.path === p)?.id as string;
    expect(range(rows, id("d/b.md"), id("d"))).toEqual([id("d"), id("d/a.md"), id("d/b.md")]);
    expect(range(rows, "gone", id("z.md"))).toEqual([id("z.md")]);
    // A folder and what is in it count once: acting on the folder acts on the rest.
    expect(topmost(rows, [id("d"), id("d/a.md"), id("z.md")])).toEqual([id("d"), id("z.md")]);
  });
});

describe("FM-20 finding a file by name", () => {
  it("matches the characters in order, prefers the file name and word starts", () => {
    expect(score("chfin", "chapitres/ch-fin.typ")).toBeDefined();
    expect(score("xyz", "chapitres/ch-fin.typ")).toBeUndefined();
    const best = fuzzy("not", ["docs/other/notebook.md", "notes.md", "a/n/o/t.md"], (p) => p);
    expect(best[0]?.item).toBe("notes.md");
    expect(best.map((b) => b.item)).toContain("a/n/o/t.md");
  });

  it("marks the matched characters", () => {
    expect(fuzzy("ab", ["xaxb"], (p) => p)[0]?.at).toEqual([1, 3]);
  });

  it("is case-insensitive", () => {
    expect(fuzzy("NOTES", ["Notes.md"], (p) => p)).toHaveLength(1);
  });

  it("FM-20 searches a thousand names in well under 50 ms", () => {
    const names = Array.from(
      { length: 1000 },
      (_, i) => `chapitres/part-${i % 37}/section-${i}.typ`,
    );
    fuzzy("sect9", names, (p) => p); // warm up
    const t0 = performance.now();
    const hits = fuzzy("sect9", names, (p) => p);
    const ms = performance.now() - t0;
    expect(hits.length).toBeGreaterThan(0);
    expect(ms).toBeLessThan(50);
  });
});
