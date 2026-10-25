import { next as A } from "@automerge/automerge";
import { describe, expect, it, vi } from "vitest";
import { FileKind } from "../../store/files";
import { findInProject, parseFilters, resolveHit } from "./project";
import { compile, typst } from "./testkit";

describe("project search", () => {
  it("SRC-10 finds in 40 files / 5 MB in under 300 ms, grouped by file, with the right count", async () => {
    const line = "Le serveur répond à la requête normalement, rien à signaler ici.\n";
    const files = Array.from({ length: 40 }, (_, i) => {
      // ~125 KB each; one needle every 500 lines in half of the files.
      let text = "";
      for (let n = 0; text.length < 125_000; n++) {
        text += n % 500 === 0 && i % 2 === 0 ? `ici NEEDLE ${n}\n` : line;
      }
      return typst(`docs/f${i}.typ`, text);
    });
    const total = files.reduce((n, f) => n + f.text().length, 0);
    expect(total).toBeGreaterThan(4_900_000);
    const expected = files.reduce(
      (n, f, i) => n + (i % 2 === 0 ? (f.text().match(/NEEDLE/g) ?? []).length : 0),
      0,
    );

    const matcher = compile("needle");
    // Best of three: the budget is about the engine, not about a busy test machine.
    let ms = Number.POSITIVE_INFINITY;
    let res = await findInProject(files, matcher);
    for (let i = 0; i < 3; i++) {
      const t0 = performance.now();
      res = await findInProject(files, matcher);
      ms = Math.min(ms, performance.now() - t0);
    }
    if ((globalThis as { process?: { env: Record<string, string | undefined> } }).process?.env.CI)
      console.info(`SRC-10: 40 files / ${(total / 1e6).toFixed(1)} MB in ${ms.toFixed(0)} ms`);

    expect(ms).toBeLessThan(300);
    expect(res.files).toHaveLength(20);
    expect(res.total).toBe(expected);
    expect(res.files.every((f) => f.path.startsWith("docs/"))).toBe(true);
    expect(res.scanned).toBe(40);
  });

  it("SRC-12 a result follows a peer's edit, and goes stale when the words change", async () => {
    let doc = A.from({ text: "alpha\nbeta foo gamma\nomega foo" });
    const res = await findInProject([typst("a.typ", doc.text)], compile("foo"), "", {
      anchor: () => doc,
    });
    const hits = res.files[0]?.hits ?? [];
    expect(hits).toHaveLength(2);
    const [first, second] = hits as [(typeof hits)[number], (typeof hits)[number]];
    expect(resolveHit(doc, first)).toEqual({ from: 11, to: 14, stale: false });

    // A peer adds a line above: the result moves with its words.
    let peer = A.clone(doc);
    peer = A.change(peer, (d) => A.splice(d, ["text"], 0, 0, "NEW LINE\n"));
    doc = A.merge(doc, peer);
    const moved = resolveHit(doc, first);
    expect(moved).toEqual({ from: 20, to: 23, stale: false });
    expect(doc.text.slice(moved.from, moved.to)).toBe("foo");

    // A peer types at the very edge of the words: still the same words, not stale.
    peer = A.clone(doc);
    peer = A.change(peer, (d) => A.splice(d, ["text"], 20, 0, ">"));
    peer = A.change(peer, (d) => A.splice(d, ["text"], 24, 0, "<"));
    doc = A.merge(doc, peer);
    expect(resolveHit(doc, first).stale).toBe(false);

    // A peer edits inside the words: greyed out, never a wrong jump.
    peer = A.clone(doc);
    peer = A.change(peer, (d) => A.splice(d, ["text"], 22, 1, "X"));
    doc = A.merge(doc, peer);
    expect(resolveHit(doc, first).stale).toBe(true);
    expect(resolveHit(doc, second).stale).toBe(false);

    // A peer deletes the line: the other result is stale and resolving never throws.
    peer = A.clone(doc);
    peer = A.change(peer, (d) => A.splice(d, ["text"], 0, d.text.length, ""));
    doc = A.merge(doc, peer);
    expect(() => resolveHit(doc, second)).not.toThrow();
    expect(resolveHit(doc, second).stale).toBe(true);
  });

  it("a hit without a document is checked in place", () => {
    const doc = A.from({ text: "foo bar" });
    const hit = { from: 4, to: 7, matched: "bar" } as Parameters<typeof resolveHit>[1];
    expect(resolveHit(doc, hit).stale).toBe(false);
    expect(resolveHit(A.from({ text: "foo baz" }), hit).stale).toBe(true);
    expect(resolveHit(A.from({ text: "foo" }), hit).stale).toBe(true);
  });

  it("SRC-19 file filters apply and binaries are never read", async () => {
    const text = vi.fn(() => "needle");
    const binary = (path: string, kind: FileKind) => ({ path, kind, text: vi.fn(() => "needle") });
    const png = binary("fig/needle.png", FileKind.image);
    const pdf = binary("out.pdf", FileKind.pdf);
    const blob = binary("data.bin", FileKind.data);
    const files = [
      { path: "ch1.typ", kind: FileKind.typst, text },
      { path: "notes/a.md", kind: FileKind.markdown, text },
      { path: "brouillons/b.typ", kind: FileKind.typst, text },
      { path: "x/brouillons/c.typ", kind: FileKind.typst, text },
      { path: "mybrouillons/d.typ", kind: FileKind.typst, text },
      { path: "refs.bib", kind: FileKind.bib, text },
      png,
      pdf,
      blob,
    ];
    const m = compile("needle");

    const all = await findInProject(files, m);
    expect(all.files.map((f) => f.path)).toEqual([
      "ch1.typ",
      "notes/a.md",
      "brouillons/b.typ",
      "x/brouillons/c.typ",
      "mybrouillons/d.typ",
      "refs.bib",
    ]);
    for (const b of [png, pdf, blob]) expect(b.text).not.toHaveBeenCalled();

    const typOnly = await findInProject(files, m, "*.typ !brouillons/");
    expect(typOnly.files.map((f) => f.path)).toEqual(["ch1.typ", "mybrouillons/d.typ"]);
    const two = await findInProject(files, m, "*.typ *.md");
    expect(two.files.map((f) => f.path)).toEqual([
      "ch1.typ",
      "notes/a.md",
      "brouillons/b.typ",
      "x/brouillons/c.typ",
      "mybrouillons/d.typ",
    ]);
    // Even a filter that names the binary does not make it readable.
    await findInProject(files, m, "*.png");
    expect(png.text).not.toHaveBeenCalled();
  });

  it("filters understand paths and negation", () => {
    const f = parseFilters("src/**/*.typ !*.bak");
    expect(f("src/a/b.typ")).toBe(true);
    expect(f("lib/b.typ")).toBe(false);
    expect(f("src/a/b.typ.bak")).toBe(false);
    expect(parseFilters("")("anything")).toBe(true);
    expect(parseFilters("!drafts/")("drafts/a.typ")).toBe(false);
  });

  it("'search in these results' keeps only the lines of the earlier results", async () => {
    const f = typst("a.typ", "foo bar\nfoo\nbar foo bar");
    const first = await findInProject([f], compile("foo"));
    const lines = new Map([["a.typ", new Set(first.files[0]?.hits.map((h) => h.line))]]);
    const again = await findInProject([f, typst("b.typ", "bar")], compile("bar"), "", {
      restrictTo: lines,
    });
    expect(again.files.map((x) => [x.path, x.hits.map((h) => h.line)])).toEqual([
      ["a.typ", [1, 3, 3]],
    ]);
  });
});
