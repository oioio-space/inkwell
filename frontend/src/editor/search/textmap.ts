/**
 * A searchable "view" of a text that differs from the original (accents removed, markup
 * hidden) together with the exact mapping back to original UTF-16 offsets.
 *
 * Only combining-mark removal is done on Latin and Greek letters: Hangul, kana (が keeps its
 * dakuten) and Cyrillic (й is not и) are left alone, because there the mark changes the letter.
 */

/** Does this text hold anything the accent folding would touch? */
const FOLDABLE = /[À-ɏͰ-ϿḀ-῿]|\p{M}/u;

const MARKS = /\p{M}/gu;

export interface TextView {
  /** The text that is actually searched. */
  text: string;
  /** For each UTF-16 unit of `text`: where its character starts in the original. */
  starts: Uint32Array;
  /** For each UTF-16 unit of `text`: where its character ends in the original (marks included). */
  ends: Uint32Array;
  /** Length of the original text. */
  originalLength: number;
}

export interface ViewOptions {
  accents: boolean;
  /** Hide Markdown/Typst emphasis and code marks (`*`, backtick, word-edge `_`). */
  markup: boolean;
}

function canFold(c: number): boolean {
  return (c >= 0xc0 && c <= 0x24f) || (c >= 0x370 && c <= 0x3ff) || (c >= 0x1e00 && c <= 0x1fff);
}

function isWordChar(c: number): boolean {
  return (c >= 48 && c <= 57) || (c >= 65 && c <= 90) || (c >= 97 && c <= 122) || c > 0x7f;
}

function isMarkup(text: string, i: number, c: number): boolean {
  if (c === 42 || c === 96) return true; // * and `
  if (c !== 95) return false; // _
  // An underscore between two word characters is part of a name (pg_ctl), not emphasis.
  const inside =
    i > 0 &&
    i + 1 < text.length &&
    isWordChar(text.charCodeAt(i - 1)) &&
    isWordChar(text.charCodeAt(i + 1));
  return !inside;
}

/** The second half of a surrogate pair: an offset there cuts a character in two. */
export function isLowSurrogate(c: number): boolean {
  return c >= 0xdc00 && c <= 0xdfff;
}

/** Removes the accents of a query string; the same folding the text gets. */
export function foldAccents(s: string): string {
  return buildView(s, { accents: true, markup: false })?.text ?? s;
}

/**
 * Builds the view of `text`, or null when it would be identical to the original (so callers
 * can search the original with no mapping at all).
 */
export function buildView(text: string, opts: ViewOptions): TextView | null {
  if (!opts.markup && !(opts.accents && FOLDABLE.test(text))) return null;
  const L = text.length;
  const starts = new Uint32Array(L + 1);
  const ends = new Uint32Array(L + 1);
  let out = "";
  let n = 0; // units written
  let lastUnits = 0; // UTF-16 units of the last kept character
  let runStart = -1; // start in `text` of the pending verbatim run

  const flush = (upTo: number) => {
    if (runStart < 0) return;
    for (let j = runStart; j < upTo; j++) {
      starts[n] = j;
      ends[n] = j + 1;
      n++;
    }
    out += text.slice(runStart, upTo);
    lastUnits = isLowSurrogate(text.charCodeAt(upTo - 1)) ? 2 : 1;
    runStart = -1;
  };

  let i = 0;
  while (i < L) {
    const c = text.charCodeAt(i);
    if (opts.markup && c < 0x80 && isMarkup(text, i, c)) {
      flush(i);
      i++;
      continue;
    }
    if (opts.accents && c >= 0x300 && c <= 0x36f) {
      // A combining mark is dropped; the character before it now ends after it.
      flush(i);
      if (n > 0) {
        ends[n - 1] = i + 1;
        if (lastUnits === 2 && n > 1) ends[n - 2] = i + 1;
      }
      i++;
      continue;
    }
    if (opts.accents && canFold(c)) {
      const d = text.charAt(i).normalize("NFD");
      const b = d.replace(MARKS, "");
      if (b.length === 1 && d.length > 1) {
        flush(i);
        out += b;
        starts[n] = i;
        ends[n] = i + 1;
        n++;
        lastUnits = 1;
        i++;
        continue;
      }
    }
    if (runStart < 0) runStart = i;
    i++;
  }
  flush(L);
  if (n === L && out === text) return null;
  return {
    text: out,
    starts: starts.subarray(0, n),
    ends: ends.subarray(0, n),
    originalLength: L,
  };
}

/** First index k of the view whose original start is >= pos. */
export function viewIndexAt(view: TextView, pos: number): number {
  let lo = 0;
  let hi = view.starts.length;
  while (lo < hi) {
    const mid = (lo + hi) >>> 1;
    if ((view.starts[mid] as number) < pos) lo = mid + 1;
    else hi = mid;
  }
  return lo;
}

/** Original offset of the start of view position k (the original length past the end). */
export function originalStart(view: TextView, k: number): number {
  return k >= view.starts.length ? view.originalLength : (view.starts[k] as number);
}

/** Original offset just after the last character of a non-empty view span ending at k. */
export function originalEnd(view: TextView, k: number): number {
  return view.ends[k - 1] as number;
}
