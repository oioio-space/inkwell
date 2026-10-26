import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { detectDialect, parseNumber } from "./dialect";
import { applySplices, CsvModel } from "./model";
import { rawCell } from "./records";
import data from "./testdata/data.psv?raw";
import excelSep from "./testdata/excel-sep.csv?raw";
import headerDupes from "./testdata/header-dupes.csv?raw";
import malformed from "./testdata/malformed.csv?raw";
import overquoted from "./testdata/overquoted.csv?raw";
import quoted from "./testdata/quoted.csv?raw";
import quotesTsv from "./testdata/quotes.tsv?raw";
import ragged from "./testdata/ragged.csv?raw";
import traps from "./testdata/traps.csv?raw";
import unicode from "./testdata/unicode.csv?raw";

const open = (text: string, hint?: Parameters<typeof detectDialect>[1]) =>
  new CsvModel(text, detectDialect(text, hint));
const rows = (m: CsvModel) =>
  Array.from({ length: m.records }, (_, r) => m.record(r).map((c) => c.value));

describe("CSV-EDGE reading and dialect", () => {
  it("EDGE-02 Excel's `sep=;` first line is honoured, kept, and is not a row", () => {
    const m = open(excelSep);
    expect(m.dialect.delimiter).toBe(";");
    expect(m.preamble).toBe(1);
    expect(m.dataStart).toBe(2);
    expect(m.value(m.preamble, 0)).toBe("Nom");
    // An edit leaves the line alone; a new column goes into every row but not that line.
    expect(applySplices(excelSep, m.editCell(2, 1, "9"))).toBe(
      "sep=;\nNom;Prix\nPomme;9\nPoire;2\n",
    );
    expect(applySplices(excelSep, m.insertCols(1)).startsWith("sep=;\n")).toBe(true);
    expect(applySplices(excelSep, m.insertCols(1))).toBe(
      "sep=;\nNom;;Prix\nPomme;;1,5\nPoire;;2\n",
    );
  });

  it("EDGE-03 RFC 4180: separator, line breaks (LF and CRLF) and doubled quotes inside quoted cells", () => {
    const m = open(quoted);
    expect(rows(m)).toEqual([
      ["id", "texte", "note"],
      ["1", "a;b", "ligne un\nligne deux"],
      ["2", 'il a dit "bonjour"', "plain"],
      ["3", "", " espaces"],
    ]);
    // The same file with CRLF between the records and inside a quoted cell.
    const crlf = quoted.replaceAll("\n", "\r\n");
    const c = open(crlf);
    expect(c.records).toBe(4);
    expect(c.value(1, 2)).toBe("ligne un\r\nligne deux");
    expect(c.value(3, 2)).toBe(" espaces");
    expect(c.text).toBe(crlf); // not a byte lost
    expect(c.eol).toBe("\r\n");
  });

  it("EDGE-04 malformed input is read leniently, flagged, and never loses a byte", () => {
    const m = open(malformed);
    expect(m.text).toBe(malformed);
    expect(m.records).toBe(2); // the open quote swallows the rest
    expect(m.value(1, 1)).toContain("unclosed quote near the end");
    expect(m.value(1, 1)).toContain("5;6");
    // A stray quote in the middle of an unquoted cell is plain text.
    expect(open('a;b\n5" pipe;2\n').value(1, 0)).toBe('5" pipe');
    // A backslash-escaped quote is not an escape in CSV.
    expect(open('a;b\nsay \\"hi\\";2\n').value(1, 0)).toBe('say \\"hi\\"');
  });

  it("EDGE-05 ragged rows, trailing delimiter, empty lines, no final newline, spaces kept", () => {
    const m = open(ragged);
    expect(rows(m)).toEqual([
      ["a", "b", "c"],
      ["1", "2"],
      ["3", "4", "5", "6"],
      [""],
      ["7", "8", "9", ""],
      ["  spaced  ", " x ", "y"],
    ]);
    expect(m.endsWithNewline).toBe(false);
    expect(m.index.uneven).toBeGreaterThan(0);
    expect(m.text).toBe(ragged);
    // Editing the last cell keeps the missing final newline missing.
    expect(applySplices(ragged, m.editCell(5, 2, "Y"))).toBe(ragged.replace("y", "Y"));
  });

  it("EDGE-06 duplicate and empty header names; no header at all; the header can be switched", () => {
    const m = open(headerDupes);
    expect(m.dialect.header).toBe(false); // an empty name: not read as a header on its own
    m.setDialect({ ...m.dialect, header: true });
    expect(m.dataStart).toBe(1);
    expect(m.value(0, 0)).toBe(m.value(0, 1)); // duplicate names are kept
    expect(m.value(0, 2)).toBe(""); // shown as "Colonne C" by the grid
    m.setDialect({ ...m.dialect, header: false });
    expect(m.dataStart).toBe(0);
  });

  it("EDGE-07 NFC and NFD, right-to-left text and emoji are read and kept exactly", () => {
    const m = open(unicode);
    expect(rows(m)[1]).toEqual(["café", "e"]);
    expect(rows(m)[2]).toEqual(["مرحبا", "שלום"]);
    expect(rows(m)[3]?.[0]).toBe("😀 fête");
    expect(rows(m)[3]?.[1]).toBe("👨‍👩‍👧");
    const nfd = "é".normalize("NFD");
    const edited = applySplices(unicode, m.editCell(1, 1, nfd));
    expect(new CsvModel(edited, m.dialect).value(1, 1)).toBe(nfd); // no silent NFC
    expect(nfd).not.toBe(nfd.normalize("NFC"));
  });

  it("EDGE-08 a 1 MB cell and a 5 000 column row", () => {
    const big = "x".repeat(1_000_000);
    const m = new CsvModel(`a;b\n"${big}";2\n`, { delimiter: ";", header: true, decimal: "," });
    expect(m.value(1, 0)).toHaveLength(1_000_000);
    expect(m.value(1, 1)).toBe("2");
    const wide = Array.from({ length: 5000 }, (_, i) => `c${i}`).join(",");
    const w = new CsvModel(`${wide}\n${wide.replaceAll("c", "v")}\n`, {
      delimiter: ",",
      header: true,
      decimal: ".",
    });
    expect(w.width).toBe(5000);
    expect(w.value(1, 4999)).toBe("v4999");
    expect(w.record(1)).toHaveLength(5000);
  });
});

