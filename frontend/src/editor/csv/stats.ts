import type { Range } from "./clipboard";
import { parseNumber } from "./dialect";
import type { CsvModel } from "./model";

export interface SelectionStats {
  /** Non-empty cells. */
  count: number;
  /** Cells read as numbers. */
  numeric: number;
  sum: number;
  mean: number;
  min: number;
  max: number;
}

/** Count, sum, mean, min and max of the cells of one or several ranges. */
export function selectionStats(model: CsvModel, ranges: readonly Range[]): SelectionStats {
  let count = 0;
  let numeric = 0;
  let sum = 0;
  let min = Number.POSITIVE_INFINITY;
  let max = Number.NEGATIVE_INFINITY;
  const decimal = model.dialect.decimal;
  for (const range of ranges) {
    for (let r = range.r0; r <= Math.min(range.r1, model.records - 1); r++) {
      const cells = model.recordUpTo(r, range.c1);
      for (let c = range.c0; c <= range.c1; c++) {
        const v = cells[c]?.value ?? "";
        if (v === "") continue;
        count++;
        const n = parseNumber(v, decimal);
        if (n === undefined) continue;
        numeric++;
        sum += n;
        if (n < min) min = n;
        if (n > max) max = n;
      }
    }
  }
  return {
    count,
    numeric,
    sum,
    mean: numeric ? sum / numeric : 0,
    min: numeric ? min : 0,
    max: numeric ? max : 0,
  };
}

const NNBSP = " ";

/** A number as French readers write it: narrow no-break space thousands, comma decimal. */
export function formatFr(n: number): string {
  if (!Number.isFinite(n)) return "";
  const s = Number(n.toPrecision(12)).toString();
  return groupFr(s.replace(".", ","));
}

/** Groups the integer part of a number written with a comma or point decimal. */
function groupFr(s: string): string {
  const m = /^([-+]?)(\d+)([.,]\d+)?$/.exec(s);
  if (!m) return s;
  const int = (m[2] as string).replace(/\B(?=(\d{3})+(?!\d))/g, NNBSP);
  return `${m[1]}${int}${m[3] ? `,${(m[3] as string).slice(1)}` : ""}`;
}

/**
 * How a numeric cell is shown: French grouping and decimal comma. Only the display changes;
 * the cell's text, and what is edited and saved, stay as typed.
 */
export function displayNumber(raw: string, decimal: "," | "."): string {
  const n = parseNumber(raw, decimal);
  if (n === undefined) return raw;
  const t = raw.trim().replace(/[\s  ]/g, "");
  const plain = decimal === "," ? t.replaceAll(".", "").replace(",", ".") : t.replaceAll(",", "");
  return groupFr(plain);
}
