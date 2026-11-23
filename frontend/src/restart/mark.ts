/**
 * The one-shot mark between the page that leaves for a project change and the restart page (PC1,
 * rules N-9 and J-1): loading restart.html is what asks Go for the change, so the restart page
 * asks only when the page that got the person's confirmation left this mark, and takes it. The
 * mark names the change that was confirmed. A restart page opened or reloaded without it asks for
 * nothing. Where the session storage is not available no mark is found, and nothing is asked.
 */
const KEY = "inkwell.leave";

/**
 * The project changes a page leaves for: a take-over (PC1a), an accepted invitation, or the
 * return from a join that waits to the document open before it (PC1b).
 */
export type LeaveKind = "takeOver" | "join" | "return";

/** Leaves the mark of `kind`; called by the page that leaves, just before it navigates. */
export function markLeave(kind: LeaveKind): void {
  try {
    sessionStorage.setItem(KEY, kind);
  } catch {
    // No session storage: the restart page then asks for nothing.
  }
}

/** Takes the mark: the change confirmed, once after {@link markLeave}; undefined otherwise. */
export function takeLeaveMark(): LeaveKind | undefined {
  try {
    const marked = sessionStorage.getItem(KEY);
    sessionStorage.removeItem(KEY);
    return marked === "takeOver" || marked === "join" || marked === "return" ? marked : undefined;
  } catch {
    return undefined;
  }
}
