import { describe, expect, it } from "vitest";
import { papaIndex } from "./papaIndex";
import { scanAll } from "./records";

function big(rows: number, cols: number): string {
  const line = Array.from({ length: cols }, (_, c) => `valeur ${c} de la ligne`).join(";");
  let out = `${Array.from({ length: cols }, (_, c) => `col${c}`).join(";")}\n`;
  for (let r = 0; r < rows; r++) out += `${line} ${r}\n`;
  return out;
}

describe("Papa Parse index", () => {
  it("gives the same record offsets as the incremental scanner, quotes and blank lines included", () => {
    const text = 'a;b\n"x;y";"two\nlines"\n\n1;2;3\n"q""uote";z\nlast;row';
    const p = papaIndex(text, ";");
    const mine = scanAll(text, ";");
    expect(Array.from(p.starts)).toEqual(mine.starts);
    expect(Array.from(p.cols)).toEqual(mine.cols);
  });

  it("a text ending with a newline has no phantom record", () => {
    const p = papaIndex("a;b\n1;2\n", ";");
    expect(Array.from(p.starts)).toEqual([0, 4]);
  });

  it("CSV-10 perf: index of a large file, logged (asserted only with INKWELL_PERF=1)", () => {
    const rows = Number(process.env.INKWELL_PERF === "1" ? 500_000 : 50_000);
    const text = big(rows, 10);
    const t0 = performance.now();
    const p = papaIndex(text, ";");
    const ms = performance.now() - t0;
    const t1 = performance.now();
    const mine = scanAll(text, ";");
    const ms2 = performance.now() - t1;
    console.info(
      `CSV-10 ${(text.length / 1e6).toFixed(1)} MB, ${p.starts.length} records: Papa Parse ${ms.toFixed(0)} ms, own scan ${ms2.toFixed(0)} ms`,
    );
    expect(p.starts.length).toBe(mine.starts.length);
    if (process.env.INKWELL_PERF === "1") expect(ms).toBeLessThan(3000);
  });
});
