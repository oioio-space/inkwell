import type { Matcher } from "./query";
import {
  buildView,
  isLowSurrogate,
  originalEnd,
  originalStart,
  type TextView,
  viewIndexAt,
} from "./textmap";

/** One occurrence. Offsets are UTF-16 units into the original text (what CodeMirror uses). */
export interface Match {
  from: number;
  to: number;
  /** 1-based line of `from`. */
  line: number;
  /** 1-based column of `from` in its line. */
  column: number;
  /** The text matched, exactly as written in the document. */
  matched: string;
  /** The line around the hit, cut with "…" when long. */
  lineText: string;
  /** The hit inside `lineText`. */
  hitFrom: number;
  hitTo: number;
  /** The hit itself was cut in `lineText` (a very long match). */
  hitTruncated: boolean;
  /** Capture groups 1…n, from the original text. */
  groups: Array<string | undefined>;
  /** Named capture groups. */
  named: Record<string, string | undefined>;
}

export interface Range {
  from?: number | undefined;
  to?: number | undefined;
}

/** Upper bound of matches reported for one text. */
export const DEFAULT_LIMIT = 20_000;

/** Characters kept around a hit in an excerpt, and of the hit itself. */
const CONTEXT = 60;
const MAX_HIT = 120;

const NO_NAMES: Record<string, string | undefined> = Object.freeze({});

const cache = new Map<string, RegExp>();

function regexFor(m: Matcher, indices: boolean): RegExp {
  const flags = indices ? `${m.flags}d` : m.flags;
  const key = `${flags}/${m.source}`;
  let re = cache.get(key);
  if (!re) {
    if (cache.size > 64) cache.clear();
    re = new RegExp(m.source, flags);
    cache.set(key, re);
  }
  re.lastIndex = 0;
  return re;
}

/**
 * Finds every occurrence, synchronously. Zero-length matches are reported once each and the
 * scan steps over them, so `a*` or `^` never loop. In a range, only matches that lie wholly
 * inside it count; the text outside still decides word boundaries and `^`/`$`.
 */
export function scanSync(
  text: string,
  matcher: Matcher,
  range: Range = {},
  limit: number = DEFAULT_LIMIT,
): Match[] {
  if (matcher.empty) return [];
  const view: TextView | null = buildView(text, { accents: matcher.fold, markup: matcher.visible });
  const hay = view ? view.text : text;
  const withIndices = view !== null;
  const re = regexFor(matcher, withIndices);

  const lo = Math.max(0, range.from ?? 0);
  const hi = Math.min(text.length, range.to ?? text.length);
  if (lo > hi) return [];
  const hayLo = view ? viewIndexAt(view, lo) : lo;
  const hayHi = view ? viewIndexAt(view, hi) : hi;

  // Lines are found incrementally: matches arrive in ascending order.
  let line = 1;
  let lineStart = 0;
  let nextBreak = text.indexOf("\n");

  const out: Match[] = [];
  re.lastIndex = hayLo;
  while (out.length < limit) {
    const m = re.exec(hay);
    if (!m) break;
    const i = m.index;
    const len = m[0].length;
    if (i > hayHi) break;
    if (len === 0) {
      const cp = hay.codePointAt(i);
      re.lastIndex = i + (cp !== undefined && cp > 0xffff ? 2 : 1);
    }
    if (i + len > hayHi) continue;

    let from: number;
    let to: number;
    if (view) {
      from = originalStart(view, i);
      to = len === 0 ? from : originalEnd(view, i + len);
    } else {
      from = i;
      to = i + len;
    }
    if (from < lo || to > hi) continue;

    while (nextBreak !== -1 && nextBreak < from) {
      line++;
      lineStart = nextBreak + 1;
      nextBreak = text.indexOf("\n", lineStart);
    }
    const lineEnd = nextBreak === -1 ? text.length : nextBreak;
    out.push(makeMatch(text, m, view, from, to, line, lineStart, lineEnd));
  }
  return out;
}

function groupText(
  text: string,
  view: TextView | null,
  idx: [number, number] | undefined,
  plain: string | undefined,
): string | undefined {
  if (!view) return plain;
  if (!idx) return undefined;
  const [a, b] = idx;
  if (a === b) return "";
  return text.slice(originalStart(view, a), originalEnd(view, b));
}

function makeMatch(
  text: string,
  m: RegExpExecArray,
  view: TextView | null,
  from: number,
  to: number,
  line: number,
  lineStart: number,
  lineEnd: number,
): Match {
  // The excerpt: a window of the line around the hit, never the whole (possibly huge) line.
  const hitEnd = Math.min(to, lineEnd);
  const hitTruncated = hitEnd - from > MAX_HIT;
  const shownEnd = hitTruncated ? from + MAX_HIT : hitEnd;
  let ws = Math.max(lineStart, from - CONTEXT);
  let we = Math.min(lineEnd, shownEnd + CONTEXT);
  if (ws > lineStart && isLowSurrogate(text.charCodeAt(ws))) ws++;
  if (we < lineEnd && isLowSurrogate(text.charCodeAt(we))) we++;
  const head = ws > lineStart ? "…" : "";
  const tail = we < lineEnd || hitTruncated ? "…" : "";
  const lineText = head + text.slice(ws, we) + tail;

  const groups: Array<string | undefined> = [];
  let named = NO_NAMES;
  if (m.length > 1) {
    const ind = (m as RegExpExecArray & { indices?: Array<[number, number] | undefined> }).indices;
    for (let k = 1; k < m.length; k++) groups.push(groupText(text, view, ind?.[k], m[k]));
    if (m.groups) {
      named = {};
      const nind = (
        m as RegExpExecArray & { indices?: { groups?: Record<string, [number, number]> } }
      ).indices?.groups;
      for (const key of Object.keys(m.groups)) {
        named[key] = groupText(text, view, nind?.[key], m.groups[key]);
      }
    }
  }
  return {
    from,
    to,
    line,
    column: from - lineStart + 1,
    matched: text.slice(from, to),
    lineText,
    hitFrom: head.length + (from - ws),
    hitTo: head.length + (shownEnd - ws),
    hitTruncated,
    groups,
    named,
  };
}
