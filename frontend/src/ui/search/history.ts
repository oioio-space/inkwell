/** The searches a person ran before, kept per project (SRC-21). */

export type UiMode = "normal" | "wildcard" | "regex";
export type Scope = "file" | "selection" | "project";

export interface HistoryEntry {
  query: string;
  mode: UiMode;
  extended: boolean;
  caseSensitive: boolean;
  wholeWord: boolean;
  ignoreAccents: boolean;
  dotAll: boolean;
  visibleText: boolean;
  filters: string;
  scope: Scope;
}

export const HISTORY_SIZE = 20;

const key = (project: string) => `inkwell.search.history.${project}`;

/** The search modes and scopes, in the order the panel shows them. */
export const MODES: readonly UiMode[] = ["normal", "wildcard", "regex"];
export const SCOPES: readonly Scope[] = ["file", "selection", "project"];

/** Keeps only well-formed entries: the stored text is untrusted (another version, a hand edit). */
function sanitize(raw: unknown): HistoryEntry | undefined {
  if (typeof raw !== "object" || raw === null) return undefined;
  const r = raw as Record<string, unknown>;
  if (typeof r.query !== "string" || r.query === "") return undefined;
  const flag = (k: string) => r[k] === true;
  return {
    query: r.query.slice(0, 500),
    mode: MODES.includes(r.mode as UiMode) ? (r.mode as UiMode) : "normal",
    extended: flag("extended"),
    caseSensitive: flag("caseSensitive"),
    wholeWord: flag("wholeWord"),
    ignoreAccents: flag("ignoreAccents"),
    dotAll: flag("dotAll"),
    visibleText: flag("visibleText"),
    filters: typeof r.filters === "string" ? r.filters.slice(0, 200) : "",
    scope: SCOPES.includes(r.scope as Scope) ? (r.scope as Scope) : "project",
  };
}

export function loadHistory(project: string): HistoryEntry[] {
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(key(project)) ?? "[]");
    if (!Array.isArray(parsed)) return [];
    return parsed
      .map(sanitize)
      .filter((e): e is HistoryEntry => e !== undefined)
      .slice(0, HISTORY_SIZE);
  } catch {
    return [];
  }
}

function save(project: string, entries: HistoryEntry[]): void {
  try {
    localStorage.setItem(key(project), JSON.stringify(entries));
  } catch {
    // Private mode or blocked storage: searching still works, it just won't be remembered.
  }
}

const same = (a: HistoryEntry, b: HistoryEntry) => JSON.stringify(a) === JSON.stringify(b);

/** Adds a search (newest first, no duplicates, the last 20) and returns the new list. */
export function pushHistory(project: string, entry: HistoryEntry): HistoryEntry[] {
  const e = sanitize(entry);
  if (!e) return loadHistory(project);
  const next = [e, ...loadHistory(project).filter((x) => !same(x, e))].slice(0, HISTORY_SIZE);
  save(project, next);
  return next;
}

export function clearHistory(project: string): void {
  try {
    localStorage.removeItem(key(project));
  } catch {
    // Nothing to forget if nothing could be stored.
  }
}
