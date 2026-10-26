// @vitest-environment node
import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import { CHUNK_CHARS, chunksOf, MAX_GROWN_CHARS } from "../store/limits";
import { lastChangeSeconds } from "./docTime";

describe("lastChangeSeconds", () => {
  it("PRV-77 is the time recorded by the change at the heads, not this machine's clock", () => {
    let doc = A.from({ text: "" });
    doc = A.change(doc, { time: 1_700_000_000 }, (d) => A.updateText(d, ["text"], "a"));
    expect(lastChangeSeconds(doc)).toBe(1_700_000_000);
    const copy = A.load<{ text: string }>(A.save(doc));
    expect(lastChangeSeconds(copy)).toBe(1_700_000_000); // another peer holding the same heads
  });

  it("PRV-78 a text past what a document grows to is not decoded (Automerge panics on its change): 0", () => {
    let doc = A.from({ text: "" });
    // Built in pieces, as the app does: a single change this big would cost gigabytes here.
    for (const [i, piece] of chunksOf("a".repeat(MAX_GROWN_CHARS + 1)).entries())
      doc = A.change(doc, { time: 1_700_000_000 }, (d) =>
        A.splice(d, ["text"], i * CHUNK_CHARS, 0, piece),
      );
    expect(lastChangeSeconds(doc)).toBe(0);
  }, 60_000);

  it("PRV-77 concurrent heads give the latest of them", () => {
    let a = A.from({ text: "x" });
    let b = A.clone(a);
    a = A.change(a, { time: 1_700_000_100 }, (d) => A.updateText(d, ["text"], "xa"));
    b = A.change(b, { time: 1_700_000_900 }, (d) => A.updateText(d, ["text"], "xb"));
    expect(lastChangeSeconds(A.merge(a, b))).toBe(1_700_000_900);
  });
});