describe("CSV-EDGE values: nothing is mangled", () => {
  it("EDGE-09 leading zeros, long ids, 1E5 and 2026-114 stay exactly as typed, through an edit elsewhere", () => {
    const m = open(traps, ";");
    expect(rows(m)[1]).toEqual([
      "00123",
      "01000",
      "FR7630006000011234567890189",
      "2026-114",
      "1E5",
      "=SUM(A1:A3)",
    ]);
    for (const v of ["00123", "01000", "2026-114", "1E5", "12345678901234567890"])
      expect(parseNumber(v, ",")).toBeUndefined();
    const edited = applySplices(traps, m.editCell(2, 0, "00999"));
    expect(edited).toBe(traps.replace("00456", "00999"));
  });

  it("EDGE-10 French and English numbers, and a date that could be read two ways", () => {
    expect(parseNumber("1 234,56", ",")).toBe(1234.56);
    expect(parseNumber("1,234.56", ".")).toBe(1234.56);
    expect(parseNumber("1.234,56", ",")).toBe(1234.56);
    expect(parseNumber("03/04/2026", ",")).toBeUndefined();
  });

  it("EDGE-11 a cell starting with = + - @ tab or CR is stored as text", () => {
    const m = open("a\n1\n");
    for (const v of ["=1+1", "+1+1", "-1+1", "@SUM(1)", "\tx", "\rx"]) {
      const out = new CsvModel(applySplices(m.text, m.editCell(1, 0, v)), m.dialect);
      expect(out.value(1, 0)).toBe(v.replace("\r", "\n"));
    }
  });
});

