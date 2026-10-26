import { next as A } from "@automerge/automerge";
import type { Session } from "./log";
import type { Author } from "./people";
import { headingsOf, sectionAt } from "./summary";
import type { HDoc } from "./types";

export type Period = "all" | "today" | "week" | "month";

/** What the history list is narrowed to: a person, a period, a section, a file (DIF-06). */
export interface Filters {
  /** The name of a person (their devices share it). */
  person?: string | undefined;
  period: Period;
  /** The title of a section. */
  section?: string | undefined;
  file?: string | undefined;
}

export const NO_FILTER: Filters = { period: "all" };

const DAY = 86_400_000;

const startOfDay = (ms: number) => {
  const d = new Date(ms);
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
};

/** The oldest moment a period still includes. */
export function since(period: Period, now: number): number {
  switch (period) {
    case "today":
      return startOfDay(now);
    case "week":
      return startOfDay(now) - 6 * DAY;
    case "month":
      return startOfDay(now) - 29 * DAY;
    default:
      return Number.NEGATIVE_INFINITY;
  }
}

const touched = new WeakMap<A.Doc<HDoc>, Map<string, string[]>>();

/**
 * The titles of the sections a session changed (the empty title for text before any heading).
 * Costly on a long text, so it is only asked while a section filter is on, and remembered.
 */
export function sectionsTouched(doc: A.Doc<HDoc>, session: Session): string[] {
  let cache = touched.get(doc);
  if (!cache) {
    cache = new Map();
    touched.set(doc, cache);
  }
  const hit = cache.get(session.id);
  if (hit) return hit;
  const text = session.after.length === 0 ? "" : A.view(doc, session.after).text;
  const heads = headingsOf(text);
  const found = new Set<string>();
  for (const p of A.diff(doc, session.before, session.after)) {
    if (p.path[0] !== "text" || p.path.length !== 2) continue;
    found.add(sectionAt(heads, p.path[1] as number)?.title ?? "");
  }
  const out = [...found];
  cache.set(session.id, out);
  return out;
}

export interface FilterContext {
  author: (actor: string) => Author;
  now: number;
  doc?: A.Doc<HDoc>;
  /** The file a session belongs to; today the one document open. */
  fileOf?: (session: Session) => string;
}

/** The sessions that pass the filters, and how many there were (the counter says « 3 sur 12 »). */
export function filterSessions(
  sessions: readonly Session[],
  f: Filters,
  ctx: FilterContext,
): { shown: Session[]; total: number } {
  const min = since(f.period, ctx.now);
  const shown = sessions.filter((s) => {
    if (f.person !== undefined && ctx.author(s.actor).name !== f.person) return false;
    if (s.end < min) return false;
    if (f.file !== undefined && ctx.fileOf && ctx.fileOf(s) !== f.file) return false;
    if (f.section !== undefined && ctx.doc && !sectionsTouched(ctx.doc, s).includes(f.section)) {
      return false;
    }
    return true;
  });
  return { shown, total: sessions.length };
}
