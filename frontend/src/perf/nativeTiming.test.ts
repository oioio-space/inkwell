import { afterEach, expect, it, vi } from "vitest";
import {
  BrowserClock,
  FOCUS_INTERRUPTED,
  FRAME_UNAVAILABLE,
  NativeClock,
  observeClock,
} from "./nativeTiming";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

it("compares eight RPC reads, two outer RPC reads and the zero-RPC bounded clock under a deterministic transport model", () => {
  let time = 100;
  let rpc = 0;
  vi.spyOn(performance, "now").mockImplementation(() => (time += 0.001));
  vi.stubGlobal(
    "XMLHttpRequest",
    class {
      status = 200;
      responseText = "";
      open() {}
      send() {
        rpc++;
        time += 2;
        this.responseText = JSON.stringify({ ms: time, resolutionMs: 0.000001 });
      }
    },
  );
  const native = new NativeClock("mock-cost");
  const browser = new BrowserClock();
  const elapsed = (read: () => number, count: number) => {
    const start = time;
    for (let i = 0; i < count; i++) read();
    return time - start;
  };
  expect(elapsed(() => native.read(), 8)).toBeCloseTo(16, 6);
  expect(elapsed(() => native.read(), 2)).toBeCloseTo(4, 6);
  expect(elapsed(() => browser.read(), 2)).toBeCloseTo(0.002, 6);
  expect(rpc).toBe(10);
  const observed = browser.observe(1);
  expect(observed.source).toBe("browser-bounded");
  expect(observed.batchSize).toBe(1000);
  expect(observed.readSpansMs).toHaveLength(4);
  expect(observed.readSpansMs.every((n) => n > 0.001 && n < 0.003)).toBe(true);
  expect(rpc).toBe(10); // No transport to resolve after the frame either.
  vi.restoreAllMocks();
});

it("refuses to certify a frozen clock or more than 1ms of timing uncertainty", () => {
  for (const quantum of [0, -1, Number.NaN, 2])
    expect(() => new BrowserClock().observe(quantum)).toThrow(/clock uncertainty/);
});

it("reads the fine native clock synchronously with a unique cache path for every boundary", () => {
  const paths: string[] = [];
  let time = 1;
  vi.stubGlobal(
    "XMLHttpRequest",
    class {
      status = 200;
      responseText = "";
      open(method: string, path: string, asynchronous: boolean) {
        expect(method).toBe("GET");
        expect(asynchronous).toBe(false);
        paths.push(path);
      }
      send() {
        time += 0.000001;
        this.responseText = JSON.stringify({ ms: time, resolutionMs: 0.000001 });
      }
    },
  );
  const clock = new NativeClock("capability-nonce");
  expect(clock.observe().resolutionMs).toBe(0.000001);
  expect(clock.read()).toBeGreaterThan(1);
  expect(new Set(paths).size).toBe(5);
  expect(paths.every((path) => path.startsWith("/__native_perf_clock/capability-nonce/"))).toBe(
    true,
  );
});

it("refuses a coarse, stale, unavailable or nonmonotonic native clock", () => {
  for (const defect of ["coarse", "stale", "unavailable", "nonmonotonic"]) {
    let count = 0;
    vi.stubGlobal(
      "XMLHttpRequest",
      class {
        status = defect === "unavailable" ? 500 : 200;
        responseText = "";
        open() {}
        send() {
          this.responseText = JSON.stringify({
            ms: defect === "stale" ? 1 : defect === "nonmonotonic" ? 2 - count++ : count++,
            resolutionMs: defect === "coarse" ? 1 : 0.000001,
          });
        }
      },
    );
    const clock = new NativeClock("test");
    expect(() => {
      clock.read();
      clock.read();
    }).toThrow(/clock/);
  }
});

it("reports observed millisecond coarsening and an explicit conservative rounding rule", () => {
  let reads = 0;
  const clock = observeClock(() => Math.floor(reads++ / 4));
  expect(clock.resolutionMs).toBe(1);
  expect(clock.deltasMs.length).toBeGreaterThanOrEqual(3);
  expect(clock.roundingRule).toBe("upper:+quantum-per-disjoint-interval");
});
it("does not invent precision for a frozen or unavailable clock", () => {
  expect(observeClock(() => 0).resolutionMs).toBe(0);
});
it("explains the display and GNOME permission interruptions separately", () => {
  expect(FRAME_UNAVAILABLE).toContain("écran éteint ou verrouillé");
  expect(FOCUS_INTERRUPTED).toContain("GNOME");
  expect(FOCUS_INTERRUPTED).toContain("XTest");
});
