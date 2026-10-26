// @vitest-environment node
import { next as A } from "@automerge/automerge";
import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { HOLD, IndexValidator } from "./admit";
import * as ops from "./ops";
import { alice, apply, bob, ctx, type Index, newProject, textUrl, view } from "./testkit";

/**
 * The verdict on a change depends on the change and on its own history, never on what the
 * receiver happens to have received before (security audit, HIGH): two replicas that receive
 * the same changes in any order end with the same index.
 */

/** The changes `doc` has beyond `base`, oldest first. */
function since(base: Index, doc: Index): Uint8Array[] {
  return A.getChanges(base, doc);
}

/**
 * What a replica holding `base` ends with when the changes arrive in `order`: what it admitted,
 * what it refused and what still waits. A change that waits is offered again after the others.
 */
function deliver(
  base: Index,
  changes: readonly Uint8Array[],
  admin: ReadonlySet<string>,
  order: readonly number[],
): { admitted: string[]; refused: string[]; held: string[] } {
  const validator = new IndexValidator(() => base);
  let pending = order.map((i) => changes[i] as Uint8Array);
  const admitted: string[] = [];
  const refused: string[] = [];
  for (let progress = true; progress && pending.length > 0; ) {
    progress = false;
    const rest: Uint8Array[] = [];
    for (const bytes of pending) {
      const why = validator.check(bytes, (h) => admin.has(h));
      const hash = A.decodeChange(bytes).hash;
      if (why === undefined) {
        progress = true;
        admitted.push(hash);
      } else if (why.startsWith(HOLD)) rest.push(bytes);
      else {
        progress = true;
        refused.push(hash);
      }
    }
    pending = rest;
  }
  return {
    admitted: admitted.sort(),
    refused: refused.sort(),
    held: pending.map((b) => A.decodeChange(b).hash).sort(),
  };
}

const permutations = <T>(xs: readonly T[]): T[][] =>
  xs.length <= 1
    ? [[...xs]]
    : xs.flatMap((x, i) =>
        permutations([...xs.slice(0, i), ...xs.slice(i + 1)]).map((p) => [x, ...p]),
      );

function twoBranches(
  base: Index,
  a: (d: Index) => Index,
  b: (d: Index) => Index,
): { all: Uint8Array[]; admin: Set<string> } {
  const left = a(A.clone(base));
  const right = b(A.clone(base));
  const lc = since(base, left);
  const rc = since(base, right);
  return {
    all: [...lc, ...rc],
    admin: new Set(lc.map((c) => A.decodeChange(c).hash)),
  };
}

