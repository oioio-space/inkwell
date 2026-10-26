/** A match of a query against a path: how good it is, and which characters matched. */
export interface FuzzyMatch<T> {
  item: T;
  score: number;
  /** Indexes in the path of the matched characters, for highlighting. */
  at: number[];
}

const SEPARATORS = new Set(["/", "-", "_", ".", " "]);

/**
 * Scores `query` against `path` as a subsequence, case-insensitively: higher is better. Matches
 * that start words, follow one another, and fall in the file name rather than the folders score
 * more; a shorter path wins a tie. Returns undefined if the characters do not all appear in order.
 */
export function score(query: string, path: string): { score: number; at: number[] } | undefined {
  const q = query.toLowerCase();
  const p = path.toLowerCase();
  if (q === "") return { score: 0, at: [] };
  const nameStart = path.lastIndexOf("/") + 1;
  const at: number[] = [];
  const chars = [...q];
  // The last place each character can go while the ones after it still fit: a match at the
  // start of a word must not be taken if it leaves no room for the rest of the query.
  const latest: number[] = new Array(chars.length);
  for (let k = chars.length - 1, end = p.length; k >= 0; k--) {
    const i = p.lastIndexOf(chars[k] as string, end - 1);
    if (i < 0) return undefined;
    latest[k] = i;
    end = i;
  }
  let total = 0;
  let from = 0;
  let previous = -2;
  for (const [k, ch] of chars.entries()) {
    // Prefer a match at the start of a word, else the first one.
    let found = -1;
    for (let i = from; i <= (latest[k] as number); i++) {
      if (p[i] !== ch) continue;
      if (found < 0) found = i;
      if (i === 0 || SEPARATORS.has(p[i - 1] as string)) {
        found = i;
        break;
      }
    }
    if (found < 0) return undefined;
    at.push(found);
    let s = 1;
    if (found === previous + 1) s += 4;
    if (found === 0 || SEPARATORS.has(p[found - 1] as string)) s += 3;
    if (found >= nameStart) s += 2;
    total += s;
    previous = found;
    from = found + 1;
  }
  if (p.endsWith(q)) total += 6;
  if (p.slice(nameStart).startsWith(q)) total += 8;
  return { score: total - path.length / 100, at };
}

/** The items whose path matches the query, best first; all of them, in order, for an empty query. */
export function fuzzy<T>(
  query: string,
  items: readonly T[],
  pathOf: (item: T) => string,
  limit = 50,
) {
  const out: FuzzyMatch<T>[] = [];
  for (const item of items) {
    const m = score(query.trim(), pathOf(item));
    if (m) out.push({ item, score: m.score, at: m.at });
  }
  if (query.trim() !== "") out.sort((a, b) => b.score - a.score);
  return out.slice(0, limit);
}
