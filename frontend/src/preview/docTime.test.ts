// @vitest-environment node
import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import { MAX_EDITABLE_CHARS } from "../store/limits";
import { lastChangeSeconds } from "./docTime";

describe("lastChangeSeconds", () => {
  it("PRV-77 is the time recorded by the change at the heads, not this machine's clock", () => {
    let doc = A.from({ text: "" });
    doc = A.change(doc, { time: 1_700_000_000 }, (d) => A.updateText(d, ["text"], "a"));
    expect(lastChangeSeconds(doc)).toBe(1_700_000_000);
    const copy = A.load<{ text: string }>(A.save(doc));
    expect(lastChangeSeconds(copy)).toBe(1_700_000_000); // another peer holding the same heads
  });

  it("PRV-78 a text too big to edit is not decoded (Automerge panics on its change): 0", () => {
    let doc = A.from({ text: "" });
    doc = A.change(doc, { time: 1_700_000_000 }, (d) =>
      A.splice(d, ["text"], 0, 0, "a".repeat(MAX_EDITABLE_CHARS + 1)),
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
