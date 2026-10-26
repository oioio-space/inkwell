// @vitest-environment node
import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import { Coverage, type DocValidator } from "../net/coverage";
import { HOLD, IndexValidator } from "./admit";
import { imm } from "./model";
import * as ops from "./ops";
import { alice, apply, bob, ctx, type Index, idOf, newProject, textUrl, view } from "./testkit";

/**
 * What waits is held, never refused (security audit): a change that needs an admin's
 * attestation, or whose dependencies are not here yet, is kept out of the Repo for now, stays
 * out of the refused list, and is judged again when roles or attestations change. A joiner
 * that receives the document before the roles and the admin's attestations ends up the same.
 */

function base(): Index {
  let doc = newProject();
  doc = apply(
    doc,
    ops.create(doc, view(doc), ctx(bob), [
      { type: "text", name: "ch1.typ", parent: null, url: textUrl() },
    ]),
  );
  return doc;
}

describe("PRJ-18 a change that waits is held, not refused", () => {
  it("needs an admin: held, then admitted when an admin vouches; its dependents wait too", () => {
    const b = base();
    const t = idOf(b, "ch1.typ");
    // An admin's deletion under approval, then a writer's rename that builds on it.
    let doc = apply(A.clone(b), ops.remove(b, view(b), ctx(alice), [t]));
    const first = A.getChanges(b, doc);
    const trashed = first[0] as Uint8Array;
    doc = apply(
      doc,
      ops.create(doc, view(doc), ctx(bob), [{ type: "dir", name: "d", parent: null }]),
    );
    const later = A.getChanges(b, doc)[1] as Uint8Array;

    const v = new IndexValidator(() => b);
    expect(v.check(trashed, () => false)).toMatch(new RegExp(`^${HOLD}`));
    expect(v.check(later, () => false)).toMatch(new RegExp(`^${HOLD}`)); // builds on a held change
    // The same change, judged again once an admin vouches for it.
    expect(v.check(trashed, () => true)).toBeUndefined();
    expect(v.check(later, () => true)).toBeUndefined();
  });

  it("dependencies unknown: held, and admitted when they arrive", () => {
    const b = base();
    const t = idOf(b, "ch1.typ");
    let doc = apply(A.clone(b), ops.rename(view(b), ctx(bob), t, "intro.typ"));
    doc = apply(
      doc,
      ops.create(doc, view(doc), ctx(bob), [{ type: "dir", name: "d", parent: null }]),
    );
    const [one, two] = A.getChanges(b, doc) as [Uint8Array, Uint8Array];
    const v = new IndexValidator(() => b);
    expect(v.check(two, () => false)).toMatch(new RegExp(`^${HOLD}`));
    expect(v.check(one, () => false)).toBeUndefined();
    expect(v.check(two, () => false)).toBeUndefined();
  });

  it("a joiner that gets the document before the roles and attestations converges", () => {
    const b = base();
    const t = idOf(b, "ch1.typ");
    const source = apply(A.clone(b), ops.remove(b, view(b), ctx(alice), [t]));
    const message = A.encodeSyncMessage({
      heads: A.getHeads(source),
      need: [],
      have: [],
      changes: [A.save(source)],
    });

    const cov = new Coverage();
    cov.setValidator("doc", new IndexValidator(() => A.clone(b)));
    cov.attested("bob", "doc", A.getHeads(b)); // the writer's own history is covered
    cov.attested("alice", "doc", A.getHeads(source)); // her attestation comes ahead of the document
    // The whole document arrives before the roles: nobody knows yet that alice is an admin.
    expect(cov.filter("doc", "alice", message)).toBeUndefined();
    expect(cov.stats.removed).toBe(0); // held, not refused
    // Then the roles.
    cov.setAdmins(["alice"]);
    const released = cov.releaseHeld();
    expect(released).toHaveLength(1);

    let joiner = A.clone(b);
    for (const r of released) {
      const msg = A.decodeSyncMessage(r.data);
      for (const chunk of msg.changes) joiner = A.loadIncremental(joiner, chunk);
    }
    expect([...A.getHeads(joiner)].sort()).toEqual([...A.getHeads(source)].sort());
    expect(cov.releaseHeld()).toEqual([]); // nothing is delivered twice
  });

  it("a forged change stays refused: it is not held for ever", () => {
    const b = base();
    const t = idOf(b, "ch1.typ");
    const forged = A.change(A.clone(b), (d) => {
      const nodes = d.nodes as unknown as Record<string, Record<string, unknown>>;
      (nodes[t as string] as Record<string, unknown>).name = "../x";
    });
    const v = new IndexValidator(() => b);
    const why = v.check(A.getLastLocalChange(forged) as Uint8Array, () => true);
    expect(why).toBe("invalid name");
  });
});

