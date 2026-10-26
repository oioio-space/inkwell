import { describe, expect, it } from "vitest";
import {
  clampZoom,
  currentPage,
  fitWidth,
  PX_PER_PT,
  scrollTarget,
  stack,
  stepZoom,
  visiblePages,
} from "./layout";

const A4 = { width: 600, height: 800 };
// 10 pages at 100 %: each 800 pt = 1066.67 px high, 16 px apart.
const layout = stack(10, () => undefined, A4, 1);
const H = 800 * PX_PER_PT;

describe("preview layout", () => {
  it("PRV-55 stacks the pages with a gap, and unknown pages take the fallback size", () => {
    expect(layout.tops[0]).toBe(0);
    expect(layout.tops[1]).toBeCloseTo(H + 16);
    expect(layout.total).toBeCloseTo(10 * H + 9 * 16);
    const mixed = stack(2, (n) => (n === 2 ? { width: 300, height: 100 } : undefined), A4, 2);
    expect(mixed.heights[1]).toBeCloseTo(100 * 2 * PX_PER_PT);
    expect(mixed.heights[0]).toBeCloseTo(800 * 2 * PX_PER_PT);
  });

  it("PRV-56 only the pages that intersect the viewport are visible; a margin widens it", () => {
    expect(visiblePages(layout, 0, 600)).toEqual({ first: 1, last: 1 });
    expect(visiblePages(layout, H - 100, 600)).toEqual({ first: 1, last: 2 });
    expect(visiblePages(layout, 3 * (H + 16), 300)).toEqual({ first: 4, last: 4 });
    expect(visiblePages(layout, 3 * (H + 16), 300, H)).toEqual({ first: 3, last: 5 });
    expect(visiblePages(layout, 1e9, 600)).toEqual({ first: 10, last: 10 });
    expect(
      visiblePages(
        stack(0, () => undefined, A4, 1),
        0,
        600,
      ),
    ).toBeUndefined();
  });

  it("PRV-57 the current page is the one that fills most of the view", () => {
    expect(currentPage(layout, 0, 600)).toBe(1);
    expect(currentPage(layout, H - 100, 600)).toBe(2); // 100 px of page 1, 500 of page 2
    expect(currentPage(layout, H - 400, 600)).toBe(1); // 400 of page 1, 200 of page 2
    expect(currentPage(layout, 1e9, 600)).toBe(10);
    expect(
      currentPage(
        stack(0, () => undefined, A4, 1),
        0,
        600,
      ),
    ).toBe(0);
  });

  it("PRV-58 zoom steps stay in range and fit-to-width fills the space", () => {
    expect(stepZoom(1, 1)).toBe(1.25);
    expect(stepZoom(1, -1)).toBe(0.9);
    expect(stepZoom(4, 1)).toBe(4);
    expect(stepZoom(0.25, -1)).toBe(0.25);
    expect(stepZoom(1.1, 1)).toBe(1.25);
    expect(clampZoom(99)).toBe(4);
    expect(clampZoom(0)).toBe(0.25);
    expect(clampZoom(Number.NaN)).toBe(1);
    expect(fitWidth(800, 600)).toBeCloseTo(1);
    expect(fitWidth(100, 600)).toBe(0.25);
  });

  it("PRV-59 the scroll target shows a point a third of the way down, within bounds", () => {
    expect(scrollTarget(layout, 2, 0, 1, 600)).toBeCloseTo(H + 16 - 200);
    expect(scrollTarget(layout, 1, 0, 1, 600)).toBe(0);
    expect(scrollTarget(layout, 10, 800, 1, 600)).toBeCloseTo(layout.total - 600);
    expect(scrollTarget(layout, 99, 0, 1, 600)).toBe(0);
  });
});
