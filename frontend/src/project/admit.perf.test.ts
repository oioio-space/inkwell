// @vitest-environment node
import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import { Coverage, type DocValidator } from "../net/coverage";
import { IndexValidator } from "./admit";
import { type IndexDoc, imm } from "./model";
import * as ops from "./ops";
import { apply, bob, ctx, type Index, idOf, newProject, textUrl, view } from "./testkit";

/**
 * Admission runs on the UI thread, for every change that arrives. Its cost must not grow with
 * the length of the history (the main file's text is in the index document, so a long editing
 * life is a long history), and a hostile sender must not be able to spend the thread.
 *
 * A change is judged from its operations, and from the index at its own heads only for the few
 * rules that need it (a move, a deletion), within a small budget; it is applied to the working
 * copy only once admitted. Budgets, on a document with 100 000 typing changes:
 *
 * - 1 000 file operations made concurrently by 10 people: under 3 s, 3 ms each, a fifth of a
 *   16 ms frame (about 1 s here; the margin is for a slower machine). Rebuilding the history
 *   for each change took 1.2 s per change, 20 minutes for the same thousand.
 * - a hostile message of 5 000 crafted moves, each of which would need the history, then 200
 *   attestation and role events: under 2 s in all (0.7 s here). The first judgements take the budget (8, each
 *   about 12 ms), the rest wait, a few hundred of them held and the others dropped; the events
 *   re-judge nothing, as nothing they report is what the held changes wait for.
 * - 100 honest changes after 1 000 refused ones: under 0.5 s: the refused changes are not in
 *   the working copy, so the honest ones are still judged on it directly.
 */
const TYPING = 100_000;
const BRANCHES = 10;
const PER_BRANCH = 100;
const BUDGET_MS = 3_000;
const HOSTILE_MS = 2_000;

/** The project as people left it before the typing: 1 000 files and a folder. */
const small = (() => {
  let doc = newProject();
  doc = apply(
    doc,
    ops.create(doc, view(doc), ctx(bob), [
      { type: "dir", name: "d", parent: null },
      ...Array.from({ length: BRANCHES * PER_BRANCH }, (_, i) => ({
        type: "text" as const,
        name: `f${i}.typ`,
        parent: null,
        url: textUrl(),
      })),
    ]),
  );
  return doc;
})();

let cached: Index | undefined;
/** `small`, then 100 000 typing changes on top (built once). */
function typed(): Index {
  if (cached) return cached;
  let doc = A.clone(small);
  for (let i = 0; i < TYPING; i++) {
    doc = A.change(doc, (d: IndexDoc) => {
      A.splice(d as never, ["text"], i, 0, "x");
    });
  }
  cached = doc;
  return doc;
}

