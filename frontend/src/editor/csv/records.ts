/**
 * Low-level CSV reading over the raw text: where each record starts, and where each cell of a
 * record lies. Nothing here builds the whole table: a cell is looked up from its record only.
 *
 * Dialect of the reader (RFC 4180, lenient): a quote opens a quoted cell only at the start of a
 * cell; inside, `""` is a quote; text after the closing quote is kept up to the delimiter. The
 * record terminator is `\n` (the text is LF-normalised by the storage layer; a `\r` before it
 * is tolerated and not part of the last cell).
 */

export type Delimiter = "," | ";" | "\t" | "|";
export const DELIMITERS: readonly Delimiter[] = [",", ";", "\t", "|"];

const QUOTE = 34;
const LF = 10;
const CR = 13;

/** A cell located in the text: `[start, end)` is its raw text, quotes included. */
export interface Cell {
  start: number;
  end: number;
  value: string;
  quoted: boolean;
}

/** One replacement of the document text, in the coordinates of the text before the change. */
export interface Splice {
  pos: number;
  del: number;
  ins: string;
}

/**
 * Scans one record from `pos`. Returns where the next record starts and how many cells this one
 * has. `next === text.length` with a terminator consumed means "no further record".
 */
export function scanRecord(
  text: string,
  pos: number,
  delim: string,
  quote = true,
): { next: number; cols: number; end: number } {
  const len = text.length;
  const dc = delim.charCodeAt(0);
  let i = pos;
  let cols = 1;
  let atCellStart = true;
  while (i < len) {
    const c = text.charCodeAt(i);
    if (quote && atCellStart && c === QUOTE) {
      // Quoted cell: skip to the closing quote ("" is an escaped quote).
      i++;
      for (;;) {
        const q = text.indexOf('"', i);
        if (q < 0) return { next: len, cols, end: len };
        if (text.charCodeAt(q + 1) === QUOTE) {
          i = q + 2;
        } else {
          i = q + 1;
          break;
        }
      }
      atCellStart = false;
      continue;
    }
    if (c === LF) {
      const end = i > pos && text.charCodeAt(i - 1) === CR ? i - 1 : i;
      return { next: i + 1, cols, end };
    }
    if (c === dc) {
      cols++;
      atCellStart = true;
    } else {
      atCellStart = false;
    }
    i++;
  }
  return { next: len, cols, end: len };
}

/** The start of every record of `text`, and their cell counts. */
export function scanAll(
  text: string,
  delim: string,
  quote = true,
): { starts: number[]; cols: number[] } {
  const starts: number[] = [];
  const cols: number[] = [];
  let pos = 0;
  while (pos < text.length) {
    const r = scanRecord(text, pos, delim, quote);
    starts.push(pos);
    cols.push(r.cols);
    pos = r.next;
  }
  return { starts, cols };
}

/** The cells of the record `[start, end)` (end excludes the terminator). */
export function splitRecord(
  text: string,
  start: number,
  end: number,
  delim: string,
  upto = Number.POSITIVE_INFINITY,
  quote = true,
): Cell[] {
  const dc = delim.charCodeAt(0);
  const cells: Cell[] = [];
  let i = start;
  for (;;) {
    const cs = i;
    let value = "";
    let quoted = false;
    if (quote && i < end && text.charCodeAt(i) === QUOTE) {
      quoted = true;
      i++;
      for (;;) {
        const q = text.indexOf('"', i);
        if (q < 0 || q >= end) {
          value += text.slice(i, end);
          i = end;
          break;
        }
        value += text.slice(i, q);
        if (q + 1 < end && text.charCodeAt(q + 1) === QUOTE) {
          value += '"';
          i = q + 2;
        } else {
          i = q + 1;
          break;
        }
      }
    }
    // Plain cell, or what follows a closing quote.
    const from = i;
    while (i < end && text.charCodeAt(i) !== dc) i++;
    value += text.slice(from, i);
    cells.push({ start: cs, end: i, value, quoted });
    if (i >= end || cells.length > upto) break;
    i++; // the delimiter
    if (i === end) {
      cells.push({ start: end, end, value: "", quoted: false });
      break;
    }
  }
  return cells;
}

/** The raw form of a value: quoted only when it has to be (RFC 4180), or when `force`. */
export function rawCell(value: string, delim: string, force = false, quote = true): string {
  if (!quote) return value;
  const v = value.replaceAll("\r\n", "\n").replaceAll("\r", "\n");
  const needs =
    force || v.includes(delim) || v.includes('"') || v.includes("\n") || v.startsWith("\r");
  return needs ? `"${v.replaceAll('"', '""')}"` : v;
}