describe("CSV-EDGE writing", () => {
  it("EDGE-12 quote minimally, keep the style of what was not edited, over-quoted rows included", () => {
    const m = open(overquoted);
    // An unchanged file is unchanged.
    expect(m.text).toBe(overquoted);
    // Editing a quoted cell keeps it quoted; the neighbours keep their quotes or lack of them.
    expect(applySplices(overquoted, m.editCell(1, 1, "22"))).toBe(
      '"a";"b";"c"\n"1";"22";"3"\n"4";x;"6"\n',
    );
    // An unquoted cell stays unquoted unless it needs quotes.
    expect(applySplices(overquoted, m.editCell(2, 1, "y"))).toBe(
      '"a";"b";"c"\n"1";"2";"3"\n"4";y;"6"\n',
    );
    expect(applySplices(overquoted, m.editCell(2, 1, "y;z"))).toBe(
      '"a";"b";"c"\n"1";"2";"3"\n"4";"y;z";"6"\n',
    );
  });

  it("EDGE-13 the end of line of the file is kept for what is added", () => {
    const crlf = "a;b\r\n1;2\r\n";
    const m = open(crlf);
    expect(applySplices(crlf, m.insertRows(2))).toBe("a;b\r\n1;2\r\n;\r\n");
    expect(applySplices(crlf, m.editCell(1, 1, "9"))).toBe("a;b\r\n1;9\r\n");
  });

  it("EDGE-15 TSV keeps literal quotes (no quoting), PSV and .tab read as tables", () => {
    const tsv = new CsvModel(quotesTsv, {
      delimiter: "\t",
      header: true,
      decimal: ",",
      quote: false,
    });
    expect(rows(tsv)).toEqual([
      ["nom", "note"],
      ['5" pipe', '"hello" he said'],
      ["x", "y"],
    ]);
    expect(applySplices(quotesTsv, tsv.editCell(2, 1, 'say "z"'))).toBe(
      quotesTsv.replace("\ty\n", '\tsay "z"\n'),
    );
    const psv = open(data);
    expect(psv.dialect.delimiter).toBe("|");
    expect(rows(psv)).toEqual([
      ["a", "b", "c"],
      ["1", "2", "3"],
    ]);
    const tab = open("a\tb\n1\t2\n");
    expect(tab.dialect.delimiter).toBe("\t");
  });
});

describe("CSV-EDGE properties", () => {
  const unit = fc.constantFrom(
    "a",
    "é",
    "😀",
    "א",
    ",",
    ";",
    "\t",
    "|",
    '"',
    "\n",
    "\r",
    " ",
    "=",
    "'",
    "0",
  );
  const cell = fc.string({ unit, maxLength: 8 });

  it("a table of random strings, serialised then read, gives the same cells for every dialect", () => {
    fc.assert(
      fc.property(
        fc.array(fc.array(cell, { minLength: 2, maxLength: 4 }), { minLength: 1, maxLength: 6 }),
        fc.constantFrom(",", ";", "\t", "|"),
        (table, d) => {
          const w = Math.max(...table.map((r) => r.length));
          const full = table.map((r) => [...r, ...Array(w - r.length).fill("k")]);
          const norm = (v: string) => v.replaceAll("\r\n", "\n").replaceAll("\r", "\n");
          const text = `${full.map((r) => r.map((v) => rawCell(v, d)).join(d)).join("\n")}\n`;
          const m = new CsvModel(text, { delimiter: d as ",", header: false, decimal: "." });
          expect(rows(m)).toEqual(full.map((r) => r.map(norm)));
        },
      ),
      { numRuns: 400 },
    );
  });

  it("editing one cell changes only that cell's bytes, for any value and any dialect", () => {
    fc.assert(
      fc.property(
        fc.array(fc.array(cell, { minLength: 3, maxLength: 3 }), { minLength: 2, maxLength: 5 }),
        fc.constantFrom(",", ";", "\t", "|"),
        fc.nat(4),
        fc.nat(2),
        cell,
        (table, d, r0, c0, v) => {
          const text = `${table.map((r) => r.map((x) => rawCell(x, d)).join(d)).join("\n")}\n`;
          const m = new CsvModel(text, { delimiter: d as ",", header: false, decimal: "." });
          const r = r0 % table.length;
          const target = m.record(r)[c0] as { start: number; end: number };
          const next = applySplices(text, m.editCell(r, c0, v));
          expect(next.slice(0, target.start)).toBe(text.slice(0, target.start));
          expect(next.slice(next.length - (text.length - target.end))).toBe(text.slice(target.end));
        },
      ),
      { numRuns: 400 },
    );
  });
});