describe("PRJ-19 admission does not slow down with the length of the history", () => {
  it("1 000 concurrent file operations on 100 000 typing changes", () => {
    const base = typed();
    const changes: Uint8Array[] = [];
    for (let b = 0; b < BRANCHES; b++) {
      // Each person works on their own files, and makes a folder now and then.
      let doc = A.clone(small);
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
                ops.rename(
                  m,
                  ctx(bob),
                  idOf(small, `f${b * PER_BRANCH + i}.typ`),
                  `n${b}-${i}.typ`,
                ),
              );
      }
      changes.push(...A.getChanges(small, doc));
    }
    expect(changes).toHaveLength(BRANCHES * PER_BRANCH);

    const validator = new IndexValidator(() => base);
    validator.heads(); // the working copy is loaded: not part of what a change costs
    const start = performance.now();
    // The branches interleaved, as a network would deliver them.
    for (let i = 0; i < PER_BRANCH; i++) {
      for (let b = 0; b < BRANCHES; b++) {
        expect(validator.check(changes[b * PER_BRANCH + i] as Uint8Array, () => false)).toBe(
          undefined,
        );
      }
    }
    const took = performance.now() - start;
    console.info(`admitted ${changes.length} changes in ${Math.round(took)} ms`);
    expect(took).toBeLessThan(BUDGET_MS);
  }, 600_000);

  it("a hostile message of 5 000 crafted moves, then 200 attestation and role events", () => {
    const base = typed();
    const dir = idOf(small, "d");
    const message: Uint8Array[] = [];
    for (let i = 0; i < 5000; i++) {
      const id = idOf(small, `f${i % (BRANCHES * PER_BRANCH)}.typ`);
      const doc = A.change(A.clone(small), (d: IndexDoc) => {
        const nodes = d.nodes as unknown as Record<string, Record<string, unknown>>;
        (nodes[id] as Record<string, unknown>).parent = imm(i % 2 === 0 ? dir : id);
      });
      message.push(...A.getChanges(small, doc));
    }
    let checks = 0;
    const inner = new IndexValidator(() => base, { now: () => 0 }); // no budget comes back
    const validator: DocValidator = {
      check(bytes, admin) {
        checks++;
        return inner.check(bytes, admin);
      },
      checkDocument: (doc, admin) => inner.checkDocument(doc as never, admin),
      resolved: (h) => inner.resolved(h),
      hasBudget: () => inner.hasBudget(),
    };
    inner.heads(); // loaded
    const cov = new Coverage();
    cov.setValidator("doc", validator);
    const hashes = message.map((c) => A.decodeChange(c).hash);
    for (let i = 0; i < hashes.length; i += 200)
      cov.attested("bob", "doc", hashes.slice(i, i + 200));
    const sync = A.encodeSyncMessage({
      heads: hashes.slice(-1),
      need: [],
      have: [],
      changes: message,
    });

    const start = performance.now();
    cov.filter("doc", "bob", sync);
    const afterMessage = checks;
    for (let i = 0; i < 200; i++) {
      cov.attested("alice", "doc", [hashes[i] as string]);
      if (i % 20 === 0) cov.setAdmins(["alice"]);
      cov.releaseHeld();
    }
    const took = performance.now() - start;
    console.info(`hostile message and events in ${Math.round(took)} ms, ${checks} checks`);
    expect(took).toBeLessThan(HOSTILE_MS);
    expect(cov.heldChanges).toBeLessThanOrEqual(256); // per sender, in changes
    expect(checks).toBe(afterMessage); // no event judged anything again
  }, 600_000);

  it("1 000 refused changes leave nothing behind that slows the honest ones down", () => {
    const base = typed();
    // A thousand crafted renames, each refused (a name no client would write).
    const crafted: Uint8Array[] = [];
    for (let i = 0; i < 1000; i++) {
      const id = idOf(small, `f${i}.typ`);
      const doc = A.change(A.clone(small), (d: IndexDoc) => {
        const nodes = d.nodes as unknown as Record<string, Record<string, unknown>>;
        (nodes[id] as Record<string, unknown>).name = imm("../etc");
      });
      crafted.push(...A.getChanges(small, doc));
    }
    // A hundred honest ones, each on the one before, on the current heads of the document.
    let doc = A.clone(base);
    const before = A.getHeads(doc);
    for (let i = 0; i < 100; i++) {
      doc = apply(doc, ops.rename(view(doc), ctx(bob), idOf(small, `f${i}.typ`), `honest${i}.typ`));
    }
    const honest = A.getChanges(A.view(base, before) as Index, doc);

    const validator = new IndexValidator(() => base);
    validator.heads();
    for (const c of crafted) expect(validator.check(c, () => false)).toBe("invalid name");
    expect(validator.heads()).toEqual([...A.getHeads(base)].sort()); // none got into the copy

    const start = performance.now();
    for (const c of honest) expect(validator.check(c, () => false)).toBeUndefined();
    const took = performance.now() - start;
    console.info(`100 honest changes after 1000 refused ones in ${Math.round(took)} ms`);
    expect(took).toBeLessThan(500);
  }, 600_000);
});
