/**
 * The commands last run from the palette on this device (rule P-4): at most 50, the latest
 * first, stored in `inkwell.palette.recent`. Storage that throws or holds anything but a list of
 * strings reads as none; an id no command has any more is dropped.
 */
import { RECENT_MAX } from "./rank";

const KEY = "inkwell.palette.recent";

function stored(): string[] {
  try {
    const raw: unknown = JSON.parse(localStorage.getItem(KEY) ?? "[]");
    return Array.isArray(raw) && raw.every((x) => typeof x === "string") ? raw : [];
  } catch {
    return [];
  }
}

/** The recent commands that still exist, the latest first. */
export function readRecents(known: (id: string) => boolean): string[] {
  return stored().filter(known).slice(0, RECENT_MAX);
}

/** Puts a command run from the palette at the front. */
export function recordRecent(id: string): void {
  const next = [id, ...stored().filter((x) => x !== id)].slice(0, RECENT_MAX);
  try {
    localStorage.setItem(KEY, JSON.stringify(next));
  } catch {
    // Storage refused (private window, quota): the palette just has no recents.
  }
}
