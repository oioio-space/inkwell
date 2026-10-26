import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { applySplices, CsvModel, windowOf } from "./model";
import { rawCell } from "./records";
import { RowIndex } from "./rowIndex";

const D = { delimiter: ";", header: true, decimal: "," } as const;

/** Applies one user action to a model the way the sheet does: splices, then incremental index. */
function act(m: CsvModel, splices: ReturnType<CsvModel["editCell"]>): string {
  const next = applySplices(m.text, splices);
  m.update(next, windowOf(splices, next.length, m.text.length));
  return next;
}

/** Every cell of the text. */
function table(text: string, delim: string): string[][] {
  const m = new CsvModel(text, { delimiter: delim as ";", header: false, decimal: "." });
  return Array.from({ length: m.records }, (_, i) => m.record(i).map((c) => c.value));
}

describe("records and index", () => {
  it("CSV-02 cell with delimiter, quote and newline reads and is rewritten with RFC 4180 quoting", () => {
    const m = new CsvModel('a;b\n"x;y";"say ""hi""\nbye"\n', D);
    expect(m.value(1, 0)).toBe("x;y");
    expect(m.value(1, 1)).toBe('say "hi"\nbye');
    expect(m.records).toBe(2);
    act(m, m.editCell(1, 0, "p;q"));
    expect(m.text).toBe('a;b\n"p;q";"say ""hi""\nbye"\n');
    act(m, m.editCell(1, 1, 'z"'));
    expect(m.value(1, 1)).toBe('z"');
    expect(m.text).toBe('a;b\n"p;q";"z"""\n');
  });

  it("CSV-13 uneven rows, empty final line and CRLF are read without loss and counted", () => {
    const text = "a;b;c\r\n1;2\r\n3;4;5;6\r\n\r\n";
    const m = new CsvModel(text, D);
    expect(m.records).toBe(4);
    expect(m.width).toBe(4);
    expect(m.index.uneven).toBe(3);
    expect(m.value(2, 3)).toBe("6");
    expect(m.value(1, 1)).toBe("2");
    expect(m.text).toBe(text);
  });

  it("index update equals a fresh scan after any text change (property)", () => {
    const piece = fc.constantFrom("a", "b", ",", '"', "\n", "\r\n", ",,", '""', " ");
    const doc = fc.array(piece, { maxLength: 40 }).map((p) => p.join(""));
    fc.assert(
      fc.property(doc, fc.nat(60), fc.nat(10), doc, (text, at, len, ins) => {
        const from = Math.min(at, text.length);
        const to = Math.min(text.length, from + len);
        const next = text.slice(0, from) + ins + text.slice(to);
        const idx = RowIndex.scan(text, ",");
        idx.update(next, from, to, ins.length);
        const fresh = RowIndex.scan(next, ",");
        expect(Array.from(idx.starts.subarray(0, idx.count))).toEqual(
          Array.from(fresh.starts.subarray(0, fresh.count)),
        );
        expect(Array.from(idx.cols.subarray(0, idx.count))).toEqual(
          Array.from(fresh.cols.subarray(0, fresh.count)),
        );
        expect(idx.length).toBe(next.length);
      }),
      { numRuns: 400 },
    );
  });

  it("serialising then parsing gives the same cells, for every delimiter (property)", () => {
    const cell = fc.string({
      unit: fc.constantFrom("a", "é", "😀", ",", ";", "\t", "|", '"', "\n", " ", "x"),
    });
    const grid = fc.array(fc.array(cell, { minLength: 2, maxLength: 4 }), {
      minLength: 1,
      maxLength: 5,
    });
    fc.assert(
      fc.property(grid, fc.constantFrom(",", ";", "\t", "|"), (rows, d) => {
        const w = Math.max(...rows.map((r) => r.length));
        const full = rows.map((r) => [...r, ...Array(w - r.length).fill("x")]);
        const text = `${full.map((r) => r.map((v) => rawCell(v, d)).join(d)).join("\n")}\n`;
        expect(table(text, d)).toEqual(full);
      }),
      { numRuns: 300 },
    );
  });

  it("editing one cell changes only that cell's bytes (property)", () => {
    const cell = fc.string({ unit: fc.constantFrom("a", "b", ",", '"', " ", "1") });
    fc.assert(
      fc.property(
        fc.array(fc.array(cell, { minLength: 3, maxLength: 3 }), { minLength: 2, maxLength: 5 }),
        fc.nat(4),
        fc.nat(2),
        cell,
        (rows, r0, c0, v) => {
          const text = `${rows.map((r) => r.map((x) => rawCell(x, ",")).join(",")).join("\n")}\n`;
          const m = new CsvModel(text, { delimiter: ",", header: false, decimal: "." });
          const r = r0 % rows.length;
          const cell0 = m.record(r)[c0] as { start: number; end: number };
          const next = applySplices(text, m.editCell(r, c0, v));
          expect(next.slice(0, cell0.start)).toBe(text.slice(0, cell0.start));
          expect(next.slice(next.length - (text.length - cell0.end))).toBe(text.slice(cell0.end));
          const after = new CsvModel(next, { delimiter: ",", header: false, decimal: "." });
          expect(after.value(r, c0)).toBe(v);
        },
      ),
      { numRuns: 300 },
    );
  });
});

