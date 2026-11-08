import { next as A } from "@automerge/automerge";
import { Text } from "@codemirror/state";
import fc from "fast-check";
import { describe, expect, it } from "vitest";
import {
  anchorRange,
  docPosition,
  incrementalChange,
  resolveRange,
  toOffset,
  toPosition,
} from "./positions";

describe("UTF-16 ↔ Automerge positions", () => {
  it("roundtrips Unicode UTF-16 positions and resolves real Automerge cursors", () => {
    fc.assert(
      fc.property(
        fc.array(fc.constantFrom("a", "é", "😀", "\n", "𝄞", "中"), { maxLength: 80 }),
        (parts) => {
          const text = parts.join("");
          const doc = A.from({ text });
          let offset = 0;
          for (const part of parts) {
            expect(toOffset(text, toPosition(text, offset))).toBe(offset);
            const range = anchorRange(doc, offset, offset);
            expect(resolveRange(doc, range)).toEqual({ from: offset, to: offset });
            offset += part.length;
          }
        },
      ),
      { numRuns: 200 },
    );
  });
  it("anchors follow concurrent insertion before an error", () => {
    let doc = A.from({ text: "x😀bad" });
    const range = anchorRange(doc, 3, 6);
    const peer = A.change(A.clone(doc), (d) => A.splice(d, ["text"], 0, 0, "hello"));
    doc = A.merge(doc, peer);
    expect(resolveRange(doc, range)).toEqual({ from: 8, to: 11 });
  });
});

it("incremental edits reconstruct arbitrary Unicode text without splitting surrogate pairs", () => {
  const text = fc
    .array(fc.constantFrom("a", "😀", "𝄞", "é", "\n"), { maxLength: 100 })
    .map((parts) => parts.join(""));
  fc.assert(
    fc.property(text, text, (old, next) => {
      const edit = incrementalChange(old, next);
      const from = toOffset(old, edit.range.start),
        to = toOffset(old, edit.range.end);
      expect(old.slice(0, from) + edit.text + old.slice(to)).toBe(next);
      for (const offset of [from, to])
        expect(
          offset > 0 &&
            offset < old.length &&
            /[\uD800-\uDBFF]/.test(old[offset - 1] ?? "") &&
            /[\uDC00-\uDFFF]/.test(old[offset] ?? ""),
        ).toBe(false);
    }),
    { numRuns: 300 },
  );
});

describe("docPosition", () => {
  it("matches toPosition on the flattened document without flattening it", () => {
    fc.assert(
      fc.property(
        fc.string({ unit: fc.constantFrom("a", "é", "\n", "😀", " ") }),
        fc.integer({ min: -2, max: 40 }),
        (text, offset) => {
          expect(docPosition(Text.of(text.split("\n")), offset)).toEqual(toPosition(text, offset));
        },
      ),
    );
  });
});
