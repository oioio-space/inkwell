import { safeGet, safeSet } from "./safeStorage";

const KEY = "inkwell.visit";

/** The document as this person left it: its heads and when. Kept on this device only. */
export interface Visit {
  heads: string[];
  at: number;
}

function all(): Record<string, Visit> {
  const raw = safeGet(KEY);
  if (!raw) return {};
  try {
    const value: unknown = JSON.parse(raw);
    return value && typeof value === "object" && !Array.isArray(value)
      ? (value as Record<string, Visit>)
      : {};
  } catch {
    return {};
  }
}

const valid = (v: unknown): v is Visit =>
  !!v &&
  typeof v === "object" &&
  Array.isArray((v as Visit).heads) &&
  (v as Visit).heads.every((h) => typeof h === "string") &&
  typeof (v as Visit).at === "number";

/** What this device remembers of the last visit to a document (`visit/<doc>`), if anything. */
export function lastVisit(doc: string): Visit | undefined {
  const v = all()[doc];
  return valid(v) ? v : undefined;
}

/** Remembers the document as it stands now: the next visit sees changes from this point. */
export function saveVisit(doc: string, heads: string[], at = Date.now()): void {
  const visits = all();
  visits[doc] = { heads, at };
  safeSet(KEY, JSON.stringify(visits));
}

const ACTORS = "inkwell.actors";
const KEEP_ACTORS = 200;

function allActors(): Record<string, unknown> {
  const raw = safeGet(ACTORS);
  try {
    const all: unknown = raw ? JSON.parse(raw) : {};
    return all && typeof all === "object" ? (all as Record<string, unknown>) : {};
  } catch {
    return {};
  }
}

/**
 * The Automerge actors this device has written under in a document (a new one each time the page
 * loads). They are kept here, on this device, and nowhere else: they alone are "mine", so no peer
 * can make its edits pass for mine by what it writes in the document.
 */
export function myActors(doc: string): Set<string> {
  const list = allActors()[doc];
  return new Set(Array.isArray(list) ? list.filter((a): a is string => typeof a === "string") : []);
}

export function rememberActor(doc: string, actor: string): void {
  const mine = myActors(doc);
  if (mine.has(actor)) return;
  mine.add(actor);
  safeSet(ACTORS, JSON.stringify({ ...allActors(), [doc]: [...mine].slice(-KEEP_ACTORS) }));
}
