import { next as A } from "@automerge/automerge";
import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { patchWindow } from "./patchWindow";
import { RowIndex } from "./rowIndex";

describe("patch window", () => {
  it("the window of any sequence of splices lets the index update match a fresh scan (property)", () => {
    const piece = fc.constantFrom("a", ",", '"', "\n", "bc", ",,", "\r\n");
    const text = fc.array(piece, { maxLength: 30 }).map((p) => p.join(""));
    const op = fc.tuple(fc.nat(50), fc.nat(6), text);
    fc.assert(
      fc.property(text, fc.array(op, { minLength: 1, maxLength: 5 }), (start, ops) => {
        const before = A.from({ text: start });
        let patches: A.Patch[] = [];
        const after = A.change(
          before,
          {
            patchCallback: (p) => {
              patches = p;
            },
          },
          (d) => {
            for (const [at, len, ins] of ops) {
              const cur = (d as { text: string }).text;
              const pos = Math.min(at, cur.length);
              A.splice(d, ["text"], pos, Math.min(len, cur.length - pos), ins);
            }
          },
        );
        const w = patchWindow(patches);
        const idx = RowIndex.scan(start, ",");
        if (!w) {
          expect(after.text).toBe(start);
          return;
        }
        idx.update(after.text, w.from, w.to, w.insLen);
        const fresh = RowIndex.scan(after.text, ",");
        expect(Array.from(idx.starts.subarray(0, idx.count))).toEqual(
          Array.from(fresh.starts.subarray(0, fresh.count)),
        );
      }),
      { numRuns: 300 },
    );
  });
});
