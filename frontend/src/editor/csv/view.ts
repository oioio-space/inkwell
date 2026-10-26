import { columnType, parseNumber } from "./dialect";
import type { CsvModel } from "./model";

/** A condition on a column's cells. Numbers use the file's decimal separator. */
export interface ColumnFilter {
  /** Keep only rows whose value is one of these (the checklist of the dropdown). */
  values?: ReadonlySet<string>;
  /** Keep rows whose value contains this text (case and accents ignored). */
  contains?: string;
  cond?: { op: ">" | "<" | "=" | "between"; a: number; b?: number };
}

export interface ViewSpec {
  sort?: { col: number; dir: 1 | -1 };
  filters: ReadonlyMap<number, ColumnFilter>;
}

export const NO_VIEW: ViewSpec = { filters: new Map() };

export const isIdentity = (v: ViewSpec): boolean => !v.sort && v.filters.size === 0;

const fold = (s: string) => s.normalize("NFD").replace(/\p{M}/gu, "").toLowerCase();

const collator = new Intl.Collator("fr", { numeric: true, sensitivity: "base" });

function passes(value: string, f: ColumnFilter, decimal: "," | "."): boolean {
  if (f.values && !f.values.has(value)) return false;
  if (f.contains && !fold(value).includes(fold(f.contains))) return false;
  if (f.cond) {
    const n = parseNumber(value, decimal);
    if (n === undefined) return false;
    const { op, a, b } = f.cond;
    if (op === ">" && !(n > a)) return false;
    if (op === "<" && !(n < a)) return false;
    if (op === "=" && n !== a) return false;
    if (op === "between" && !(n >= Math.min(a, b ?? a) && n <= Math.max(a, b ?? a))) return false;
  }
  return true;
}

/** The type of a column from its first data rows. */
export function typeOfColumn(model: CsvModel, col: number) {
  const values: string[] = [];
  const end = Math.min(model.records, model.dataStart + 200);
  for (let r = model.dataStart; r < end; r++) values.push(model.value(r, col));
  return columnType(values, model.dialect.decimal);
}

/**
 * The record indexes of the data rows as the view shows them. The file is not touched: this is
 * a lookup table from displayed row to file row.
 */
export function buildView(model: CsvModel, spec: ViewSpec): Int32Array {
  const first = model.dataStart;
  const total = Math.max(0, model.records - first);
  let rows = new Int32Array(total);
  for (let i = 0; i < total; i++) rows[i] = first + i;
  const decimal = model.dialect.decimal;
  if (spec.filters.size > 0) {
    const kept: number[] = [];
    next: for (let i = 0; i < total; i++) {
      const r = rows[i] as number;
      for (const [col, f] of spec.filters)
        if (!passes(model.value(r, col), f, decimal)) continue next;
      kept.push(r);
    }
    rows = Int32Array.from(kept);
  }
  const sort = spec.sort;
  if (sort) {
    const n = rows.length;
    const keys = new Array<number | string>(n);
    for (let i = 0; i < n; i++) {
      const v = model.value(rows[i] as number, sort.col);
      keys[i] = parseNumber(v, decimal) ?? v;
    }
    const idx = Int32Array.from({ length: n }, (_, i) => i);
    const cmp = (a: number, b: number): number => {
      const x = keys[a] as number | string;
      const y = keys[b] as number | string;
      if (x === "" || y === "") return x === y ? a - b : x === "" ? 1 : -1; // empty last, either way
      let c: number;
      if (typeof x === "number" && typeof y === "number") c = x - y;
      else if (typeof x === "number")
        c = -1; // numbers before text
      else if (typeof y === "number") c = 1;
      else c = collator.compare(x, y);
      return c === 0 ? a - b : c * sort.dir;
    };
    idx.sort(cmp);
    rows = Int32Array.from(idx, (i) => rows[i] as number);
  }
  return rows;
}

/** The distinct values of a column with how often they occur (for the filter checklist). */
export function distinctValues(
  model: CsvModel,
  col: number,
  limit = 1000,
): { value: string; count: number }[] {
  const counts = new Map<string, number>();
  for (let r = model.dataStart; r < model.records; r++) {
    const v = model.value(r, col);
    counts.set(v, (counts.get(v) ?? 0) + 1);
    if (counts.size > limit * 20) break;
  }
  return [...counts]
    .map(([value, count]) => ({ value, count }))
    .sort((a, b) => collator.compare(a.value, b.value))
    .slice(0, limit);
}
