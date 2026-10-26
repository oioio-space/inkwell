import { describe, expect, it } from "vitest";
import { formatElapsed, pace, Stopwatch } from "./clock";

describe("Stopwatch", () => {
  it("PRS-06 starts at the first advance only, pauses, and resets", () => {
    let now = 1000;
    const w = new Stopwatch(() => now);
    expect(w.running).toBe(false);
    expect(w.elapsedMs()).toBe(0);
    w.startOnce();
    now += 5000;
    expect(w.elapsedMs()).toBe(5000);
    w.pause();
    now += 9000;
    expect(w.elapsedMs()).toBe(5000);
    w.startOnce(); // a later advance does not restart a paused timer
    expect(w.running).toBe(false);
    w.start();
    now += 1000;
    expect(w.elapsedMs()).toBe(6000);
    w.reset();
    expect(w.elapsedMs()).toBe(0);
    expect(w.started).toBe(false);
    w.startOnce();
    expect(w.running).toBe(true);
  });
});

describe("pace", () => {
  const min = 60_000;
  it("PRS-06 says ahead, behind or on time against the plan", () => {
    // 20 minutes over 10 slides: 2 minutes per slide; at slide 6 the plan says 10 minutes spent.
    expect(pace(10 * min, 20, 6, 10)).toEqual({ kind: "onTime" });
    expect(pace(10 * min + 29_000, 20, 6, 10)).toEqual({ kind: "onTime" });
    expect(pace(13 * min, 20, 6, 10)).toEqual({ kind: "behind", minutes: 3 });
    expect(pace(7 * min, 20, 6, 10)).toEqual({ kind: "ahead", minutes: 3 });
    expect(pace(10 * min + 40_000, 20, 6, 10)).toEqual({ kind: "behind", minutes: 1 });
  });
  it("has nothing to say without a target", () => {
    expect(pace(5 * min, 0, 3, 10)).toBeUndefined();
    expect(pace(5 * min, 20, 0, 10)).toBeUndefined();
    expect(pace(5 * min, 20, 1, 0)).toBeUndefined();
  });
});

describe("formatElapsed", () => {
  it("writes minutes and seconds, and hours when there are some", () => {
    expect(formatElapsed(0)).toBe("0:00");
    expect(formatElapsed(83_000)).toBe("1:23");
    expect(formatElapsed(3_723_000)).toBe("1:02:03");
    expect(formatElapsed(-5)).toBe("0:00");
  });
});
