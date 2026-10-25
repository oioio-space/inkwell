/// Search worker: runs guarded (regex) scans off the UI thread, so the host can give up on it.
import type { Matcher } from "./query";
import { type Match, type Range, scanSync } from "./scan";

interface Job {
  id: number;
  text: string;
  matcher: Matcher;
  range: Range;
  limit: number;
}

const scope = self as unknown as {
  postMessage(message: unknown): void;
  onmessage: ((e: { data: Job }) => void) | null;
};

scope.onmessage = (e) => {
  const { id, text, matcher, range, limit } = e.data;
  try {
    const matches: Match[] = scanSync(text, matcher, range, limit);
    scope.postMessage({ id, matches });
  } catch (err) {
    scope.postMessage({ id, error: err instanceof Error ? err.message : String(err) });
  }
};
scope.postMessage({ ready: true });
