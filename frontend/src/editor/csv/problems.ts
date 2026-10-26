import type { CsvModel } from "./model";

export type ProblemKind = "unclosedQuote" | "strayQuote" | "backslashQuote";

/** Something odd in the file's quoting. Nothing is repaired: the text stays as it is. */
export interface Problem {
  /** The record's number in the file, from 1. */
  line: number;
  kind: ProblemKind;
}

/**
 * Finds quoting that a spreadsheet would read differently: a quote that never closes, a quote
 * in the middle of a cell or after a closing quote, and a backslash-escaped quote (`\"`, which
 * CSV does not use: the quote is doubled instead). The file is read leniently either way.
 */
export function findProblems(model: CsvModel, limit = 50): Problem[] {
  const out: Problem[] = [];
  const text = model.text;
  if (!model.quoting || !text.includes('"')) return out;
  let q = text.indexOf('"');
  for (let r = model.preamble; r < model.records && out.length < limit; r++) {
    const start = model.start(r);
    const end = model.index.endOf(text, r);
    if (q >= 0 && q < start) q = text.indexOf('"', start);
    if (q < 0) break; // no quote left anywhere after this record
    if (q >= end) continue;
    const line = r + 1;
    let kind: ProblemKind | undefined;
    for (const cell of model.record(r)) {
      const raw = text.slice(cell.start, cell.end);
      if (!raw.includes('"')) continue;
      if (cell.quoted) {
        const body = raw.slice(1);
        const closing = closingQuote(body);
        if (closing < 0) kind = "unclosedQuote";
        else if (closing < body.length - 1) kind = "strayQuote";
      } else if (raw.includes('\\"')) {
        kind = "backslashQuote";
      } else {
        kind = "strayQuote";
      }
      if (kind) break;
    }
    if (kind) out.push({ line, kind });
  }
  return out;
}

/** Index of the quote that closes a quoted cell's body ("" is an escaped quote), or -1. */
function closingQuote(body: string): number {
  let i = 0;
  for (;;) {
    const q = body.indexOf('"', i);
    if (q < 0) return -1;
    if (body.charAt(q + 1) === '"') i = q + 2;
    else return q;
  }
}
