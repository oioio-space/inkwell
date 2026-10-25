import type { FileHits, Hit, ProjectHits, ResolvedHit } from "../../editor/search/project";
import { t } from "../../i18n";

/** One finished search, as the panel keeps it (stacked until "Effacer"). */
export interface SearchRun {
  id: number;
  /** What was searched, for the title of the stack entry. */
  label: string;
  /** Signature of the query + scope, to know whether the field still matches this run. */
  signature: string;
  results: ProjectHits;
  open: boolean;
  collapsed: ReadonlySet<string>;
}

/** "37 résultats dans 5 fichiers", with the singulars right. */
export function countLabel(total: number, files: number): string {
  if (total === 0) return t("search.count.none");
  if (total === 1) return t("search.count.one");
  if (files === 1) return t("search.count.inOneFile", { count: total });
  return t("search.count.many", { count: total, files });
}

export interface Located {
  path: string;
  /** Index of the hit in its file's list. */
  index: number;
  hit: Hit;
  at: ResolvedHit;
}

/** The hits of a run in reading order, with where they are now; stale ones are left out. */
export function liveHits(
  files: ReadonlyArray<FileHits>,
  locate: (path: string, hit: Hit) => ResolvedHit,
): Located[] {
  const out: Located[] = [];
  for (const f of files) {
    f.hits.forEach((hit, index) => {
      const at = locate(f.path, hit);
      if (!at.stale) out.push({ path: f.path, index, hit, at });
    });
  }
  return out;
}

/** Index of the hit after (or before) `current`, wrapping around; -1 when there is none. */
export function stepIndex(count: number, current: number, backwards: boolean): number {
  if (count === 0) return -1;
  if (current < 0) return backwards ? count - 1 : 0;
  return (current + (backwards ? count - 1 : 1)) % count;
}

/** The whole lines that hold hits, one per line, for "Copier les lignes". */
export function linesOf(text: string, hits: ReadonlyArray<{ line: number }>): string[] {
  const lines = text.split("\n");
  const seen = new Set<number>();
  const out: string[] = [];
  for (const h of hits) {
    if (seen.has(h.line)) continue;
    seen.add(h.line);
    out.push(lines[h.line - 1] ?? "");
  }
  return out;
}
