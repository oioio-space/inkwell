import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { findAllSync } from "./find";
import { compileQuery, type QuerySpec } from "./query";

const base: QuerySpec = {
  text: "",
  mode: "normal",
  caseSensitive: false,
  wholeWord: false,
  ignoreAccents: false,
  dotAll: false,
};

function matcher(text: string, over: Partial<QuerySpec> = {}) {
  const r = compileQuery({ ...base, text, ...over });
  if (!r.ok) throw new Error(r.error);
  return r.matcher;
}

/** The matched words of every occurrence, as written in the document. */
function words(doc: string, text: string, over: Partial<QuerySpec> = {}) {
  return findAllSync(doc, matcher(text, over)).map((m) => m.matched);
}

describe("search query", () => {
  it("SRC-01 normal mode without case finds Réplica and RÉPLICA, and replica only without accents", () => {
    const doc = "Réplica, RÉPLICA, replica, réplica";
    expect(words(doc, "réplica")).toEqual(["Réplica", "RÉPLICA", "réplica"]);
    expect(words(doc, "réplica", { ignoreAccents: true })).toEqual([
      "Réplica",
      "RÉPLICA",
      "replica",
      "réplica",
    ]);
    expect(words(doc, "réplica", { caseSensitive: true })).toEqual(["réplica"]);
  });

  it("SRC-02 whole word DNS is not found inside DNSSEC", () => {
    const doc = "DNS, DNSSEC, un DNS. (DNS) sansDNS DNS_x";
    const found = findAllSync(doc, matcher("DNS", { wholeWord: true }));
    expect(found.map((m) => m.from)).toEqual([0, 16, 22]);
  });

  it("SRC-03 wildcard pg_*promote stays on one line", () => {
    const doc = "pg_ctl promote\npg_\npromote\npg_x promote pg_y promote";
    const found = words(doc, "pg_*promote", { mode: "wildcard" });
    expect(found).toEqual(["pg_ctl promote", "pg_x promote", "pg_y promote"]);
  });

  it("SRC-04 regex metacharacters in a wildcard query are literal", () => {
    const doc = "a.b(c) aXb(c) a.b(cc)";
    expect(words(doc, "a.b(c)", { mode: "wildcard" })).toEqual(["a.b(c)"]);
    expect(words(doc, "a?b[c]c", { mode: "wildcard" })).toEqual([]);
    expect(words("a[b", "a[b", { mode: "wildcard" })).toEqual(["a[b"]);
    expect(words("xay xby xcy", "x[ab]y", { mode: "wildcard" })).toEqual(["xay", "xby"]);
    expect(words("xay xby xcy", "x[!ab]y", { mode: "wildcard" })).toEqual(["xcy"]);
  });

  it("SRC-05 regex ^\\+ (.+)$ gives one occurrence per list line", () => {
    const doc = "+ un\ntexte\n+ deux\n- non\n+ trois";
    const found = findAllSync(doc, matcher("^\\+ (.+)$", { mode: "regex" }));
    expect(found.map((m) => m.groups[0])).toEqual(["un", "deux", "trois"]);
    expect(found.map((m) => m.line)).toEqual([1, 3, 5]);
  });

  it("SRC-06 an invalid regex gives a clear message, not an exception", () => {
    const r = compileQuery({ ...base, text: "(abc", mode: "regex" });
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.error).toMatch(/parenthèse non fermée/i);
    for (const bad of ["abc)", "*a", "[a", "a{2,1}", "\\", "(?<n", "[z-a]", "\\p{Nope}"]) {
      const res = compileQuery({ ...base, text: bad, mode: "regex" });
      expect(res.ok).toBe(false);
      if (!res.ok) expect(res.error).toMatch(/^[A-ZÉ].*\.$/);
    }
  });

  it("SRC-08 regexes that match nothing step forward without looping", () => {
    expect(
      findAllSync("baab", matcher("a*", { mode: "regex" })).map((m) => [m.from, m.to]),
    ).toEqual([
      [0, 0],
      [1, 3],
      [3, 3],
      [4, 4],
    ]);
    const lines = findAllSync("a\n\nb", matcher("^", { mode: "regex" }));
    expect(lines.map((m) => m.from)).toEqual([0, 2, 3]);
    // Over an emoji the scan steps a whole code point.
    expect(findAllSync("😀", matcher("", { mode: "regex" }))).toEqual([]);
    expect(findAllSync("😀x", matcher("x*", { mode: "regex" })).map((m) => m.from)).toEqual([
      0, 2, 3,
    ]);
  });

  it("SRC-09 Unicode: exact positions for emoji, CJK, composed and decomposed accents", () => {
    const doc = "😀 café 日本語 café end";
    const emoji = findAllSync(doc, matcher("😀"))[0];
    expect([emoji?.from, emoji?.to]).toEqual([0, 2]);
    const cjk = findAllSync(doc, matcher("日本"))[0];
    expect(doc.slice(cjk?.from, cjk?.to)).toBe("日本");
    expect(cjk?.column).toBe(9);

    // NFC and NFD forms are found alike, with exact original offsets.
    const both = findAllSync(doc, matcher("cafe", { ignoreAccents: true }));
    expect(both.map((m) => m.matched)).toEqual(["café", "café"]);
    expect(both.map((m) => [m.from, m.to])).toEqual([
      [3, 7],
      [12, 17],
    ]);
    // A decomposed query finds composed text too.
    const nfd = findAllSync("café", matcher("café", { ignoreAccents: true }));
    expect(nfd.map((m) => [m.from, m.to])).toEqual([[0, 4]]);
    // Not folding Hangul or kana: が stays distinct from か.
    expect(words("か が", "か", { ignoreAccents: true })).toEqual(["か"]);
  });

  it("SRC-13 a search in the selection finds nothing outside it", () => {
    const doc = "foo foo foo foo";
    const m = matcher("foo");
    expect(findAllSync(doc, m, { from: 4, to: 11 }).map((x) => x.from)).toEqual([4, 8]);
    expect(findAllSync(doc, m, { from: 5, to: 10 })).toEqual([]);
    // Word boundaries still see the text outside the selection.
    expect(findAllSync("foobar", matcher("foo", { wholeWord: true }), { from: 0, to: 3 })).toEqual(
      [],
    );
    // Same inside an accent-folded view.
    const accented = "été été été";
    expect(
      findAllSync(accented, matcher("ete", { ignoreAccents: true }), { from: 4, to: 7 }).map(
        (x) => x.from,
      ),
    ).toEqual([4]);
  });

  it("SRC-22 normal mode searches the markup too; the visible-text option hides it", () => {
    const doc = "un **mot** gras et mot **gras** puis `code_x` pg_ctl";
    // By default the markup is part of the text.
    expect(words(doc, "**mot**")).toEqual(["**mot**"]);
    expect(findAllSync(doc, matcher("mot"))[0]?.from).toBe(5);
    // As it appears: "mot gras" is not found in "**mot** gras" in plain mode…
    expect(words(doc, "mot gras")).toEqual([]);
    // …but is with the visible-text option, and the span covers the marks inside it.
    const seen = findAllSync(doc, matcher("mot gras", { visibleText: true }));
    expect(seen.map((m) => m.matched)).toEqual(["mot** gras", "mot **gras"]);
    // Underscores inside names stay.
    expect(words(doc, "pg_ctl", { visibleText: true })).toEqual(["pg_ctl"]);
    expect(words(doc, "codex", { visibleText: true })).toEqual([]);
  });

  it("extended mode reads \\n and \\t in the query", () => {
    expect(words("a\tb\nc", "b\\nc", { mode: "extended" })).toEqual(["b\nc"]);
    expect(words("a\tb", "a\\tb", { mode: "extended" })).toEqual(["a\tb"]);
    expect(words("a\\tb", "a\\tb")).toEqual(["a\\tb"]);
  });

  it("regex dotAll lets the dot cross lines", () => {
    expect(words("a\nb", "a.b", { mode: "regex" })).toEqual([]);
    expect(words("a\nb", "a.b", { mode: "regex", dotAll: true })).toEqual(["a\nb"]);
  });

  it("an empty query matches nothing", () => {
    expect(findAllSync("abc", matcher(""))).toEqual([]);
  });

  it("long lines are cut around the hit and stay exact", () => {
    const doc = `${"x".repeat(500)}NEEDLE${"y".repeat(500)}`;
    const [m] = findAllSync(doc, matcher("NEEDLE"));
    expect(m?.lineText.length).toBeLessThan(200);
    expect(m?.lineText.startsWith("…")).toBe(true);
    expect(m?.lineText.slice(m.hitFrom, m.hitTo)).toBe("NEEDLE");
    expect(m?.column).toBe(501);
  });

  describe("properties", () => {
    const plain = fc
      .string({ unit: "grapheme-ascii", maxLength: 12 })
      .filter((s) => !/[*?[\n]/.test(s));
    const doc = fc.string({ unit: "grapheme", maxLength: 60 });

    it("a wildcard without metacharacters is the same as normal mode", () => {
      fc.assert(
        fc.property(plain, doc, (needle, text) => {
          const a = findAllSync(text, matcher(needle, { mode: "wildcard" }));
          const b = findAllSync(text, matcher(needle));
          expect(a.map((m) => [m.from, m.to])).toEqual(b.map((m) => [m.from, m.to]));
        }),
      );
    });

    it("every match is a real slice of the document, in order, with exact lines", () => {
      fc.assert(
        fc.property(
          plain,
          fc.string({ unit: "grapheme", maxLength: 80 }),
          fc.boolean(),
          (needle, text, acc) => {
            const found = findAllSync(text, matcher(needle, { ignoreAccents: acc }));
            let last = -1;
            for (const m of found) {
              expect(text.slice(m.from, m.to)).toBe(m.matched);
              expect(m.from).toBeGreaterThanOrEqual(last);
              last = m.from;
              expect(m.line).toBe(text.slice(0, m.from).split("\n").length);
            }
          },
        ),
      );
    });
  });
});
