import { findAllSync } from "../search/find";
import { type Matcher, unescapeExtended } from "../search/query";
import { expandReplacement } from "../search/replace";
import type { Match } from "../search/scan";
import type { Range } from "./clipboard";
import type { CsvModel } from "./model";

/** Where to look: the whole table, one column, a selection (record coordinates) or the header. */
export type CellScope =
  | { kind: "sheet" }
  | { kind: "column"; col: number }
  | { kind: "selection"; ranges: readonly Range[] }
  | { kind: "headers" };

/** A cell that holds at least one occurrence. */
export interface CellHit {
  /** Record index in the text. */
  r: number;
  c: number;
  value: string;
  matches: Match[];
}

export interface CellSearchOptions {
  /** The whole cell must be the match. */
  entireCell?: boolean;
  limit?: number;
}

/**
 * The B6 engine applied to cell values, not to the raw text: a quote or a separator of the file
 * never matches, and a match never spans two cells. Results come in file order.
 */
export function findInCells(
  model: CsvModel,
  matcher: Matcher,
  scope: CellScope,
  opts: CellSearchOptions = {},
): CellHit[] {
  const hits: CellHit[] = [];
  const limit = opts.limit ?? 5000;
  if (matcher.empty) return hits;
  const visit = (r: number, c: number, value: string) => {
    if (value === "") return;
    let matches = findAllSync(value, matcher);
    if (opts.entireCell) matches = matches.filter((m) => m.from === 0 && m.to === value.length);
    if (matches.length) hits.push({ r, c, value, matches });
  };
  const records = model.records;
  const width = model.width;
  if (scope.kind === "headers") {
    if (model.dialect.header)
      for (let c = 0; c < width; c++) visit(model.preamble, c, model.value(model.preamble, c));
  } else if (scope.kind === "column") {
    for (let r = model.dataStart; r < records && hits.length < limit; r++)
      visit(r, scope.col, model.value(r, scope.col));
  } else if (scope.kind === "selection") {
    for (const g of scope.ranges)
      for (let r = g.r0; r <= Math.min(g.r1, records - 1) && hits.length < limit; r++) {
        const cells = model.recordUpTo(r, g.c1);
        for (let c = g.c0; c <= g.c1; c++) visit(r, c, cells[c]?.value ?? "");
      }
  } else {
    for (let r = model.dataStart; r < records && hits.length < limit; r++) {
      const cells = model.record(r);
      for (let c = 0; c < cells.length; c++) visit(r, c, cells[c]?.value ?? "");
    }
  }
  return hits;
}

export interface ReplaceCellOptions {
  /** Expand `$1`, `$&`... in the replacement. */
  regex?: boolean;
  /** Read `\n`, `\t` in the replacement. */
  extended?: boolean;
}

/** The value of the cell once every occurrence is replaced. */
export function replacedValue(
  hit: CellHit,
  template: string,
  opts: ReplaceCellOptions = {},
): string {
  const tpl = opts.extended ? unescapeExtended(template) : template;
  let out = "";
  let at = 0;
  for (const m of hit.matches) {
    out += hit.value.slice(at, m.from) + (opts.regex ? expandReplacement(tpl, m) : tpl);
    at = m.to;
  }
  return out + hit.value.slice(at);
}
