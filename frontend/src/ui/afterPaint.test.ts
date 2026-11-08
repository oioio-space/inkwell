import { afterEach, describe, expect, it, vi } from "vitest";
import { afterPaint } from "./afterPaint";

const paintThenTask = () =>
  new Promise<void>((done) =>
    requestAnimationFrame(() => setTimeout(() => setTimeout(done, 0), 0)),
  );

describe("afterPaint", () => {
  afterEach(() => vi.restoreAllMocks());

  it("runs nothing synchronously, then everything in arrival order after the next paint", async () => {
    const seen: number[] = [];
    afterPaint(() => seen.push(1));
    afterPaint(() => seen.push(2));
    expect(seen).toEqual([]);
    await paintThenTask();
    expect(seen).toEqual([1, 2]);
  });

  it("keeps going after a callback throws", async () => {
    const report = vi.fn();
    vi.stubGlobal("reportError", report);
    const seen: string[] = [];
    afterPaint(() => {
      throw new Error("boom");
    });
    afterPaint(() => seen.push("next"));
    await paintThenTask();
    vi.unstubAllGlobals();
    expect(seen).toEqual(["next"]);
    expect(report).toHaveBeenCalledWith(new Error("boom"));
  });

  it("yields to the next paint when a drain exceeds its slice", async () => {
    let now = 0;
    vi.spyOn(performance, "now").mockImplementation(() => now);
    const seen: number[] = [];
    afterPaint(() => {
      seen.push(1);
      now += 20; // longer than one slice
    });
    afterPaint(() => seen.push(2));
    await new Promise<void>((done) => requestAnimationFrame(() => setTimeout(done, 0)));
    expect(seen).toEqual([1]);
    await paintThenTask();
    expect(seen).toEqual([1, 2]);
  });

  it("runs in a plain task while the page is hidden", async () => {
    vi.spyOn(document, "hidden", "get").mockReturnValue(true);
    const raf = vi.spyOn(window, "requestAnimationFrame");
    const seen: number[] = [];
    afterPaint(() => seen.push(1));
    await new Promise<void>((done) => setTimeout(done, 0));
    expect(seen).toEqual([1]);
    expect(raf).not.toHaveBeenCalled();
  });
});
