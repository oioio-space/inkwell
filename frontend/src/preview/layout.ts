/**
 * The geometry of the preview: pages stacked in a scroller, each `scale` CSS pixels per point,
 * `gap` pixels apart. Everything is arithmetic on page sizes, so the visible pages and the
 * current page are known without measuring the DOM (and are testable without a layout engine).
 */

export interface PageBox {
  /** Size in points. */
  width: number;
  height: number;
}

export interface Layout {
  /** Top of each page, in CSS pixels from the top of the stack. */
  tops: number[];
  /** Height of each page in CSS pixels. */
  heights: number[];
  /** Height of the whole stack. */
  total: number;
}

/** CSS pixels per point at 100 %: Typst's point is 1/72 inch, a CSS pixel 1/96. */
export const PX_PER_PT = 96 / 72;
export const ZOOM_MIN = 0.25;
export const ZOOM_MAX = 4;
export const GAP = 16;

export const clampZoom = (z: number): number =>
  Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, Number.isFinite(z) ? z : 1));

/** The next zoom step up or down: 25 %, 33 %, 50 %, 67 %, 75 %, 90 %, 100 %, 125 %, 150 %, 200 %, 300 %, 400 %. */
const STEPS = [0.25, 0.33, 0.5, 0.67, 0.75, 0.9, 1, 1.25, 1.5, 2, 3, 4];
export function stepZoom(zoom: number, direction: 1 | -1): number {
  if (direction > 0) return STEPS.find((s) => s > zoom + 0.005) ?? ZOOM_MAX;
  return [...STEPS].reverse().find((s) => s < zoom - 0.005) ?? ZOOM_MIN;
}

/** The zoom that makes a page `pageWidth` points wide fill `available` CSS pixels. */
export const fitWidth = (available: number, pageWidth: number): number =>
  clampZoom(available / (pageWidth * PX_PER_PT));

/** Stacks `count` pages; those not yet known take the size of `fallback`. */
export function stack(
  count: number,
  box: (page: number) => PageBox | undefined,
  fallback: PageBox,
  zoom: number,
  gap = GAP,
): Layout {
  const scale = zoom * PX_PER_PT;
  const tops: number[] = [];
  const heights: number[] = [];
  let y = 0;
  for (let n = 1; n <= count; n++) {
    const b = box(n) ?? fallback;
    tops.push(y);
    const h = b.height * scale;
    heights.push(h);
    y += h + gap;
  }
  return { tops, heights, total: Math.max(0, y - gap) };
}

/** The 1-based pages that intersect the viewport, widened by `margin` pixels each way. */
export function visiblePages(
  layout: Layout,
  scrollTop: number,
  viewport: number,
  margin = 0,
): { first: number; last: number } | undefined {
  const n = layout.tops.length;
  if (n === 0) return undefined;
  const from = scrollTop - margin;
  const to = scrollTop + viewport + margin;
  let first = 0;
  while (first < n - 1 && (layout.tops[first] ?? 0) + (layout.heights[first] ?? 0) < from) first++;
  let last = first;
  while (last < n - 1 && (layout.tops[last + 1] ?? 0) <= to) last++;
  return { first: first + 1, last: last + 1 };
}

/** The page that fills most of the viewport (the first of two equal ones). */
export function currentPage(layout: Layout, scrollTop: number, viewport: number): number {
  const range = visiblePages(layout, scrollTop, viewport);
  if (!range) return 0;
  let best = range.first;
  let bestSeen = -1;
  for (let n = range.first; n <= range.last; n++) {
    const top = layout.tops[n - 1] ?? 0;
    const bottom = top + (layout.heights[n - 1] ?? 0);
    const seen = Math.min(bottom, scrollTop + viewport) - Math.max(top, scrollTop);
    if (seen > bestSeen + 0.5) {
      best = n;
      bestSeen = seen;
    }
  }
  return best;
}

/** Where to scroll to bring point `y` (points) of `page` a third of the way down the viewport. */
export function scrollTarget(
  layout: Layout,
  page: number,
  y: number,
  zoom: number,
  viewport: number,
): number {
  const top = layout.tops[page - 1];
  if (top === undefined) return 0;
  return Math.max(0, Math.min(layout.total - viewport, top + y * zoom * PX_PER_PT - viewport / 3));
}
