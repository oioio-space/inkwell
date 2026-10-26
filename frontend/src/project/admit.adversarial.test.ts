// @vitest-environment node
import { next as A } from "@automerge/automerge";
import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { HOLD, IndexValidator } from "./admit";
import { imm } from "./model";
import * as ops from "./ops";
import { alice, apply, bob, ctx, type Index, idOf, newProject, textUrl, view } from "./testkit";

/**
 * PRJ-20 whatever is sent, the verdict on each change is the same on every replica, in every
 * delivery order (security audit: crafted changes, not only honest ones). Changes are made on
 * forks of one index, by raw edits no client would make (a name deleted, a node replaced, a
 * stamp forged, a setting overwritten), some by an admin, then merged and built on; each replica
 * gets them in its own order, offers again what waits, and must end with the same changes
 * admitted, refused and waiting, and the same heads.
 */

// biome-ignore lint/suspicious/noExplicitAny: a draft of the index, edited the way a forger would
type Loose = Record<string, any>;
const hexId = (n: number): string => (n % 0xffffff).toString(16).padStart(32, "0");

function project(free: boolean): Index {
  let doc = newProject();
  doc = apply(
    doc,
    ops.create(doc, view(doc), ctx(bob), [
      { type: "text", name: "a.typ", parent: null, url: textUrl() },
      { type: "text", name: "b.typ", parent: null, url: textUrl() },
      { type: "dir", name: "d", parent: null },
    ]),
  );
  if (free) doc = apply(doc, ops.setDeletionMode(doc, ctx(alice), "free"));
  return doc;
}

/** One crafted edit, chosen by `kind`, on node `pick`, with `n` for whatever it needs. */
function craft(doc: Index, kind: number, pick: number, n: number): Index {
  const ids = Object.keys((doc as Loose).nodes ?? {});
  const id = ids[pick % ids.length] as string;
  const other = ids[(pick + n) % ids.length] as string;
  const stamp = { by: imm("bob"), at: n };
  return A.change(doc, (d: Loose) => {
    const node = d.nodes[id];
    switch (kind % 26) {
      case 0:
        node.name = imm(`n${n}.typ`);
        break;
      case 1:
        node.name = imm("../etc");
        break;
      case 2:
        node.name = `plain${n}`; // a text object where a value belongs
        break;
      case 3:
        delete node.name;
        break;
      case 4:
        node.parent = imm(other);
        break;
      case 5:
        node.parent = imm(hexId(n + 7)); // a folder that is not there
        break;
      case 6:
        node.parent = null;
        break;
      case 7:
        delete d.nodes[id];
        break;
      case 8:
        d.nodes[hexId(n + 100)] = {
          type: imm("dir"),
          name: imm(`new${n}`),
          parent: null,
          by: imm("bob"),
          at: 1,
        };
        break;
      case 9:
        d.nodes[hexId(n + 100)] = { name: imm("x") }; // no type, no stamp
        break;
      case 10:
        d.nodes[id] = {
          type: imm("dir"),
          name: imm("replaced"),
          parent: null,
          by: imm("bob"),
          at: 1,
        };
        break;
      case 11:
        node.trashed = stamp;
        break;
      case 12:
        delete node.trashed;
        break;
      case 13:
        node.deletion = { ...stamp, req: imm(hexId(n + 3)) };
        break;
      case 14:
        delete node.deletion;
        break;
      case 15:
        d.main = imm(id);
        break;
      case 16:
        d.settings = { deletion: imm(n % 2 === 0 ? "free" : "approval") };
        break;
      case 17:
        d.settings = { deletion: imm("whenever") };
        break;
      case 18:
        d.sneaky = imm("x");
        break;
      case 19:
        node.type = imm("blob");
        break;
      case 20:
        d.decisions ??= {};
        d.decisions[id] = { ...stamp, req: imm(hexId(n + 3)), verdict: imm("refused") };
        break;
      case 21:
        d.packages ??= {};
        d.packages["@preview/x:0.1.0"] = {
          ...stamp,
          hash: imm(`blake3:${"ab".repeat(32)}`),
          size: 3,
        };
        break;
      case 22:
        A.splice(d, ["text"], 0, 0, "typing");
        break;
      case 23:
        node.ftype = imm(n % 2 === 0 ? "json" : "NOT OK");
        break;
      case 24:
        node.who = imm("someone");
        break;
      default:
        node.name = imm(`m${n}.typ`);
        node.parent = imm(other);
    }
  });
}

interface World {
  base: Index;
  changes: Uint8Array[];
  admin: Set<string>;
}

