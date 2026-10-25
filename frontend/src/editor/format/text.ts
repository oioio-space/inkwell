// Grapheme- and word-aware offsets. A command may never cut a user-perceived character
// (a composed "é", an emoji ZWJ sequence, an Arabic cluster) in two.

const graphemes = new Intl.Segmenter(undefined, { granularity: "grapheme" });
const words = new Intl.Segmenter(undefined, { granularity: "word" });

/** Segmenting a window keeps a 5 MB single-line file O(1) per query. */
const WINDOW = 256;

/** Number of user-perceived characters (grapheme clusters) in `s`. */
export function graphemeCount(s: string): number {
  let n = 0;
  for (const _ of graphemes.segment(s)) n++;
  return n;
}

/** The grapheme boundary at or before (dir -1) / at or after (dir 1) `offset` in `text`. */
export function snapToGrapheme(text: string, offset: number, dir: -1 | 1): number {
  if (offset <= 0) return 0;
  if (offset >= text.length) return text.length;
  const lo = Math.max(0, offset - WINDOW);
  const hi = Math.min(text.length, offset + WINDOW);
  let before = lo;
  for (const { index } of graphemes.segment(text.slice(lo, hi))) {
    const start = lo + index;
    if (start === offset) return offset;
    if (start > offset) return dir === 1 ? start : before;
    before = start;
  }
  return dir === 1 ? hi : before;
}

/** The word touching `offset` as [from, to], preferring the one that contains it. */
export function wordAt(text: string, offset: number): [number, number] | null {
  const lo = Math.max(0, offset - WINDOW);
  const hi = Math.min(text.length, offset + WINDOW);
  let touching: [number, number] | null = null;
  for (const { index, segment, isWordLike } of words.segment(text.slice(lo, hi))) {
    if (!isWordLike) continue;
    // Unicode joins letters and underscores into one word; `_mot_` is a word in `_`s.
    const body = segment.replace(/^_+|_+$/g, "");
    if (body === "") continue;
    const from = lo + index + segment.indexOf(body);
    const to = from + body.length;
    if (from < offset && offset < to) return [from, to];
    if (from <= offset && offset <= to) touching ??= [from, to];
  }
  return touching;
}

export const leadRun = (s: string, ch: string): number => {
  let n = 0;
  while (s[n] === ch) n++;
  return n;
};

export const trailRun = (s: string, ch: string): number => {
  let n = 0;
  while (s[s.length - 1 - n] === ch) n++;
  return n;
};