describe("PRJ-21 what is held costs nothing until what it waits for happens", () => {
  /** A validator that counts the judgements asked of it. */
  function counted(b: Index, now?: () => number) {
    const inner = new IndexValidator(() => b, now ? { now } : {});
    const seen = { checks: 0 };
    const v: DocValidator = {
      check(bytes, admin) {
        seen.checks++;
        return inner.check(bytes, admin);
      },
      checkDocument: (doc, admin) => inner.checkDocument(doc as never, admin),
      resolved: (h) => inner.resolved(h),
      hasBudget: () => inner.hasBudget(),
    };
    return { v, seen, inner };
  }

  const message = (changes: Uint8Array[]) =>
    A.encodeSyncMessage({ heads: [], need: [], have: [], changes });

  type Draft = { nodes: Record<string, Record<string, unknown>>; text: string };

  it("the cap counts changes, not messages", () => {
    const b = base();
    let doc = A.clone(b);
    const chain: Uint8Array[] = [];
    for (let i = 0; i < 600; i++) {
      doc = A.change(doc, (d) => {
        (d as unknown as Draft).text = `${i}`;
      });
      chain.push(A.getLastLocalChange(doc) as Uint8Array);
    }
    const { v } = counted(b);
    const cov = new Coverage();
    cov.setValidator("doc", v);
    cov.attested(
      "bob",
      "doc",
      chain.map((c) => A.decodeChange(c).hash),
    );
    // The first is lost: all the others wait for it, in one message.
    cov.filter("doc", "bob", message(chain.slice(1)));
    expect(cov.heldChanges).toBeLessThanOrEqual(256);
    expect(cov.heldChanges).toBeGreaterThan(0);
  });

  it("an event judges again only what its news concerns", () => {
    const b = base();
    const t = idOf(b, "ch1.typ");
    const { v, seen } = counted(b);
    const cov = new Coverage();
    cov.setValidator("doc", v);
    // Twenty forged deletions by a writer: each needs an admin, each waits.
    const forged: Index[] = [];
    for (let i = 0; i < 20; i++) {
      forged.push(
        A.change(A.clone(b), (d) => {
          ((d as unknown as Draft).nodes[t] as Record<string, unknown>).trashed = {
            by: imm(`bob${i}`),
            at: i,
          };
        }),
      );
    }
    for (const f of forged) {
      cov.attested("bob", "doc", A.getHeads(f));
      cov.filter("doc", "bob", message(A.getChanges(b, f)));
    }
    expect(cov.heldChanges).toBe(20);
    const after = seen.checks;
    // A hundred events that vouch for something else: nothing is judged again.
    for (let i = 0; i < 100; i++) {
      cov.attested("carol", "doc", A.getHeads(b));
      cov.releaseHeld();
    }
    expect(seen.checks).toBe(after);
    // An admin vouches for one of them: that one, and no other, is judged again.
    cov.attested("alice", "doc", A.getHeads(forged[3] as Index));
    cov.setAdmins(["alice"]);
    const released = cov.releaseHeld();
    expect(released).toHaveLength(1);
    expect(seen.checks).toBe(after + 1);
  });

  it("a change that waited for what it builds on is judged when that arrives", () => {
    const b = base();
    const t = idOf(b, "ch1.typ");
    const one = apply(A.clone(b), ops.rename(view(b), ctx(bob), t, "one.typ"));
    const two = apply(one, ops.rename(view(one), ctx(bob), t, "two.typ"));
    const [first, second] = A.getChanges(b, two) as [Uint8Array, Uint8Array];
    const { v, seen } = counted(b);
    const cov = new Coverage();
    cov.setValidator("doc", v);
    cov.attested("bob", "doc", A.getHeads(two));
    cov.filter("doc", "bob", message([second]));
    expect(cov.heldChanges).toBe(1);
    const waiting = seen.checks;
    cov.releaseHeld(); // nothing arrived yet
    expect(seen.checks).toBe(waiting);
    cov.filter("doc", "bob", message([first]));
    const released = cov.releaseHeld();
    expect(released).toHaveLength(1);
    expect(cov.heldChanges).toBe(0);
  });

  it("a spent budget holds a change, it does not refuse it; time gives the budget back", () => {
    const b = base();
    const t = idOf(b, "ch1.typ");
    // Moves: each reads the history, unless it is made on the very heads of the copy.
    const moves: Uint8Array[] = [];
    for (let i = 0; i < 30; i++) {
      const f = A.change(A.clone(b), (d) => {
        ((d as unknown as Draft).nodes[t] as Record<string, unknown>).parent = imm(
          `${i}`.padStart(32, "0"),
        );
      });
      moves.push(A.getLastLocalChange(f) as Uint8Array);
    }
    // The copy moved on since: the moves are concurrent with what it holds.
    const doc = A.change(b, (d) => {
      (d as unknown as Draft).text = "typed";
    });
    let clock = 0;
    const { inner } = counted(doc, () => clock);
    const verdicts = moves.map((m) => inner.check(m, () => false));
    const held = verdicts.filter((x) => x?.startsWith(HOLD));
    expect(held.length).toBeGreaterThan(0);
    expect(verdicts.filter((x) => x !== undefined && !x.startsWith(HOLD))).toEqual([]);
    clock += 60_000; // a minute later
    const again = moves.map((m, i) =>
      verdicts[i]?.startsWith(HOLD) ? inner.check(m, () => false) : verdicts[i],
    );
    expect(again.filter((x) => x?.startsWith(HOLD)).length).toBeLessThan(held.length);
  });
});
