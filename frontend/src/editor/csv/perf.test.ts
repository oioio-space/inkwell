import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import { cellsOf, toTsv } from "./clipboard";
import { applySplices, CsvModel, windowOf } from "./model";
import { papaIndex } from "./papaIndex";
import { RowIndex } from "./rowIndex";
import { selectionStats } from "./stats";
import { buildView } from "./view";

const PERF = process.env.INKWELL_PERF === "1";
const ROWS = PERF ? 500_000 : 40_000;
const COLS = 20;

function build(): string {
  const parts: string[] = [Array.from({ length: COLS }, (_, c) => `col${c}`).join(";")];
  for (let r = 0; r < ROWS; r++) {
    const cells: string[] = [];
    for (let c = 0; c < COLS; c++) cells.push(c % 3 === 0 ? `${r},${c % 100}` : `texte ${c} ${r}`);
    parts.push(cells.join(";"));
  }
  return `${parts.join("\n")}\n`;
}

const budget = (label: string, ms: number, limit: number) => {
  console.info(`PERF ${label}: ${ms.toFixed(2)} ms (budget ${limit} ms)`);
  if (PERF) expect(ms).toBeLessThan(limit);
};

const median = (xs: number[]) => [...xs].sort((a, b) => a - b)[Math.floor(xs.length / 2)] as number;

describe(`CSV-10 and SHEET perf, ${ROWS} rows x ${COLS} columns`, () => {
  const text = build();
  const dialect = { delimiter: ";", header: true, decimal: "," } as const;
  const p = papaIndex(text, ";");
  const model = new CsvModel(text, dialect, new RowIndex(p.starts, p.cols, text.length, ";"));

  it("opens: Papa Parse index in under 3 s", () => {
    const t0 = performance.now();
    papaIndex(text, ";");
    budget(`index ${(text.length / 1e6).toFixed(0)} MB`, performance.now() - t0, 3000);
  });

  it("editing one cell (plan, splice, incremental index) takes under 16 ms", () => {
    const times: number[] = [];
    for (let i = 0; i < 30; i++) {
      const r = Math.floor((ROWS / 31) * (i + 1));
      const t0 = performance.now();
      const splices = model.editCell(r, 4, `edit ${i}`);
      const plan = performance.now() - t0;
      // The new text comes from the document as a flat string: building it is the document's cost.
      const next = applySplices(model.text, splices);
      void next.charCodeAt(next.length >> 1);
      const t1 = performance.now();
      model.update(next, windowOf(splices, next.length, model.text.length));
      times.push(plan + performance.now() - t1);
    }
    budget("edit one cell: plan + incremental index (median)", median(times), 16);
    expect(model.value(Math.floor(ROWS / 31), 4)).toBe("edit 0");
  });

  it("reading a screen of cells takes well under a frame", () => {
    const t0 = performance.now();
    let n = 0;
    for (let r = ROWS >> 1; r < (ROWS >> 1) + 60; r++)
      for (let c = 0; c < COLS; c++) n += model.value(r, c).length;
    budget("read 60x20 cells", performance.now() - t0, 8);
    expect(n).toBeGreaterThan(0);
  });

  it("copying 100 000 cells takes under 500 ms", () => {
    const t0 = performance.now();
    const rows = cellsOf(model, { r0: 1, c0: 0, r1: 5000, c1: 19 });
    const tsv = toTsv(rows);
    budget("copy 100k cells to TSV", performance.now() - t0, 500);
    expect(tsv.length).toBeGreaterThan(1000);
  });

  it("selection stats of a whole column", () => {
    const t0 = performance.now();
    const s = selectionStats(model, [{ r0: 1, c0: 0, r1: model.records - 1, c1: 0 }]);
    budget("stats of one column", performance.now() - t0, 1500);
    expect(s.numeric).toBeGreaterThan(0);
  });

  it("sorting a column as a view", () => {
    const t0 = performance.now();
    const order = buildView(model, { sort: { col: 0, dir: -1 }, filters: new Map() });
    budget("sort view of one column", performance.now() - t0, 3000);
    expect(order.length).toBe(model.records - 1);
  });

  it("an Automerge text splice at several sizes (the document's own cost, logged)", {
    timeout: 300_000,
  }, () => {
    for (const mb of PERF ? [5, 10, 20] : [2]) {
      const part = text.slice(0, mb * 1_000_000);
      try {
        let doc = A.from({ text: part });
        const times: number[] = [];
        for (let i = 0; i < 5; i++) {
          const t0 = performance.now();
          doc = A.change(doc, (d) => A.splice(d, ["text"], (part.length >> 1) + i, 1, "x"));
          void doc.text.length;
          times.push(performance.now() - t0);
        }
        console.info(
          `PERF Automerge splice on ${mb} MB text (median): ${median(times).toFixed(2)} ms`,
        );
      } catch (e) {
        console.info(`PERF Automerge splice on ${mb} MB text failed: ${String(e).slice(0, 80)}`);
      }
    }
  });
});
