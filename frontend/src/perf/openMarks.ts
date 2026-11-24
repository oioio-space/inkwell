/**
 * PERF-EXT-VIEWER-CSV (spec §15.4, plan EX0), end-to-end builds only: `ex0:open` when the file
 * panel's handler starts, `ex0:first-render` after the frame in which the opened table's first
 * cells are in the page, and the measure `ex0:open-render` between them, read by
 * frontend/e2e/ex0.spec.ts through `performance.getEntriesByName`.
 */
export function markOpen(path: string): void {
  if (import.meta.env.VITE_E2E_OBSERVE !== "1") return;
  for (const name of ["ex0:open", "ex0:first-render"]) performance.clearMarks(name);
  performance.clearMeasures("ex0:open-render");
  performance.mark("ex0:open", { detail: path });
  const deadline = performance.now() + 10_000;
  const poll = () => {
    if (document.querySelector('[role="grid"] [role="gridcell"]')) {
      // The cells are in the page for this frame: the mark goes after it is painted.
      requestAnimationFrame(() => {
        performance.mark("ex0:first-render");
        performance.measure("ex0:open-render", "ex0:open", "ex0:first-render");
      });
    } else if (performance.now() < deadline) requestAnimationFrame(poll);
  };
  requestAnimationFrame(poll);
}
