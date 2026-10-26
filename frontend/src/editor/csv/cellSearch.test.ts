import { describe, expect, it } from "vitest";
import { compileQuery, type QuerySpec } from "../search/query";
import { findInCells, replacedValue } from "./cellSearch";
import { applySplices, CsvModel } from "./model";
import { findProblems } from "./problems";

const D = { delimiter: ";", header: true, decimal: "," } as const;
const spec = (text: string, over: Partial<QuerySpec> = {}): QuerySpec => ({
  text,
  mode: "normal",
  caseSensitive: false,
  wholeWord: false,
  ignoreAccents: false,
  dotAll: false,
  ...over,
});
const matcher = (text: string, over: Partial<QuerySpec> = {}) => {
  const r = compileQuery(spec(text, over));
  if (!r.ok) throw new Error(r.error);
  return r.matcher;
};

const TEXT = 'Nom;Note;Prix\nPomme;"a;b";1,5\nPoire;pomme verte;2\n"Cerise";POMME;3\n';

describe("search and replace on cells (CSV-EDGE-16..19)", () => {
  const m = new CsvModel(TEXT, D);

  it("CSV-EDGE-16 scopes: sheet, column, selection, headers", () => {
    const q = matcher("pomme");
    expect(findInCells(m, q, { kind: "sheet" }).map((h) => `${h.r}:${h.c}`)).toEqual([
      "1:0",
      "2:1",
      "3:1",
    ]);
    expect(findInCells(m, q, { kind: "column", col: 1 }).map((h) => h.r)).toEqual([2, 3]);
    expect(
      findInCells(m, q, { kind: "selection", ranges: [{ r0: 1, c0: 0, r1: 2, c1: 0 }] }).map(
        (h) => h.r,
      ),
    ).toEqual([1]);
    expect(findInCells(m, matcher("nom"), { kind: "headers" }).map((h) => h.c)).toEqual([0]);
    expect(findInCells(m, matcher("nom"), { kind: "sheet" })).toEqual([]);
  });

  it("CSV-EDGE-17 options: entire cell, case, accents, regex and wildcards", () => {
    expect(
      findInCells(m, matcher("pomme", { caseSensitive: true }), { kind: "sheet" }).map(
        (h) => h.value,
      ),
    ).toEqual(["pomme verte"]);
    expect(
      findInCells(m, matcher("POMME"), { kind: "sheet" }, { entireCell: true }).map(
        (h) => `${h.r}:${h.c}`,
      ),
    ).toEqual(["1:0", "3:1"]);
    expect(
      findInCells(m, matcher("cérise", { ignoreAccents: true }), { kind: "sheet" }).map(
        (h) => h.value,
      ),
    ).toEqual(["Cerise"]);
    expect(
      findInCells(
        m,
        matcher("^p.*e$", { mode: "regex" }),
        { kind: "sheet" },
        { entireCell: true },
      ).map((h) => h.value),
    ).toEqual(["Pomme", "Poire", "pomme verte", "POMME"]);
    expect(
      findInCells(m, matcher("po?re", { mode: "wildcard" }), { kind: "sheet" }).map((h) => h.value),
    ).toEqual(["Poire"]);
  });

  it("CSV-EDGE-18 hits name their cell and value, in file order", () => {
    const hits = findInCells(m, matcher("a"), { kind: "column", col: 0 });
    expect(hits.map((h) => h.value)).toEqual(
      ["Pomme", "Poire", "Cerise"].filter((v) => /a/i.test(v)),
    );
  });

  it("the separator and the quotes of the file never match", () => {
    expect(findInCells(m, matcher(";"), { kind: "sheet" })).toHaveLength(1); // only the cell a;b
    expect(findInCells(m, matcher('"'), { kind: "sheet" })).toEqual([]);
  });

  it("CSV-EDGE-19 replacing re-quotes as needed and never breaks the structure", () => {
    const hits = findInCells(m, matcher("pomme"), { kind: "sheet" });
    const edits = hits.map((h) => ({ r: h.r, c: h.c, value: replacedValue(h, "x;y\nz") }));
    const out = applySplices(TEXT, m.editCells(edits));
    const after = new CsvModel(out, D);
    expect(after.records).toBe(4);
    expect(after.width).toBe(3);
    expect(after.value(1, 0)).toBe("x;y\nz");
    expect(after.value(2, 1)).toBe("x;y\nz verte");
    expect(out).toContain('"x;y\nz"');
    // The cells nobody changed keep their bytes.
    expect(out).toContain("Poire;");
    expect(out.endsWith("3\n")).toBe(true);
  });

  it("replacement expands $1 in regex mode and \\t in extended mode", () => {
    const hit = findInCells(m, matcher("(p)omme", { mode: "regex" }), {
      kind: "column",
      col: 0,
    })[0];
    expect(hit && replacedValue(hit, "$1-x", { regex: true })).toBe("P-x");
    expect(hit && replacedValue(hit, "a\\tb", { extended: true })).toBe("a\tb");
  });
});

describe("problems with the quoting (CSV-EDGE-04) and never losing a byte", () => {
  const find = (text: string) => findProblems(new CsvModel(text, D));

  it("an unclosed quote is reported with its line, and the text is untouched", () => {
    const text = 'a;b\n1;"never closed\n3;4\n';
    expect(find(text)).toEqual([{ line: 2, kind: "unclosedQuote" }]);
    const m = new CsvModel(text, D);
    expect(m.text).toBe(text);
    expect(m.records).toBe(2); // the quote swallows the rest, as a lenient reader does
  });

  it("a quote in the middle of a cell and a backslash-escaped quote", () => {
    expect(find('a;b\n5" pipe;2\n')).toEqual([{ line: 2, kind: "strayQuote" }]);
    expect(find('a;b\nsay \\"hi\\";2\n')).toEqual([{ line: 2, kind: "backslashQuote" }]);
    expect(find('a;b\n"x"y;2\n')).toEqual([{ line: 2, kind: "strayQuote" }]);
    expect(find('a;b\n"ok ""quoted""";2\n')).toEqual([]);
  });

  it("a file with no quote costs nothing and has no problem", () => {
    expect(find("a;b\n1;2\n")).toEqual([]);
  });
});
