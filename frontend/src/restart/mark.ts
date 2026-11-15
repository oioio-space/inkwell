/**
 * The one-shot mark between the page that leaves for a project change and the restart page (PC1a,
 * rule N-9): loading restart.html is what asks Go for the take-over, so the restart page asks only
 * when the page that got the person's confirmation left this mark, and takes it. A restart page
 * opened or reloaded without it asks for nothing. Where the session storage is not available no
 * mark is found, and nothing is asked.
 */
const KEY = "inkwell.leave";

/** Leaves the mark; called by the page that leaves, just before it navigates. */
export function markLeave(): void {
  try {
    sessionStorage.setItem(KEY, "1");
  } catch {
    // No session storage: the restart page then asks for nothing.
  }
}

/** Takes the mark: true once after {@link markLeave}, false otherwise. */
export function takeLeaveMark(): boolean {
  try {
    const marked = sessionStorage.getItem(KEY) === "1";
    sessionStorage.removeItem(KEY);
    return marked;
  } catch {
    return false;
  }
}
