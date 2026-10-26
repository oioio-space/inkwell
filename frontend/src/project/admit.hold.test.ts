// @vitest-environment node
import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import { Coverage } from "../net/coverage";
import { HOLD, IndexValidator } from "./admit";
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
