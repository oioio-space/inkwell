/**
 * The palette's ranker (docs/plans/2026-11-25-ex0b2-palette.md §1, UX1 rule 48): ordered match
 * classes, each checked in one linear pass, best first:
 *   1 the label equals the query; 2 the label starts with it; 3 each query word starts a word of
 *   the label, in order, label words skipped allowed, each used once; 4 the query is one piece
 *   inside the label (KMP); 5 as 1 to 3 on a keyword; 6 as 4 on a keyword. Nothing else matches.
 * Order: available first; then, with a query, the class, then recency, then the label; without
 * one, recency, then the label. Texts are compared only in the counted loops below (rule P-8,
 * gate criterion `ranker-lint`): no built-in search, no regular expression.
 */
import { fold, foldQuery } from "./fold";

/** Character comparisons, placements in the order, and texts folded (P-1, P-8). */
export const rankStats = { comparisons: 0, selection: 0, folds: 0 };

/** A folded text and the offsets where its words start. */
interface Folded {
  readonly s: string;
  readonly starts: readonly number[];
}

export interface Entry<T> {
  readonly item: T;
  readonly label: Folded;
  readonly keywords: readonly Folded[];
}

export interface Prepared<T> {
  /** In label order (`Intl.Collator` of the language). */
  readonly entries: readonly Entry<T>[];
}

export interface Query {
  readonly s: string;
  readonly words: readonly string[];
  /** KMP's failure table of `s`. */
  readonly failure: Int32Array;
}

function folded(text: string): Folded {
  rankStats.folds++;
  const s = fold(text);
  const starts: number[] = [];
  for (let i = 0; i < s.length; i++) if (i === 0 || s[i - 1] === " ") starts.push(i);
  return { s, starts };
}

/** Folds every label and keyword once, and puts the items in label order. */
export function prepare<T>(
  items: readonly T[],
  label: (item: T) => string,
  keywords: (item: T) => readonly string[],
  language: string,
): Prepared<T> {
  const collator = new Intl.Collator(language);
  const shown = items.map((item) => ({ item, text: label(item) }));
  shown.sort((a, b) => collator.compare(a.text, b.text));
  return {
    entries: shown.map(({ item, text }) => ({
      item,
      label: folded(text),
      keywords: keywords(item).map(folded),
    })),
  };
}

/** A query as the ranker reads it: folded and cut, with its words and KMP table. */
export function query(text: string): Query {
  const s = foldQuery(text);
  const failure = new Int32Array(s.length);
  let k = 0;
  for (let i = 1; i < s.length; i++) {
    for (;;) {
      rankStats.comparisons++;
      if (s[i] === s[k]) {
        k++;
        break;
      }
      if (k === 0) break;
      k = failure[k - 1] ?? 0;
    }
    failure[i] = k;
  }
  return { s, words: s === "" ? [] : s.split(" "), failure };
}

/** 1 the text equals the query, 2 it starts with it, 0 neither. */
function prefix(t: string, q: string): number {
  if (q.length > t.length) return 0;
  for (let i = 0; i < q.length; i++) {
    rankStats.comparisons++;
    if (t[i] !== q[i]) return 0;
  }
  return q.length === t.length ? 1 : 2;
}

/** Whether `w` is written in `t` from `at`. */
function startsAt(t: string, at: number, w: string): boolean {
  if (at + w.length > t.length) return false;
  for (let i = 0; i < w.length; i++) {
    rankStats.comparisons++;
    if (t[at + i] !== w[i]) return false;
  }
  return true;
}

/** Each query word starts a word of the text, in order, each text word used once (greedy). */
function wordStarts(t: Folded, words: readonly string[]): boolean {
  let j = 0;
  for (const w of words) {
    let found = false;
    while (!found && j < t.starts.length) {
      found = startsAt(t.s, t.starts[j] ?? 0, w);
      j++;
    }
    if (!found) return false;
  }
  return true;
}

/** The query is one piece inside the text: Knuth-Morris-Pratt, at most 2 comparisons a character. */
function inside(t: string, q: Query): boolean {
  const m = q.s.length;
  if (m > t.length) return false;
  let k = 0;
  for (let i = 0; i < t.length; i++) {
    for (;;) {
      rankStats.comparisons++;
      if (t[i] === q.s[k]) {
        k++;
        break;
      }
      if (k === 0) break;
      k = q.failure[k - 1] ?? 0;
    }
    if (k === m) return true;
  }
  return false;
}

/** How a text matches: 1 equal, 2 prefix, 3 word starts, 4 one piece; 0 not at all. */
function matchText(t: Folded, q: Query): number {
  const p = prefix(t.s, q.s);
  if (p > 0) return p;
  if (wordStarts(t, q.words)) return 3;
  return inside(t.s, q) ? 4 : 0;
}

/** An entry's class for a query (1 to 6, see the header); 0 for no match or an empty query. */
export function classOf<T>(entry: Entry<T>, q: Query): number {
  if (q.s === "") return 0;
  const own = matchText(entry.label, q);
  if (own > 0) return own;
  let best = 0;
  for (const k of entry.keywords) {
    const c = matchText(k, q);
    if (c > 0 && (best === 0 || c < best)) best = c;
    if (best !== 0 && best < 4) break;
  }
  return best === 0 ? 0 : best < 4 ? 5 : 6;
}

/** How many recent commands are kept (and ordered first in their class). */
export const RECENT_MAX = 50;

/**
 * The items matching `q`, in the palette's order. One pass over the label-ordered entries puts
 * each match in its bucket (available or not, class), the recent ones at their recency slot.
 */
export function rank<T>(
  p: Prepared<T>,
  q: Query,
  opts: { available: (item: T) => boolean; recency: (item: T) => number },
): T[] {
  const buckets: { recent: (T | undefined)[]; rest: T[] }[] = [];
  const bucket = (ok: boolean, cls: number) => {
    const at = (ok ? 0 : 7) + cls;
    buckets[at] ??= { recent: [], rest: [] };
    return buckets[at];
  };
  const empty = q.s === "";
  for (const entry of p.entries) {
    const cls = empty ? 0 : classOf(entry, q);
    if (!empty && cls === 0) continue;
    rankStats.selection++;
    const r = opts.recency(entry.item);
    const b = bucket(opts.available(entry.item), cls);
    if (r >= 0 && r < RECENT_MAX) b.recent[r] = entry.item;
    else b.rest.push(entry.item);
  }
  const out: T[] = [];
  for (const b of buckets) {
    if (!b) continue;
    for (const item of b.recent) {
      if (item === undefined) continue;
      rankStats.selection++;
      out.push(item);
    }
    for (const item of b.rest) {
      rankStats.selection++;
      out.push(item);
    }
  }
  return out;
}
