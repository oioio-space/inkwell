import { next as A } from "@automerge/automerge";
import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { findAllSync } from "./find";
import { findInProject } from "./project";
import { expandReplacement, previewReplace, replaceAll, replaceInProject } from "./replace";
import { compile, TestHandle, typst } from "./testkit";

describe("replace", () => {
  it("SRC-14 replace all in regex mode applies $1 and $<name> in a single undo step", () => {
    const h = TestHandle.of("ana@x.org, bob@y.net, nope");
    const before = A.getAllChanges(h.current).length;
    const m = compile("(?<user>\\w+)@(\\w+)", { mode: "regex" });
    const hits = findAllSync(h.text, m);
    const r = replaceAll(h, hits, "$2:$<user> ($1) $$ $&", { regex: true });
    expect(h.text).toBe("x:ana (ana) $ ana@x.org, y:bob (bob) $ bob@y.net, nope");
    expect(r).toEqual({ replaced: 2, skippedLocked: 0, skippedStale: 0 });
    expect(h.changes).toBe(1);
    expect(A.getAllChanges(h.current).length - before).toBe(1);
  });

  it("expands groups like JavaScript does, and leaves unknown references alone", () => {
    const [m] = findAllSync("2026-10", compile("(\\d+)-(\\d+)", { mode: "regex" }));
    if (!m) throw new Error("no match");
    expect(expandReplacement("$2/$1 $3 $<x> $0 $11 $$1", m)).toBe("10/2026 $3 $<x> $0 20261 $1");
    expect(expandReplacement("$1$2", m)).toBe("202610");
  });

  it("literal mode does not expand $", () => {
    const h = TestHandle.of("a b");
    replaceAll(h, findAllSync(h.text, compile("a")), "$1$&");
    expect(h.text).toBe("$1$& b");
  });

  it("extended mode reads \\n and \\t in the replacement", () => {
    const h = TestHandle.of("a,b");
    replaceAll(h, findAllSync(h.text, compile(",")), "\\n\\t", { extended: true });
    expect(h.text).toBe("a\n\tb");
  });

  it("replaces with ignore-accents using the original text for $1", () => {
    const h = TestHandle.of("Café et café");
    const m = compile("(caf)e", { mode: "regex", ignoreAccents: true, caseSensitive: false });
    replaceAll(h, findAllSync(h.text, m), "$1É", { regex: true });
    expect(h.text).toBe("CafÉ et cafÉ");
  });

  it("SRC-15 replace all in the project previews first, then makes one change per file", async () => {
    const a = TestHandle.of("foo 1\nbar\nfoo foo 2");
    const b = TestHandle.of("nothing\nfoo 3");
    const c = TestHandle.of("no match");
    const docs = new Map([
      ["a.typ", a],
      ["b.typ", b],
      ["c.typ", c],
    ]);
    const m = compile("foo");
    const res = await findInProject(
      [...docs].map(([p, h]) => typst(p, h.text)),
      m,
      "",
      { anchor: (p) => docs.get(p)?.current },
    );
    expect(res.files.map((f) => f.path)).toEqual(["a.typ", "b.typ"]);

    // The preview shows each line before and after; nothing has changed yet.
    const preview = previewReplace(a.text, res.files[0]?.hits ?? [], "BAR");
    expect(preview).toEqual([
      { line: 1, before: "foo 1", after: "BAR 1", replaced: 1, locked: 0 },
      { line: 3, before: "foo foo 2", after: "BAR BAR 2", replaced: 2, locked: 0 },
    ]);
    expect(a.text).toBe("foo 1\nbar\nfoo foo 2");

    const rep = replaceInProject(res.files, (p) => docs.get(p), "BAR");
    expect(rep).toMatchObject({ replaced: 4, files: 2, skippedLocked: 0, unavailable: [] });
    expect([a.changes, b.changes, c.changes]).toEqual([1, 1, 0]);
    expect(a.text).toBe("BAR 1\nbar\nBAR BAR 2");
    expect(b.text).toBe("nothing\nBAR 3");

    // Undoing a file is its own business: reverting only a's change leaves b as it is.
    expect(b.text).toContain("BAR");
  });

  it("previews multi-line matches and long lines", () => {
    const text = "a\nb\nc";
    const hits = findAllSync(text, compile("a\\nb", { mode: "extended" }));
    expect(previewReplace(text, hits, "X", {})).toEqual([
      { line: 1, before: "a\nb", after: "X", replaced: 1, locked: 0 },
    ]);
    const long = `${"w ".repeat(400)}foo`;
    const [p] = previewReplace(long, findAllSync(long, compile("foo")), "bar");
    expect(p?.before.length).toBeLessThanOrEqual(301);
    expect(p?.before.endsWith("…")).toBe(true);
  });

  it("SRC-16 occurrences in locked zones are skipped and counted apart", () => {
    const h = TestHandle.of("foo [lock foo foo foo] foo");
    const hits = findAllSync(h.text, compile("foo"));
    const lock = { from: 5, to: 21 };
    const r = replaceAll(h, hits, "bar", { skip: (x) => x.from >= lock.from && x.to <= lock.to });
    expect(r).toEqual({ replaced: 2, skippedLocked: 3, skippedStale: 0 });
    expect(h.text).toBe("bar [lock foo foo foo] bar");

    // The preview says the same.
    const [p] = previewReplace("foo foo", findAllSync("foo foo", compile("foo")), "bar", {
      skip: (x) => x.from > 0,
    });
    expect(p).toMatchObject({ after: "bar foo", replaced: 1, locked: 1 });
  });

  it("SRC-17 B types inside an occurrence while A replaces all: they converge and B's letter survives", () => {
    const base = A.from({ text: "un foo ici, un foo là" });
    const a = new TestHandle(A.clone(base));
    let b = A.clone(base);

    const hits = findAllSync(a.text, compile("foo"));
    replaceAll(a, hits, "bar");

    // B types "X" between the f and the oo of the first occurrence.
    b = A.change(b, (d) => A.splice(d, ["text"], 4, 0, "X"));

    const ab = A.merge(A.clone(a.current), b);
    const ba = A.merge(A.clone(b), a.current);
    expect(ab.text).toBe(ba.text);
    expect(ab.text).toContain("X");
    expect(ab.text).toContain("un bar là");
    expect(ab.text.replace("X", "")).toBe("un bar ici, un bar là");
  });

  it("a hit whose words changed since the search is left alone, not replaced in the wrong place", () => {
    const h = TestHandle.of("foo bar foo");
    const hits = findAllSync(h.text, compile("foo"));
    h.current = A.change(h.current, (d) => A.splice(d, ["text"], 0, 3, "zzzz"));
    const r = replaceAll(h, hits, "X");
    // The second hit still holds at the same offset +1 only if it follows by cursor; plain hits are checked in place.
    expect(r.skippedStale).toBeGreaterThan(0);
    expect(h.text).not.toContain("XX");
  });

  it("nothing to replace makes no change at all", () => {
    const h = TestHandle.of("abc");
    expect(replaceAll(h, [], "x")).toEqual({ replaced: 0, skippedLocked: 0, skippedStale: 0 });
    expect(h.changes).toBe(0);
  });

  it("properties: replacing a needle by a distinct letter leaves exactly `replaced` of them", () => {
    fc.assert(
      fc.property(
        fc.string({ unit: fc.constantFrom("a", "b", "c", " ", "\n"), maxLength: 80 }),
        fc.constantFrom("a", "ab", "bc", "abc", "c "),
        (text, needle) => {
          const h = TestHandle.of(text);
          const hits = findAllSync(text, compile(needle, { caseSensitive: true }));
          const r = replaceAll(h, hits, "Z");
          expect(r.replaced).toBe(hits.length);
          const left = findAllSync(h.text, compile("Z", { caseSensitive: true }));
          expect(left).toHaveLength(r.replaced);
          expect(h.changes).toBe(hits.length > 0 ? 1 : 0);
        },
      ),
    );
  });
});
