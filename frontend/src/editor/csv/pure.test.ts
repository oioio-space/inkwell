import { describe, expect, it } from "vitest";
import { colName, parseRef, rangeRef } from "./address";
import {
  cellsOf,
  fromClipboard,
  htmlTable,
  looksLikeFormula,
  neutralise,
  toHtml,
  toTsv,
} from "./clipboard";
import { columnType, detectDialect, parseNumber } from "./dialect";
import { applySplices, CsvModel } from "./model";
import { continueSeries, today } from "./series";
import { displayNumber, formatFr, selectionStats } from "./stats";
import { csvChartCode, csvTableCode } from "./typst";
import { buildView, type ColumnFilter } from "./view";

describe("dialect", () => {
  it("CSV-01 detects `;`, header and decimal comma", () => {
    const d = detectDialect("Nom;Prix;Date\nPomme;1,50;2026-01-02\nPoire;2,25;2026-01-03\n");
    expect(d).toEqual({ delimiter: ";", header: true, decimal: "," });
  });

  it("CSV-EDGE-01 each delimiter, a single column, and a decimal-comma file with `;`", () => {
    expect(detectDialect("a,b,c\n1,2,3\n").delimiter).toBe(",");
    expect(detectDialect("a\tb\tc\n1\t2\t3\n").delimiter).toBe("\t");
    expect(detectDialect("a|b|c\n1|2|3\n").delimiter).toBe("|");
    expect(detectDialect("a;b\n1,5;2,5\n3,5;4,5\n").delimiter).toBe(";");
    expect(detectDialect("seul\n1\n2\n").delimiter).toBe(",");
  });

  it("CSV-EDGE-06 no header when the first row looks like data", () => {
    expect(detectDialect("1;2\n3;4\n").header).toBe(false);
  });

  it("CSV-EDGE-09 look-alike numbers stay text", () => {
    for (const v of ["00123", "1E5", "2026-114", "12345678901234567890", "+33 6 12 34 56 78"])
      expect(parseNumber(v, ",")).toBeUndefined();
    expect(columnType(["00123", "00456", "00789"], ",")).toBe("text");
  });

  it("CSV-EDGE-10 French and English numbers are both read", () => {
    expect(parseNumber("1 234,56", ",")).toBe(1234.56);
    expect(parseNumber("1 234,5", ",")).toBe(1234.5);
    expect(parseNumber("1,234.56", ".")).toBe(1234.56);
    expect(parseNumber("-3,2", ",")).toBe(-3.2);
    expect(parseNumber("abc", ",")).toBeUndefined();
  });
});

describe("clipboard", () => {
  const m = new CsvModel('a;b\n"x;y";2\n3;"q\nr"\n', {
    delimiter: ";",
    header: true,
    decimal: ",",
  });

  it("CSV-08 copies TSV that spreadsheets read, with quoting only where needed", () => {
    const rows = cellsOf(m, { r0: 1, c0: 0, r1: 2, c1: 1 });
    expect(toTsv(rows)).toBe('x;y\t2\n3\t"q\nr"');
    expect(toTsv([["1,5", "a\tb"]])).toBe('1,5\t"a\tb"');
  });

  it("copies an HTML table with escaped cells", () => {
    expect(toHtml([["<b>", "a&b"]])).toBe(
      "<table><tbody><tr><td>&lt;b&gt;</td><td>a&amp;b</td></tr></tbody></table>",
    );
  });

  it("CSV-09 reads an HTML table's cell text and drops markup, scripts and styles", () => {
    const html =
      '<table><tr><th>Nom</th><th style="color:red">Prix</th></tr><tr><td><b>Pomme</b><script>alert(1)</script></td><td onclick="x()">1,5<br>€</td></tr></table>';
    const t = htmlTable(html);
    expect(t).toEqual([
      ["Nom", "Prix"],
      ["Pomme", "1,5\n€"],
    ]);
    expect(JSON.stringify(t)).not.toContain("<");
  });

  it("CSV-EDGE-21 pasted text with delimiters and newlines inside a quoted cell lands in one cell", () => {
    expect(fromClipboard({ text: 'a\t"b\tc"\t"d\ne"\n1\t2\t3\n' })).toEqual([
      ["a", "b\tc", "d\ne"],
      ["1", "2", "3"],
    ]);
    expect(fromClipboard({ text: "a;b\n1;2" })).toEqual([
      ["a", "b"],
      ["1", "2"],
    ]);
  });

  it("CSV-22 formulas are recognised and neutralised, plain negative numbers are not", () => {
    expect(looksLikeFormula("=SUM(A1:A3)")).toBe(true);
    expect(looksLikeFormula("+cmd|' /C calc'!A0")).toBe(true);
    expect(looksLikeFormula("@SUM(1)")).toBe(true);
    expect(looksLikeFormula("-5")).toBe(false);
    expect(looksLikeFormula("-5,25")).toBe(false);
    expect(looksLikeFormula("-x")).toBe(true);
    expect(neutralise([["=1+1", "ok", "-2"]])).toEqual({ rows: [["'=1+1", "ok", "-2"]], count: 1 });
  });

  it("CSV-EDGE-11 a formula typed in a cell is stored as text, never evaluated", () => {
    const f = new CsvModel("a\n1\n", { delimiter: ";", header: true, decimal: "," });
    expect(f.value(1, 0)).toBe("1");
    const text = f.text.replace("1", "=SUM(A1:A3)");
    expect(new CsvModel(text, f.dialect).value(1, 0)).toBe("=SUM(A1:A3)");
  });
});

