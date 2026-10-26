/** Column letters: 0 -> A, 25 -> Z, 26 -> AA. */
export function colName(c: number): string {
  let n = c + 1;
  let s = "";
  while (n > 0) {
    const m = (n - 1) % 26;
    s = String.fromCharCode(65 + m) + s;
    n = Math.floor((n - 1) / 26);
  }
  return s;
}

/** The index of a column name ("B" -> 1, "AA" -> 26), or undefined. */
export function colIndex(name: string): number | undefined {
  if (!/^[A-Za-z]{1,4}$/.test(name)) return undefined;
  let n = 0;
  for (const ch of name.toUpperCase()) n = n * 26 + (ch.charCodeAt(0) - 64);
  return n - 1;
}

/** "B12" for 0-based (row 11, col 1), with rows counted from 1 in the table's own numbering. */
export function refOf(row: number, col: number): string {
  return `${colName(col)}${row + 1}`;
}

/** "B12" or "B12:D20". */
export function rangeRef(r0: number, c0: number, r1: number, c1: number): string {
  const a = refOf(r0, c0);
  return r0 === r1 && c0 === c1 ? a : `${a}:${refOf(r1, c1)}`;
}

/** Reads "B12" or "b12:d20" into 0-based coordinates. */
export function parseRef(
  text: string,
): { r0: number; c0: number; r1: number; c1: number } | undefined {
  const m = /^\s*([A-Za-z]{1,4})(\d{1,7})(?::([A-Za-z]{1,4})(\d{1,7}))?\s*$/.exec(text);
  if (!m) return undefined;
  const c0 = colIndex(m[1] as string);
  const r0 = Number(m[2]) - 1;
  const c1 = m[3] ? colIndex(m[3]) : c0;
  const r1 = m[4] ? Number(m[4]) - 1 : r0;
  if (c0 === undefined || c1 === undefined || r0 < 0 || r1 < 0) return undefined;
  return { r0: Math.min(r0, r1), c0: Math.min(c0, c1), r1: Math.max(r0, r1), c1: Math.max(c0, c1) };
}
