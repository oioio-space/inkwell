import { parseCsv } from "../ui/toolbar/csv";

/** One cell that changed: « B3 : 12 → 15 » (DIF-09). */
export interface CellChange {
  /** Spreadsheet reference: column letters, row number. */
  ref: string;
  before: string | undefined;
  after: string | undefined;
  row: number;
  col: number;
}

/** Column letters: 0 is A, 25 is Z, 26 is AA. */
export function columnName(col: number): string {
  let n = col;
  let out = "";
  for (; n >= 0; n = Math.floor(n / 26) - 1) out = String.fromCharCode(65 + (n % 26)) + out;
  return out;
}

/** The delimiter a file uses: the one that appears most on its first lines. */
export function delimiterOf(text: string): string {
  const head = text.split("\n", 5).join("\n");
  let best = ",";
  let bestCount = -1;
  for (const d of [",", ";", "\t"]) {
    const n = head.split(d).length - 1;
    if (n > bestCount) {
      best = d;
      bestCount = n;
    }
  }
  return best;
}

/** The cells that differ between two versions of a table, row by row and column by column. */
export function cellDiff(before: string, after: string): CellChange[] {
  const delimiter = delimiterOf(after || before);
  const a = parseCsv(before, delimiter);
  const b = parseCsv(after, delimiter);
  const out: CellChange[] = [];
  for (let row = 0; row < Math.max(a.length, b.length); row++) {
    const ra = a[row];
    const rb = b[row];
    for (let col = 0; col < Math.max(ra?.length ?? 0, rb?.length ?? 0); col++) {
      const x = ra?.[col];
      const y = rb?.[col];
      if (x === y) continue;
      out.push({ ref: `${columnName(col)}${row + 1}`, before: x, after: y, row, col });
    }
  }
  return out;
}