describe("addresses, stats, display", () => {
  it("name box references", () => {
    expect(colName(0)).toBe("A");
    expect(colName(27)).toBe("AB");
    expect(rangeRef(11, 1, 19, 3)).toBe("B12:D20");
    expect(parseRef("b12")).toEqual({ r0: 11, c0: 1, r1: 11, c1: 1 });
    expect(parseRef("D20:B12")).toEqual({ r0: 11, c0: 1, r1: 19, c1: 3 });
    expect(parseRef("zzz")).toBeUndefined();
  });

  it("selection stats read French numbers", () => {
    const m = new CsvModel("n;v\na;1 234,5\nb;2\nc;x\nd;\n", {
      delimiter: ";",
      header: true,
      decimal: ",",
    });
    const s = selectionStats(m, [{ r0: 1, c0: 1, r1: 4, c1: 1 }]);
    expect(s).toMatchObject({ count: 3, numeric: 2, sum: 1236.5, min: 2, max: 1234.5 });
    expect(s.mean).toBeCloseTo(618.25);
  });

  it("SHEET numbers display in French while the stored text is unchanged", () => {
    expect(displayNumber("1234567,89", ",")).toBe("1 234 567,89");
    expect(displayNumber("1234.5", ".")).toBe("1 234,5");
    expect(displayNumber("00123", ",")).toBe("00123");
    expect(formatFr(0.1 + 0.2)).toBe("0,3");
  });
});

describe("sort and filter views", () => {
  const m = new CsvModel("n;v\na;1 234,5\nb;2\nc;10\nd;\ne;x\n", {
    delimiter: ";",
    header: true,
    decimal: ",",
  });

  it("CSV-11 sorts French numbers numerically, numbers before text, empty last, file untouched", () => {
    const before = m.text;
    expect(Array.from(buildView(m, { sort: { col: 1, dir: 1 }, filters: new Map() }))).toEqual([
      2, 3, 1, 5, 4,
    ]);
    expect(Array.from(buildView(m, { sort: { col: 1, dir: -1 }, filters: new Map() }))).toEqual([
      5, 1, 3, 2, 4,
    ]);
    expect(m.text).toBe(before);
  });

  it("CSV-12 a filtered view maps displayed rows to the right file rows", () => {
    const f = new Map<number, ColumnFilter>([[1, { cond: { op: ">", a: 5 } }]]);
    const rows = buildView(m, { filters: f });
    expect(Array.from(rows)).toEqual([1, 3]);
    expect(applySplices(m.text, m.editCell(rows[1] as number, 1, "11"))).toBe(
      "n;v\na;1 234,5\nb;2\nc;11\nd;\ne;x\n",
    );
    const some = new Map<number, ColumnFilter>([[0, { values: new Set(["b", "e"]) }]]);
    expect(Array.from(buildView(m, { filters: some }))).toEqual([2, 5]);
  });
});

describe("fill handle series", () => {
  it("continues numbers, dates, names and numbered text; copies a single value", () => {
    expect(continueSeries(["1", "2"], 3)).toEqual(["3", "4", "5"]);
    expect(continueSeries(["5"], 2)).toEqual(["5", "5"]);
    expect(continueSeries(["1,5", "2,0"], 2)).toEqual(["2,5", "3,0"]);
    expect(continueSeries(["2026-01-30", "2026-01-31"], 2)).toEqual(["2026-02-01", "2026-02-02"]);
    expect(continueSeries(["28/02/2026"], 2)).toEqual(["01/03/2026", "02/03/2026"]);
    expect(continueSeries(["Lundi"], 3)).toEqual(["Mardi", "Mercredi", "Jeudi"]);
    expect(continueSeries(["dimanche"], 1)).toEqual(["lundi"]);
    expect(continueSeries(["Article 1"], 2)).toEqual(["Article 2", "Article 3"]);
    expect(continueSeries(["x", "y"], 3)).toEqual(["x", "y", "x"]);
  });

  it("goes backwards when dragging up or left", () => {
    expect(continueSeries(["3", "4"], 2, -1)).toEqual(["2", "1"]);
    expect(continueSeries(["Mardi"], 2, -1)).toEqual(["Lundi", "Dimanche"]);
  });

  it("Ctrl+; inserts today in French form", () => {
    expect(today(new Date(2026, 9, 25))).toBe("25/10/2026");
  });
});

describe("typst code", () => {
  const d = { delimiter: ";", header: true, decimal: "," } as const;

  it("CSV-16 a table that reads the file", () => {
    expect(csvTableCode("data/x.csv", d, 3)).toBe(
      '#table(columns: 3, ..csv("data/x.csv", delimiter: ";").flatten())',
    );
    expect(csvTableCode("x.csv", { ...d, delimiter: "," }, 2)).toBe(
      '#table(columns: 2, ..csv("x.csv").flatten())',
    );
  });

  it("CSV-17 a chart of two columns, with the French decimal handled", () => {
    const code = csvChartCode('a "b".csv', d, 0, 2);
    expect(code).toContain('#import "@preview/lilaq:0.5.0" as lq');
    expect(code).toContain('csv("a \\"b\\".csv", delimiter: ";").slice(1)');
    expect(code).toContain('float(r.at(2).replace(",", "."))');
  });
});
