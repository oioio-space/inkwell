import type { Matcher } from "./query";
import { defaultRunner, type SearchRunner } from "./runner";
import { DEFAULT_LIMIT, type Match, type Range, scanSync } from "./scan";

export { SearchTooCostly } from "./runner";
export type { Match, Range } from "./scan";

export interface FindOptions extends Range {
  /** Most matches reported (default 20 000). */
  limit?: number | undefined;
  runner?: SearchRunner | undefined;
}

/**
 * Every occurrence of `matcher` in `text` (optionally only within `from`…`to`).
 * Queries that can run away (regex, several wildcards) go through the runner, which gives up
 * after 200 ms and rejects with SearchTooCostly; the others run right here.
 */
export async function findAll(
  text: string,
  matcher: Matcher,
  opts: FindOptions = {},
): Promise<Match[]> {
  const range = { from: opts.from, to: opts.to };
  const limit = opts.limit ?? DEFAULT_LIMIT;
  if (!matcher.guarded) return scanSync(text, matcher, range, limit);
  return (opts.runner ?? defaultRunner()).run({ text, matcher, range, limit });
}

/** Same as findAll, on the calling thread with no time limit (small texts, tests, workers). */
export function findAllSync(text: string, matcher: Matcher, opts: FindOptions = {}): Match[] {
  return scanSync(text, matcher, { from: opts.from, to: opts.to }, opts.limit ?? DEFAULT_LIMIT);
}
