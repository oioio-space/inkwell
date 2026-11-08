import { describe, expect, it } from "vitest";
import {
  authorizeNativeRun,
  exactTrustedInsertion,
  rawEventTiming,
  visibleAndFocused,
} from "./nativeProtocol";

const challenge = "a".repeat(32);
const capability = {
  type: "capability",
  protocol: 1,
  challenge,
  engine: "WebKitGTK",
  native: true,
  runId: "native-test",
  phase: "warm",
  sampleCount: 30,
  refreshHz: 60,
};
describe("native-only authorization", () => {
  it("needs a matching native runtime capability, independently of the build flag", () => {
    expect(authorizeNativeRun({ VITE_PERF_NATIVE: "1" }, challenge)).toBeUndefined();
    expect(authorizeNativeRun(capability, challenge)?.sampleCount).toBe(30);
    for (const altered of [
      { challenge: "b".repeat(32) },
      { native: false },
      { engine: "Chromium" },
      { protocol: 0 },
      { sampleCount: 29 },
      { refreshHz: 0 },
      { refreshHz: Number.NaN },
    ])
      expect(authorizeNativeRun({ ...capability, ...altered }, challenge)).toBeUndefined();
  });
  it("cold means one first input in one of at least thirty fresh process replicates", () => {
    expect(
      authorizeNativeRun(
        { ...capability, phase: "cold", sampleCount: 1, replicateIndex: 1, replicateCount: 30 },
        challenge,
      )?.phase,
    ).toBe("cold");
    expect(
      authorizeNativeRun({ ...capability, phase: "cold", sampleCount: 30 }, challenge),
    ).toBeUndefined();
    expect(
      authorizeNativeRun(
        { ...capability, phase: "cold", sampleCount: 1, replicateIndex: 1, replicateCount: 29 },
        challenge,
      ),
    ).toBeUndefined();
  });
});
describe("raw engine EventTiming boundary", () => {
  it("preserves processing bounds, duration and quantization without deriving paint from a callback", () => {
    const entry = {
      entryType: "event",
      name: "input",
      startTime: 20,
      duration: 24,
      processingStart: 21,
      processingEnd: 24,
      interactionId: 17,
    };
    expect(rawEventTiming(entry)).toEqual({
      name: "input",
      startTime: 20,
      duration: 24,
      processingStart: 21,
      processingEnd: 24,
      interactionId: 17,
    });
    expect(rawEventTiming({ ...entry, duration: 0 })?.duration).toBe(0);
    expect(rawEventTiming({ ...entry, name: "keydown" })).toBeUndefined();
  });
  it("never turns rAF, missing or malformed entries into a numeric paint sample", () => {
    expect(
      rawEventTiming({
        entryType: "raf",
        name: "raf",
        startTime: 20,
        duration: 24,
        processingStart: 21,
        processingEnd: 24,
      }),
    ).toBeUndefined();
    expect(rawEventTiming(undefined)).toBeUndefined();
    expect(
      rawEventTiming({
        entryType: "event",
        name: "input",
        startTime: 20,
        duration: NaN,
        processingStart: 21,
        processingEnd: 24,
      }),
    ).toBeUndefined();
  });
});
it("requires the real single-character insertion from the trusted input path", () => {
  expect(exactTrustedInsertion("x", [{ from: 7, to: 7, text: "x" }])).toBe(true);
  expect(exactTrustedInsertion("x", [{ from: 7, to: 8, text: "x" }])).toBe(false);
  expect(exactTrustedInsertion("x", [{ from: 7, to: 7, text: "fake" }])).toBe(false);
  expect(exactTrustedInsertion("x", [])).toBe(false);
});
it("a hidden or unfocused native view cannot pass", () => {
  expect(visibleAndFocused("visible", true)).toBe(true);
  expect(visibleAndFocused("hidden", true)).toBe(false);
  expect(visibleAndFocused("visible", false)).toBe(false);
});
