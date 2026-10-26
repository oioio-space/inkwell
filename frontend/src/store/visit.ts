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
