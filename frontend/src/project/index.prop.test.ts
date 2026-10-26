import { next as A } from "@automerge/automerge";
import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { IndexValidator } from "./admit";
import { type Materialized, materialize } from "./materialize";
import { plain } from "./model";
import { collisionKey, validateName, validatePath } from "./names";
import * as ops from "./ops";
import {
  admin2,
  alice,
  bob,
  ctx,
  HASH,
  type Index,
  newProject,
  type Person,
  textUrl,
  view,
} from "./testkit";

const PEOPLE: Person[] = [alice, bob, admin2];

/** One step of one replica: what is picked is taken modulo what exists there. */
interface Step {
  replica: number;
  kind: number;
  a: number;
  b: number;
  name: number;
}

const NAMES = [
  "notes.md",
  "Notes.md",
  "NOTES.MD",
  "café.md",
  "café.md",
  "ch1.typ",
  "fig.png",
  "docs",
  "Docs",
  "a",
  "b",
];

const step = fc.record({
  replica: fc.nat(2),
  kind: fc.nat(11),
  a: fc.nat(40),
  b: fc.nat(40),
  name: fc.nat(NAMES.length - 1),
});

/** Does one step on a replica, as its client would; steps that cannot be done do nothing. */
function act(doc: Index, who: Person, s: Step): Index {
  const m = view(doc);
  const c = ctx(who, 1000 + s.a);
  const pick = <T>(xs: T[], i: number): T | undefined =>
    xs.length === 0 ? undefined : xs[i % xs.length];
  const entry = pick(m.entries, s.a);
  const dirs = m.entries.filter((e) => e.type === "dir");
  const name = NAMES[s.name] as string;
  let r: ops.Result | undefined;
  switch (s.kind) {
    case 0:
    case 1:
      r = ops.create(doc, m, c, [
        { type: "text", name, parent: pick(dirs, s.b)?.id ?? null, url: textUrl() },
      ]);
      break;
    case 2:
      r = ops.create(doc, m, c, [{ type: "dir", name, parent: pick(dirs, s.b)?.id ?? null }]);
      break;
    case 3:
      r = entry && ops.rename(m, c, entry.id, name);
      break;
    case 4:
    case 5:
      r = entry && ops.move(m, c, [entry.id], pick(dirs, s.b)?.id ?? null);
      break;
    case 6:
      r = entry && ops.remove(doc, m, c, [entry.id]);
      break;
    case 7:
      r = ops.restore(
        doc,
        m,
        c,
        m.trash.slice(0, 1).map((t) => t.id),
      );
      break;
    case 8:
      r = ops.accept(
        m,
        c,
        m.pending.slice(0, 1).map((e) => e.id),
      );
      break;
    case 9:
      r = ops.refuse(
        m,
        c,
        m.pending.slice(0, 1).map((e) => e.id),
      );
      break;
    case 10:
      r = ops.purge(
        doc,
        m,
        c,
        m.trash.slice(0, 1).map((t) => t.id),
      );
      break;
    default:
      r = ops.setDeletionMode(doc, c, s.b % 2 === 0 ? "free" : "approval");
  }
  if (!r?.ok) return doc;
  return A.change(doc, { message: JSON.stringify(r.plan.message), time: 1 }, r.plan.apply);
}

function runAll(base: Index, steps: Step[]): Index[] {
  const replicas = [A.clone(base), A.clone(base), A.clone(base)];
  for (const s of steps) {
    const i = s.replica % replicas.length;
    replicas[i] = act(replicas[i] as Index, PEOPLE[i] as Person, s);
  }
  return replicas;
}

const merge = (...docs: Index[]): Index =>
  docs.slice(1).reduce((acc, d) => A.merge(acc, A.clone(d)), A.clone(docs[0] as Index));

/** What a replica shows, in a form that two replicas can be compared on. */
function shown(m: Materialized) {
  return {
    entries: m.entries.map((e) => [
      e.id,
      e.path,
      e.type,
      e.main,
      e.renamedFrom,
      e.deletion?.req,
      e.refused?.req,
    ]),
    trash: m.trash.map((t) => [t.id, t.path]),
    refs: m.refs,
  };
}

/** What must hold of any tree, whatever was done to it. */
function sound(m: Materialized) {
  const seen = new Map<string, Set<string>>();
  for (const e of m.entries) {
    expect(validatePath(e.path), e.path).toBeUndefined();
    const siblings = seen.get(e.parentId ?? "") ?? new Set<string>();
    const key = collisionKey(e.name);
    expect(siblings.has(key), `duplicate ${e.path}`).toBe(false);
    siblings.add(key);
    seen.set(e.parentId ?? "", siblings);
    // Reachable: its parent is visible too.
    if (e.parentId !== null) expect(m.byId.has(e.parentId)).toBe(true);
  }
}

