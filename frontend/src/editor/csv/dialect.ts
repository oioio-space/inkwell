import { DELIMITERS, type Delimiter, scanRecord, splitRecord } from "./records";

/** How a CSV file is written: what the status bar shows and lets the person correct. */
export interface Dialect {
  delimiter: Delimiter;
  /** The first record names the columns. */
  header: boolean;
  /** Decimal separator of the numbers in the cells ("," in French files). */
  decimal: "," | ".";
  /** Cells may be quoted (RFC 4180). Off for a TSV that holds quotes as plain text. */
  quote?: boolean | undefined;
}

/** A `.tsv` or `.tab` holds quotes as plain text: no cell is quoted unless the person turns it on. */
export const quoteOf = (path: string): boolean => !/\.(tsv|tab)$/i.test(path);

/** The delimiter a file name fixes (.tsv and .tab: tab, .psv: pipe); none for .csv and the rest. */
export function delimiterOf(path: string): Delimiter | undefined {
  if (/\.(tsv|tab)$/i.test(path)) return "\t";
  if (/\.psv$/i.test(path)) return "|";
  return undefined;
}

/** What the extension says: the fixed delimiter, or the comma that `.csv` stands for. */
export const expectedDelimiter = (path: string): Delimiter => delimiterOf(path) ?? ",";

const SPACES = /[\s\u00a0\u202f]/g;
const HAS_SPACE = /[\s\u00a0\u202f]/;

/**
 * Reads a number written with the given decimal separator ("1 234,5", "-3,2", "1,234.5").
 * Returns undefined for anything that is not entirely a number.
 */
export function parseNumber(raw: string, decimal: "," | "."): number | undefined {
  const s = raw.trim();
  if (s === "" || s.length > 40) return undefined;
  // Spaces only group thousands ("1 234,5"); anything else with spaces is a phone number or id.
  if (HAS_SPACE.test(s) && !/^[-+]?\d{1,3}([\s\u00a0\u202f]\d{3})+([.,]\d+)?$/.test(s))
    return undefined;
  const t = s.replace(SPACES, "");
  let normal: string;
  if (decimal === ",") {
    if (t.includes(".") && !/^[-+]?\d{1,3}(\.\d{3})+(,\d+)?$/.test(t)) return undefined;
    normal = t.replaceAll(".", "").replace(",", ".");
  } else {
    if (t.includes(",") && !/^[-+]?\d{1,3}(,\d{3})+(\.\d+)?$/.test(t)) return undefined;
    normal = t.replaceAll(",", "");
  }
  if (!/^[-+]?(\d+\.?\d*|\.\d+)$/.test(normal)) return undefined;
  // Look-alikes stay text: 00123, phone numbers, 20-digit ids (display and sort only).
  const digits = normal.replace(/^[-+]/, "");
  if (/^0\d/.test(digits) || digits.replace(".", "").length > 15) return undefined;
  const n = Number(normal);
  return Number.isFinite(n) ? n : undefined;
}

const DATE_ISO = /^\d{4}-\d{2}-\d{2}$/;
const DATE_FR = /^\d{1,2}\/\d{1,2}\/\d{2,4}$/;

export type ColumnType = "number" | "date" | "text";

/** The type of a column from a sample of its values (empty cells ignored). */
export function columnType(values: string[], decimal: "," | "."): ColumnType {
  let n = 0;
  let nums = 0;
  let dates = 0;
  for (const v of values) {
    if (v.trim() === "") continue;
    n++;
    if (parseNumber(v, decimal) !== undefined) nums++;
    else if (DATE_ISO.test(v.trim()) || DATE_FR.test(v.trim())) dates++;
  }
  if (n === 0) return "text";
  if (nums / n >= 0.9) return "number";
  if (dates / n >= 0.9) return "date";
  return "text";
}

/** Sample records of a text, as arrays of cell values, for detection. */
function sample(text: string, delim: string, max: number): string[][] {
  const rows: string[][] = [];
  let pos = 0;
  while (pos < text.length && rows.length < max) {
    const r = scanRecord(text, pos, delim);
    rows.push(splitRecord(text, pos, r.end, delim).map((c) => c.value));
    pos = r.next;
  }
  return rows;
}

/**
 * The delimiter that gives the most regular table of two columns or more, or undefined when none
 * does (a single column, prose). On a tie `prefer` wins, then `,` `;` tab `|` in that order.
 */