describe("edits as text splices", () => {
  const base = "n;v\na;1\nb;2\nc;3\n";

  it("CSV-01 an unchanged file is the same text, and an edit touches only its cell", () => {
    const m = new CsvModel(base, D);
    expect(m.text).toBe(base);
    expect(applySplices(base, m.editCell(2, 1, "20"))).toBe("n;v\na;1\nb;20\nc;3\n");
  });

  it("insert and delete rows and columns", () => {
    const m = new CsvModel(base, D);
    expect(applySplices(base, m.insertRows(2))).toBe("n;v\na;1\n;\nb;2\nc;3\n");
    expect(applySplices(base, m.insertRows(4, 2))).toBe("n;v\na;1\nb;2\nc;3\n;\n;\n");
    expect(applySplices(base, m.deleteRows(1, 2))).toBe("n;v\nc;3\n");
    expect(applySplices(base, m.insertCols(1))).toBe("n;;v\na;;1\nb;;2\nc;;3\n");
    expect(applySplices(base, m.insertCols(2))).toBe("n;v;\na;1;\nb;2;\nc;3;\n");
    expect(applySplices(base, m.deleteCols(0))).toBe("v\n1\n2\n3\n");
    expect(applySplices(base, m.deleteCols(1))).toBe("n\na\nb\nc\n");
    expect(applySplices(base, m.duplicateRow(1))).toBe("n;v\na;1\na;1\nb;2\nc;3\n");
  });

  it("move rows and columns", () => {
    const m = new CsvModel(base, D);
    expect(applySplices(base, m.moveRows(1, 1, 3))).toBe("n;v\nb;2\na;1\nc;3\n");
    expect(applySplices(base, m.moveRows(3, 1, 1))).toBe("n;v\nc;3\na;1\nb;2\n");
    expect(applySplices(base, m.moveRows(1, 1, 4))).toBe("n;v\nb;2\nc;3\na;1\n");
    expect(applySplices(base, m.moveCols(0, 1, 2))).toBe("v;n\n1;a\n2;b\n3;c\n");
  });

  it("pasting past the end extends the table", () => {
    const m = new CsvModel(base, D);
    const out = applySplices(
      base,
      m.editCells([
        { r: 3, c: 1, value: "x" },
        { r: 4, c: 0, value: "d" },
        { r: 4, c: 2, value: "e;f" },
      ]),
    );
    expect(out).toBe('n;v\na;1\nb;2\nc;x\nd;;"e;f"\n');
  });

  it("CSV-11 sort reorders data rows and keeps the header; rows already in order are not touched", () => {
    const m = new CsvModel(base, D);
    expect(applySplices(base, m.applySort([3, 2, 1]))).toBe("n;v\nc;3\nb;2\na;1\n");
    // Only the row that moves is cut and written again.
    const one = m.applySort([2, 3, 1]);
    expect(one).toHaveLength(2);
    expect(applySplices(base, one)).toBe("n;v\nb;2\nc;3\na;1\n");
  });

  it("any reordering of the rows gives the rows in that order, with or without a final newline (property)", () => {
    fc.assert(
      fc.property(
        fc.shuffledSubarray([1, 2, 3, 4, 5, 6], { minLength: 6, maxLength: 6 }),
        fc.boolean(),
        (perm, finalNewline) => {
          const rows = ["h", ...[1, 2, 3, 4, 5, 6].map((i) => `r${i};${i}`)];
          const text = rows.join("\n") + (finalNewline ? "\n" : "");
          const m = new CsvModel(text, D);
          const out = applySplices(text, m.applySort(perm));
          const want = ["h", ...perm.map((i) => `r${i};${i}`)];
          expect(out.replace(/\n$/, "").split("\n")).toEqual(want);
        },
      ),
      { numRuns: 200 },
    );
  });

  it("a final line without newline stays without it", () => {
    const m = new CsvModel("a;b\n1;2", D);
    expect(applySplices(m.text, m.editCell(1, 1, "9"))).toBe("a;b\n1;9");
  });

  it("sep= line is honoured and kept", () => {
    const m = new CsvModel("sep=;\na;b\n1;2\n");
    expect(m.dialect.delimiter).toBe(";");
    expect(m.preamble).toBe(1);
    expect(m.dataStart).toBe(2);
    expect(m.text.startsWith("sep=;\n")).toBe(true);
  });
});
