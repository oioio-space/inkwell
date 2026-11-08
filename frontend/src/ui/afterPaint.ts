/**
 * Runs non-urgent work just after the next paint (requestAnimationFrame, then a task), in arrival
 * order. A message landing at any moment then starts at the beginning of a frame and has the whole
 * frame to finish, instead of straddling the next refresh: in a fresh window the first preview,
 * language-server and history results each cost a missed refresh this way
 * (docs/perf/2026-11-07-mutter-instrumentation.md, "Cold missed refreshes"). A drain stops after
 * `SLICE_MS` and continues after the following paint. A hidden page paints nothing: work then runs
 * in a plain task, and a page hidden while work waits is caught by `FALLBACK_MS`.
 */
const SLICE_MS = 8;
const FALLBACK_MS = 100;
const queue: (() => void)[] = [];
let scheduled = false;

function drain(): void {
  scheduled = false;
  const until = performance.now() + SLICE_MS;
  while (queue.length > 0) {
    const work = queue.shift() as () => void;
    try {
      work();
    } catch (error) {
      // One failing callback must not hold back the others; the error still surfaces.
      reportError(error);
    }
    if (queue.length > 0 && performance.now() >= until) break;
  }
  if (queue.length > 0) schedule();
}

function schedule(): void {
  if (scheduled) return;
  scheduled = true;
  if (document.hidden || typeof requestAnimationFrame !== "function") {
    setTimeout(drain, 0);
    return;
  }
  let ran = false;
  const run = () => {
    if (ran) return;
    ran = true;
    clearTimeout(fallback);
    drain();
  };
  const fallback = setTimeout(run, FALLBACK_MS);
  requestAnimationFrame(() => setTimeout(run, 0));
}

/** Runs everything queued now, in order (tests play the paint with it). */
export function drainAfterPaint(): void {
  while (queue.length > 0) drain();
}

/** Queues `work` to run after the next paint, after everything queued before it. */
export function afterPaint(work: () => void): void {
  queue.push(work);
  schedule();
}