export function findDelimiter(text: string, prefer?: Delimiter): Delimiter | undefined {
  const sep = /^sep=.\r?\n/.exec(text);
  const body = sep ? text.slice(sep[0].length) : text;
  const head = body.length > 200_000 ? body.slice(0, 200_000) : body;
  const order = prefer ? [prefer, ...DELIMITERS.filter((d) => d !== prefer)] : DELIMITERS;
  let best: Delimiter | undefined;
  let bestScore = 0;
  for (const d of order) {
    const rows = sample(head, d, 60).filter((r) => r.length > 1 || (r[0] ?? "") !== "");
    if (rows.length === 0) continue;
    const freq = new Map<number, number>();
    for (const r of rows) freq.set(r.length, (freq.get(r.length) ?? 0) + 1);
    let mode = 0;
    let modeN = 0;
    for (const [k, v] of freq) {
      if (v > modeN) {
        mode = k;
        modeN = v;
      }
    }
    if (mode < 2) continue;
    const score = (modeN / rows.length) * 1000 + mode;
    if (score > bestScore + 0.5) {
      best = d;
      bestScore = score;
    }
  }
  return best;
}

/** A text that reads as a table: worth offering as one (never forced). */
export function looksTabular(text: string): boolean {
  const d = findDelimiter(text);
  if (!d) return false;
  const rows = sample(text.slice(0, 200_000), d, 60).filter((r) => r.length > 1);
  return rows.length >= 3;
}

/** What a file opens with: its dialect, and what the content says against the extension, if anything. */
export interface Start {
  dialect: Dialect;
  contradiction?: { found: Delimiter; expected: Delimiter };
}

/**
 * The dialect to open a file with. The extension gives the starting point, the content confirms
 * it: `.tsv` is tab, `.psv` pipe, `.csv` the comma, or the semicolon of decimal-comma files, or
 * whatever else detection finds. A table that is clearly written another way is read that way and
 * reported as a `contradiction`. `remembered` is the person's earlier choice for this file.
 */
export function startDialect(path: string, text: string, remembered?: Delimiter): Start {
  const quote = quoteOf(path) ? {} : { quote: false as const };
  if (remembered) return { dialect: { ...detectDialect(text, remembered), ...quote } };
  const expected = expectedDelimiter(path);
  const found = findDelimiter(text, expected);
  const dialect = { ...detectDialect(text, found ?? expected), ...quote };
  const semicolonCsv = found === ";" && expected === "," && delimiterOf(path) === undefined;
  const named = /\.(csv|tsv|tab|psv)$/i.test(path);
  if (!named || !found || found === expected || semicolonCsv) return { dialect };
  return { dialect, contradiction: { found, expected } };
}

/**
 * Guesses delimiter, header and decimal separator from the start of the text. `forced` is the
 * delimiter the person chose; `prefer` the one the extension suggests, which wins ties and
 * stands in when nothing can be detected.
 */
export function detectDialect(text: string, forced?: Delimiter, prefer?: Delimiter): Dialect {
  // Excel's `sep=;` first line names the delimiter and is not part of the table.
  const sep = /^sep=(.)\r?\n/.exec(text);
  if (sep) {
    const d = sep[1] as string;
    const named = DELIMITERS.find((x) => x === d);
    return detectDialect(text.slice(sep[0].length), named ?? forced, prefer);
  }
  const head = text.length > 200_000 ? text.slice(0, 200_000) : text;
  const delimiter = forced ?? findDelimiter(head, prefer) ?? prefer ?? ",";
  const rows = sample(head, delimiter, 200);
  let comma = 0;
  let dot = 0;
  for (const r of rows.slice(1)) {
    for (const v of r) {
      if (/^[-+]?\d[\d\s  ]*,\d+$/.test(v.trim())) comma++;
      else if (/^[-+]?\d+\.\d+$/.test(v.trim())) dot++;
    }
  }
  const decimal: "," | "." = comma > dot ? "," : ".";
  return { delimiter, header: detectHeader(rows, decimal), decimal };
}

/** The first record is a header when it is all text and the records below hold numbers or dates. */
function detectHeader(rows: string[][], decimal: "," | "."): boolean {
  const first = rows[0];
  if (!first || rows.length < 2) return false;
  if (first.some((v) => v.trim() === "")) return false;
  if (first.some((v) => parseNumber(v, decimal) !== undefined)) return false;
  if (new Set(first).size !== first.length) return false;
  const body = rows.slice(1);
  const typed = first.some(
    (_, c) =>
      columnType(
        body.map((r) => r[c] ?? ""),
        decimal,
      ) !== "text",
  );
  return typed || body.every((r) => r.length === first.length);
}
