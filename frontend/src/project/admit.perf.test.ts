// @vitest-environment node
import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import { IndexValidator } from "./admit";
import type { IndexDoc } from "./model";
import * as ops from "./ops";
import { apply, bob, ctx, type Index, idOf, newProject, textUrl, view } from "./testkit";

/**
 * Admission runs on the UI thread, for every change that arrives. Its cost must not grow with
 * the length of the history (the main file's text is in the index document, so a long editing
 * life is a long history): each change is applied once to one working copy and judged on what it
 * did (its patches) and on the few nodes it touches; a pass over the history is paid only for
 * what a rule cannot settle otherwise (a move, a deletion), never for a rename or a creation.
 *
 * Budget: 1 000 file operations made concurrently by 10 people on a document with 100 000
 * typing changes are admitted in under 3 s: 3 ms each, a fifth of a 16 ms frame. It runs in
 * about 1 s on the development machine; the margin is for a slower one. The version that
 * rebuilt the history for each change (a clone of a view) took 1.2 s per change at this size,
 * 20 minutes for the same thousand.
 */
const TYPING = 100_000;
const BRANCHES = 10;
const PER_BRANCH = 100;
const BUDGET_MS = 3_000;

function typed(): Index {
  let doc = newProject();
  doc = apply(
    doc,
    ops.create(
      doc,
      view(doc),
      ctx(bob),
      Array.from({ length: BRANCHES * PER_BRANCH }, (_, i) => ({
        type: "text" as const,
        name: `f${i}.typ`,
        parent: null,
        url: textUrl(),
      })),
    ),
  );
  for (let i = 0; i < TYPING; i++) {
    doc = A.change(doc, (d: IndexDoc) => {
      A.splice(d as never, ["text"], i, 0, "x");
    });
  }
  return doc;
}

describe("PRJ-19 admission does not slow down with the length of the history", () => {
  it("1 000 concurrent file operations on 100 000 typing changes", () => {
    const base = typed();
    const changes: Uint8Array[] = [];
    for (let b = 0; b < BRANCHES; b++) {
      // Each person works on their own files, and makes a folder now and then.
      let doc = A.clone(base);
      for (let i = 0; i < PER_BRANCH; i++) {
        const m = view(doc);
        doc =
          i % 10 === 0
            ? apply(
                doc,
                ops.create(doc, m, ctx(bob), [{ type: "dir", name: `d${b}-${i}`, parent: null }]),
              )
            : apply(
                doc,
                ops.rename(m, ctx(bob), idOf(base, `f${b * PER_BRANCH + i}.typ`), `n${b}-${i}.typ`),
              );
      }
      changes.push(...A.getChanges(base, doc));
    }
    expect(changes).toHaveLength(BRANCHES * PER_BRANCH);

    const validator = new IndexValidator(() => base);
    validator.heads(); // the shadow is loaded: not part of what a change costs
    const start = performance.now();
    // The branches interleaved, as a network would deliver them.
    for (let i = 0; i < PER_BRANCH; i++) {
      for (let b = 0; b < BRANCHES; b++) {
        expect(
          validator.check(changes[b * PER_BRANCH + i] as Uint8Array, () => false),
        ).toBeUndefined();
      }
    }
    const took = performance.now() - start;
    console.info(`admitted ${changes.length} changes in ${Math.round(took)} ms`);
    expect(took).toBeLessThan(BUDGET_MS);
  }, 600_000);
});