describe("PRJ-08 PRJ-16 concurrent rename, move, delete and create converge", {
  timeout: 60_000,
}, () => {
  const base = (() => {
    let doc = newProject();
    const m = view(doc);
    const r = ops.create(doc, m, ctx(alice), [
      { type: "dir", name: "docs", parent: null },
      { type: "text", name: "ch1.typ", parent: { spec: 0 }, url: textUrl() },
      { type: "dir", name: "img", parent: null },
      { type: "blob", name: "fig.png", parent: { spec: 2 }, hash: HASH, size: 3 },
    ]);
    if (!r.ok) throw new Error("setup");
    doc = A.change(doc, r.plan.apply);
    return doc;
  })();

  it("any order of merging shows the same tree", () => {
    fc.assert(
      fc.property(fc.array(step, { minLength: 1, maxLength: 36 }), (steps) => {
        const [a, b, c] = runAll(base, steps) as [Index, Index, Index];
        const left = merge(merge(a, b), c);
        const right = merge(c, merge(b, a));
        const mid = merge(b, c, a);
        expect(shown(view(left))).toEqual(shown(view(right)));
        expect(shown(view(left))).toEqual(shown(view(mid)));
        sound(view(left));
      }),
      { numRuns: 300 },
    );
  });

  it("whatever was done, the tree is sound", () => {
    fc.assert(
      fc.property(fc.array(step, { minLength: 1, maxLength: 50 }), (steps) => {
        for (const r of runAll(base, steps)) sound(view(r));
      }),
      { numRuns: 200 },
    );
  });

  it("every change an honest client made is admitted by a replica that holds the one before", () => {
    fc.assert(
      fc.property(fc.array(step, { minLength: 1, maxLength: 20 }), (steps) => {
        let doc = A.clone(base);
        for (const s of steps) {
          const who = PEOPLE[s.replica % PEOPLE.length] as Person;
          const before = A.clone(doc);
          doc = act(doc, who, s);
          if (A.getHeads(doc).join() === A.getHeads(before).join()) continue;
          const bytes = A.getLastLocalChange(doc) as Uint8Array;
          const validator = new IndexValidator(() => A.clone(before));
          // The change was made by `who`: an admin vouches for it only if `who` is one.
          expect(validator.check(bytes, () => who.role === "admin")).toBeUndefined();
        }
      }),
      { numRuns: 150 },
    );
  });
});

describe("PRJ-15 a valid client never produces an invalid name", { timeout: 60_000 }, () => {
  const anyName = fc.oneof(
    fc.string({ unit: "binary" }),
    fc.string({ unit: "grapheme" }),
    fc.constantFrom(
      "..",
      ".",
      "CON",
      "a/b",
      "a\\b",
      ".git",
      "x ".repeat(5),
      "a\u0000",
      "é".repeat(200),
    ),
  );

  it("create and rename either refuse a name or write a valid one that every replica admits", () => {
    const base = newProject();
    fc.assert(
      fc.property(anyName, anyName, (first, second) => {
        let doc = A.clone(base);
        const m = view(doc);
        const made = ops.create(doc, m, ctx(bob), [
          { type: "text", name: first, parent: null, url: textUrl() },
        ]);
        if (made.ok) {
          doc = A.change(doc, made.plan.apply);
          expect(validateName(first)).toBeUndefined();
        } else {
          expect(validateName(first) !== undefined || made.error.code === "exists").toBe(true);
          return;
        }
        const id = Object.keys(plain(doc).nodes ?? {}).find((k) => k !== plain(doc).main) as string;
        const renamed = ops.rename(view(doc), ctx(bob), id, second);
        if (!renamed.ok) {
          expect(
            validateName(second) !== undefined ||
              renamed.error.code === "exists" ||
              renamed.error.code === "unchanged",
          ).toBe(true);
          return;
        }
        const before = A.clone(doc);
        doc = A.change(doc, renamed.plan.apply);
        const validator = new IndexValidator(() => before);
        expect(
          validator.check(A.getLastLocalChange(doc) as Uint8Array, () => false),
        ).toBeUndefined();
        for (const n of Object.values(plain(doc).nodes ?? {})) {
          expect(validateName(n.name)).toBeUndefined();
        }
        expect(materialize(plain(doc)).warnings).toEqual([]);
      }),
      { numRuns: 1500 },
    );
  });
});
