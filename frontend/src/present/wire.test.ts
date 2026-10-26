import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { MAX_EPOCH, parseWire } from "./wire";

describe("presentation wire messages", () => {
  it("PRS-62 reads every message the pilot can send", () => {
    const ok = [
      { t: "state", epoch: 2, seq: 9, slide: 3, step: 1, screen: "black" },
      { t: "laser", epoch: 2, x: 0.25, y: 1 },
      { t: "laser", epoch: 2, x: null, y: null },
      { t: "take", epoch: 3 },
      { t: "ask" },
      { t: "hello" },
      { t: "give", to: "12D3KooWabc", epoch: 4 },
      { t: "deny", to: "12D3KooWabc" },
      { t: "end", epoch: 4 },
    ];
    for (const m of ok) expect(parseWire(m)).toEqual(m);
  });

  it("PRS-62 refuses what is not exactly a message", () => {
    const bad: unknown[] = [
      null,
      "state",
      [],
      {},
      { t: "nope" },
      { t: "state", epoch: -1, seq: 0, slide: 1, step: 0, screen: "show" },
      { t: "state", epoch: 1, seq: 0, slide: 0, step: 0, screen: "show" },
      { t: "state", epoch: 1, seq: 0, slide: 1.5, step: 0, screen: "show" },
      { t: "state", epoch: 1, seq: 0, slide: 1, step: 0, screen: "red" },
      { t: "state", epoch: MAX_EPOCH + 1, seq: 0, slide: 1, step: 0, screen: "show" },
      { t: "laser", epoch: 1, x: 2, y: 0 },
      { t: "laser", epoch: 1, x: Number.NaN, y: 0 },
      { t: "laser", epoch: 1, x: null, y: 0.5 },
      { t: "give", to: "", epoch: 1 },
      { t: "give", to: "a b", epoch: 1 },
      { t: "give", to: "x".repeat(65), epoch: 1 },
      { t: "deny", to: 7 },
    ];
    for (const m of bad) expect(parseWire(m), JSON.stringify(m)).toBeUndefined();
  });

  it("PRS-62 never throws and never returns something out of bounds, whatever arrives", () => {
    fc.assert(
      fc.property(fc.anything(), (v) => {
        const m = parseWire(v);
        if (!m) return true;
        if ("epoch" in m && !(m.epoch >= 0 && m.epoch <= MAX_EPOCH)) return false;
        if (m.t === "state" && !(m.slide >= 1 && m.step >= 0)) return false;
        if (m.t === "laser" && m.x !== null && !(m.x >= 0 && m.x <= 1)) return false;
        return true;
      }),
      { numRuns: 500 },
    );
  });
});
