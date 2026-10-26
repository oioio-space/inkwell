import { describe, expect, it } from "vitest";
import {
  CHUNK_CHARS,
  chunksOf,
  growthRefused,
  MAX_EDITABLE_CHARS,
  MAX_GROWN_CHARS,
  SLOW_CHARS,
  sizeTier,
  tooBigToEdit,
  tooBigToGrow,
} from "./limits";

const Mi = 1024 * 1024;

describe("PERF-02 PERF-03 the one size gate (ADR 0028 D1)", () => {
  it("what enters a document stops at 2 Mi characters, what a document grows to at 4 Mi", () => {
    expect(MAX_EDITABLE_CHARS).toBe(2 * Mi);
    expect(MAX_GROWN_CHARS).toBe(4 * Mi);
    expect(tooBigToEdit(MAX_EDITABLE_CHARS)).toBe(false);
    expect(tooBigToEdit(MAX_EDITABLE_CHARS + 1)).toBe(true);
    expect(tooBigToGrow(MAX_GROWN_CHARS)).toBe(false);
    expect(tooBigToGrow(MAX_GROWN_CHARS + 1)).toBe(true);
  });

  it("a text refuses to grow past 4 Mi, but shrinking or staying put is always allowed", () => {
    expect(growthRefused(MAX_GROWN_CHARS - 10, MAX_GROWN_CHARS + 1)).toBe(true);
    expect(growthRefused(MAX_GROWN_CHARS + 50, MAX_GROWN_CHARS + 20)).toBe(false);
    expect(growthRefused(MAX_GROWN_CHARS + 50, MAX_GROWN_CHARS + 50)).toBe(false);
    expect(growthRefused(0, 100)).toBe(false);
  });

  it("tiers: fluid up to 512 Ki, slow notice up to 2 Mi (editable all along), then read-only", () => {
    expect(SLOW_CHARS).toBe(512 * 1024);
    expect(sizeTier(0)).toBe("fluid");
    expect(sizeTier(SLOW_CHARS)).toBe("fluid");
    expect(sizeTier(SLOW_CHARS + 1)).toBe("large");
    expect(sizeTier(MAX_EDITABLE_CHARS)).toBe("large");
    expect(sizeTier(MAX_EDITABLE_CHARS + 1)).toBe("readonly");
  });
});

describe("PERF-04 chunks", () => {
  it("cuts a text into pieces of at most 64 Ki that join back to it", () => {
    const text = "x".repeat(CHUNK_CHARS * 2 + 5);
    const parts = chunksOf(text);
    expect(parts.map((p) => p.length)).toEqual([CHUNK_CHARS, CHUNK_CHARS, 5]);
    expect(parts.join("")).toBe(text);
    expect(chunksOf("")).toEqual([]);
    expect(chunksOf("abc")).toEqual(["abc"]);
  });

  it("never cuts between the two halves of a character outside the BMP", () => {
    const text = `${"x".repeat(CHUNK_CHARS - 1)}😀${"y".repeat(10)}`;
    const parts = chunksOf(text);
    expect(parts.join("")).toBe(text);
    for (const p of parts) {
      expect(p.length).toBeLessThanOrEqual(CHUNK_CHARS);
      const last = p.charCodeAt(p.length - 1);
      expect(last >= 0xd800 && last <= 0xdbff).toBe(false);
    }
  });
});