/** Forks of the base, crafted edits on them, merges and edits on the merges. */
function world(
  free: boolean,
  steps: readonly {
    kind: number;
    pick: number;
    n: number;
    from: number;
    merge: boolean;
    admin: boolean;
  }[],
): World {
  const base = project(free);
  const pool: Index[] = [base];
  const seen = new Map<string, Uint8Array>();
  const admin = new Set<string>();
  for (const s of steps) {
    const from = pool[s.from % pool.length] as Index;
    let doc = A.clone(from);
    if (s.merge && pool.length > 1)
      doc = A.merge(doc, A.clone(pool[(s.from + 1) % pool.length] as Index));
    let next: Index;
    try {
      next = craft(doc, s.kind, s.pick, s.n);
    } catch {
      continue; // an edit the document itself does not allow
    }
    for (const c of A.getChanges(base, next)) {
      const h = A.decodeChange(c).hash;
      if (!seen.has(h)) {
        seen.set(h, c);
        if (s.admin && A.getHeads(next).includes(h)) admin.add(h);
      }
    }
    pool.push(next);
  }
  return { base, changes: [...seen.values()], admin };
}

type Outcome = { admitted: string[]; refused: string[]; held: string[]; heads: string[] };

/** What a replica holding the base ends with when the changes come in `order`. */
function deliver(w: World, order: readonly number[]): Outcome {
  let clock = 0;
  const validator = new IndexValidator(() => w.base, { now: () => (clock += 10_000) });
  let pending = order.map((i) => w.changes[i] as Uint8Array);
  const admitted: string[] = [];
  const refused: string[] = [];
  for (let progress = true; progress && pending.length > 0; ) {
    progress = false;
    const rest: Uint8Array[] = [];
    for (const bytes of pending) {
      const hash = A.decodeChange(bytes).hash;
      const why = validator.check(bytes, (h) => w.admin.has(h));
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
    heads: validator.heads(),
  };
}

const step = fc.record({
  kind: fc.constantFrom(0, 1, 2, 3, 4, 5, 7, 8, 9, 10, 11, 11, 12, 13, 14, 15, 16, 16, 17, 20, 25),
  pick: fc.nat(20),
  n: fc.nat(1000),
  from: fc.nat(20),
  merge: fc.boolean(),
  admin: fc.boolean(),
});

describe("PRJ-20 every delivery order gives the same verdict on each change", () => {
  it("property: crafted changes, forks, merges, any order", () => {
    fc.assert(
      fc.property(
        fc.boolean(),
        fc.array(step, { minLength: 3, maxLength: 14 }),
        fc.array(fc.nat(), { minLength: 40, maxLength: 40 }),
        (free, steps, seeds) => {
          const w = world(free, steps);
          fc.pre(w.changes.length > 1);
          const causal = deliver(
            w,
            w.changes.map((_, i) => i),
          );
          for (let k = 0; k < 4; k++) {
            const order = w.changes.map((_, i) => i);
            for (let i = order.length - 1; i > 0; i--) {
              const j = (seeds[(k * 7 + i) % seeds.length] as number) % (i + 1);
              [order[i], order[j]] = [order[j] as number, order[i] as number];
            }
            expect(deliver(w, order)).toEqual(causal);
          }
        },
      ),
      { numRuns: 200 },
    );
  }, 300_000);

  it("a change that deletes the winning name is judged the same wherever it lands", () => {
    // The audit's case: two replicas of one name, a crafted change deleting the winner.
    const base = project(false);
    const id = idOf(base, "a.typ");
    const rename = A.change(A.clone(base), (d: Loose) => {
      d.nodes[id].name = imm("winner.typ");
    });
    const forged = A.change(A.clone(base), (d: Loose) => {
      d.nodes[id].name = imm("other.typ");
    });
    const merged = A.merge(A.clone(rename), A.clone(forged));
    const crafted = A.change(merged, (d: Loose) => {
      delete d.nodes[id].name;
    });
    const w: World = {
      base,
      changes: A.getChanges(base, crafted),
      admin: new Set(),
    };
    const orders = [
      [0, 1, 2],
      [2, 1, 0],
      [1, 2, 0],
      [1, 0, 2],
    ];
    const first = deliver(w, orders[0] as number[]);
    for (const o of orders) expect(deliver(w, o)).toEqual(first);
    expect(first.admitted).toHaveLength(2); // the two renames; the deletion of a name is refused
    expect(first.refused.length + first.held.length).toBe(1);
  });
});