describe("PRJ-17 the verdict depends only on the change's own history", () => {
  it("an admin switches to approval while a writer trashes a file: every order converges", () => {
    let base = newProject();
    base = apply(
      base,
      ops.create(base, view(base), ctx(bob), [
        { type: "text", name: "ch1.typ", parent: null, url: textUrl() },
      ]),
    );
    base = apply(base, ops.setDeletionMode(base, ctx(alice), "free"));
    const t = view(base).byPath.get("ch1.typ")?.id as string;
    const { all, admin } = twoBranches(
      base,
      (d) => apply(d, ops.setDeletionMode(d, ctx(alice), "approval")),
      (d) => apply(d, ops.remove(d, view(d), ctx(bob), [t])),
    );
    const outcomes = permutations([0, 1]).map((order) => deliver(base, all, admin, order));
    expect(outcomes[0]?.refused).toEqual([]); // the writer acted under the rule he knew
    expect(new Set(outcomes.map((o) => JSON.stringify(o))).size).toBe(1);
  });

  it("the same, with the writer's change arriving after a later change of the admin", () => {
    let base = newProject();
    base = apply(
      base,
      ops.create(base, view(base), ctx(bob), [
        { type: "text", name: "ch1.typ", parent: null, url: textUrl() },
      ]),
    );
    base = apply(base, ops.setDeletionMode(base, ctx(alice), "free"));
    const t = view(base).byPath.get("ch1.typ")?.id as string;
    const { all, admin } = twoBranches(
      base,
      (d) => {
        d = apply(d, ops.setDeletionMode(d, ctx(alice), "approval"));
        return apply(
          d,
          ops.create(d, view(d), ctx(alice), [{ type: "dir", name: "docs", parent: null }]),
        );
      },
      (d) => apply(d, ops.remove(d, view(d), ctx(bob), [t])),
    );
    const heads = new Set(
      permutations([0, 1, 2]).map((order) => JSON.stringify(deliver(base, all, admin, order))),
    );
    expect(heads.size).toBe(1);
  });

  it("property: two concurrent branches, any delivery order, one result", () => {
    const safe = (op: (d: Index, n: number) => Index) => (d: Index, n: number) => {
      try {
        return op(d, n);
      } catch {
        return d; // an operation the client itself refuses is not one to judge
      }
    };
    const adminOp = (d: Index, n: number): Index => {
      const m = view(d);
      const files = m.entries.filter((e) => e.type !== "dir" && e.id !== m.mainId);
      const file = files[n % Math.max(files.length, 1)];
      switch (n % 5) {
        case 0:
          return apply(d, ops.setDeletionMode(d, ctx(alice), n % 2 === 0 ? "free" : "approval"));
        case 1:
          return apply(
            d,
            ops.create(d, m, ctx(alice), [{ type: "dir", name: `d${n}`, parent: null }]),
          );
        case 2:
          return file ? apply(d, ops.rename(m, ctx(alice), file.id, `r${n}.typ`)) : d;
        case 3:
          return file ? apply(d, ops.remove(d, m, ctx(alice), [file.id])) : d;
        default:
          return file ? apply(d, ops.move(m, ctx(alice), [file.id], null)) : d;
      }
    };
    const writerOp = (d: Index, n: number): Index => {
      const m = view(d);
      const files = m.entries.filter((e) => e.type !== "dir" && e.id !== m.mainId);
      const file = files[n % Math.max(files.length, 1)];
      switch (n % 4) {
        case 0:
          return apply(
            d,
            ops.create(d, m, ctx(bob), [
              { type: "text", name: `w${n}.typ`, parent: null, url: textUrl() },
            ]),
          );
        case 1:
          return file ? apply(d, ops.rename(m, ctx(bob), file.id, `b${n}.typ`)) : d;
        case 2:
          return file ? apply(d, ops.remove(d, m, ctx(bob), [file.id])) : d;
        default:
          return file ? apply(d, ops.setDeletionMode(d, ctx(bob), "free")) : d;
      }
    };
    fc.assert(
      fc.property(
        fc.array(fc.constantFrom(5, 15, 25, 0, 10, 1, 2, 3, 4), { minLength: 1, maxLength: 3 }),
        fc.array(fc.constantFrom(2, 6, 10, 0, 1, 3), { minLength: 1, maxLength: 3 }),
        fc.nat(5),
        fc.array(fc.nat(), { minLength: 12, maxLength: 12 }),
        (as, bs, deletion, seeds) => {
          let base = newProject();
          base = apply(
            base,
            ops.create(base, view(base), ctx(bob), [
              { type: "text", name: "ch1.typ", parent: null, url: textUrl() },
              { type: "text", name: "ch2.typ", parent: null, url: textUrl() },
            ]),
          );
          if (deletion % 2 === 0) base = apply(base, ops.setDeletionMode(base, ctx(alice), "free"));
          const { all, admin } = twoBranches(
            base,
            (d) => as.reduce(safe(adminOp), d),
            (d) => bs.reduce(safe(writerOp), d),
          );
          const causal = deliver(
            base,
            all,
            admin,
            all.map((_, i) => i),
          );
          // a few shuffles of the delivery order, from the seeds
          for (let k = 0; k < 3; k++) {
            const order = all.map((_, i) => i);
            for (let i = order.length - 1; i > 0; i--) {
              const j = (seeds[(k * 4 + i) % seeds.length] as number) % (i + 1);
              [order[i], order[j]] = [order[j] as number, order[i] as number];
            }
            expect(deliver(base, all, admin, order)).toEqual(causal);
          }
        },
      ),
      { numRuns: 150 },
    );
  }, 120_000);
});
