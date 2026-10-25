// @vitest-environment node
import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import { Coverage } from "./coverage";

type Doc = { text: string };

function typed(doc: A.Doc<Doc>, s: string): A.Doc<Doc> {
  return A.change(doc, (d) => A.splice(d, ["text"], d.text.length, 0, s));
}

/** A sync message carrying exactly `changes`, as a peer whose heads are `heads` sends it. */
function message(heads: string[], changes: Uint8Array[]): Uint8Array {
  return A.encodeSyncMessage({ heads, need: [], have: [], changes });
}

/** The changes of `doc` that `base` does not have. */
function newChanges(doc: A.Doc<Doc>, base: A.Doc<Doc>): Uint8Array[] {
  return A.getChanges(base, doc);
}

function hashes(data: Uint8Array | undefined): string[] {
  if (!data) return [];
  return A.decodeSyncMessage(data).changes.map((c) => A.decodeChange(c).hash);
}

function setup() {
  const root = A.from<Doc>({ text: "" });
  const honest = typed(typed(root, "a"), "b");
  const forged = typed(A.clone(root), "X");
  return { root, honest, forged };
}

describe("Coverage.filter", () => {
  it("passes changes covered by an attested head, unchanged", () => {
    const { honest } = setup();
    const cov = new Coverage();
    cov.attested("w", "doc", A.getHeads(honest));
    const msg = message(A.getHeads(honest), newChanges(honest, A.init()));
    expect(cov.filter("doc", "p", msg)).toBe(msg);
    expect(cov.stats.removed).toBe(0);
  });

  it("removes a change that no writer covers, and its head", () => {
    const { root, honest, forged } = setup();
    const cov = new Coverage();
    cov.attested("w", "doc", A.getHeads(honest));
    const fChanges = newChanges(forged, root);
    const msg = message(A.getHeads(forged), fChanges);
    const out = cov.filter("doc", "p", msg);
    expect(out).toBeDefined();
    expect(hashes(out)).toEqual([]);
    // The heads no longer name the change we refused: it would make us ask for it.
    expect(A.decodeSyncMessage(out as Uint8Array).heads).toEqual([]);
    expect(cov.stats.removed).toBe(1);
  });

  it("keeps the covered changes of a mixed message", () => {
    const { root, honest, forged } = setup();
    const cov = new Coverage();
    cov.attested("w", "doc", A.getHeads(honest));
    const good = newChanges(honest, root);
    const bad = newChanges(forged, root);
    const out = cov.filter("doc", "p", message(A.getHeads(honest), [...good, ...bad]));
    expect(hashes(out).sort()).toEqual(good.map((c) => A.decodeChange(c).hash).sort());
  });

  it("does not depend on the order of the changes", () => {
    const { root, honest } = setup();
    const cov = new Coverage();
    cov.attested("w", "doc", A.getHeads(honest));
    const reversed = newChanges(honest, root).reverse();
    const msg = message(A.getHeads(honest), reversed);
    expect(cov.filter("doc", "p", msg)).toBe(msg);
  });

  it("covers ancestors that arrive after their descendant", () => {
    const { root, honest } = setup();
    const cov = new Coverage();
    cov.attested("w", "doc", A.getHeads(honest));
    const [first, second] = newChanges(honest, root);
    if (!first || !second) throw new Error("expected two changes");
    const late = message(A.getHeads(honest), [second]); // the head, without its parent
    expect(cov.filter("doc", "p", late)).toBe(late);
    const parent = message(A.getHeads(honest), [first]);
    expect(cov.filter("doc", "p", parent)).toBe(parent);
  });

  it("covers a stranger's change only through a writer who accepted it", () => {
    const { root, forged } = setup();
    const cov = new Coverage();
    const f = newChanges(forged, root);
    expect(hashes(cov.filter("doc", "p", message(A.getHeads(forged), f)))).toEqual([]);
    cov.attested("w", "doc", A.getHeads(forged)); // the writer took it in, and vouches
    const msg = message(A.getHeads(forged), f);
    expect(cov.filter("doc", "p", msg)).toBe(msg);
  });

  it("keeps documents apart", () => {
    const { root, honest } = setup();
    const cov = new Coverage();
    cov.attested("w", "doc1", A.getHeads(honest));
    const out = cov.filter("doc2", "p", message(A.getHeads(honest), newChanges(honest, root)));
    expect(hashes(out)).toEqual([]);
  });

  it("accepts a full-document fragment that is covered", () => {
    const { honest } = setup();
    const cov = new Coverage();
    cov.attested("w", "doc", A.getHeads(honest));
    const fragment = A.save(honest);
    const msg = message(A.getHeads(honest), [fragment]);
    expect(cov.filter("doc", "p", msg)).toBe(msg);
    expect(cov.stats.fragments).toBe(1);
  });

  it("drops the whole message when a fragment has an uncovered change", () => {
    const { honest, forged } = setup();
    const cov = new Coverage();
    cov.attested("w", "doc", A.getHeads(honest));
    // A document made of honest history plus the stranger's change.
    const merged = A.merge(A.clone(honest), forged);
    const out = cov.filter(
      "doc",
      "p",
      message(A.getHeads(merged), [A.save(merged), ...newChanges(honest, A.init())]),
    );
    expect(out).toBeUndefined();
    expect(cov.stats.dropped).toBe(1);
  });

  it("drops what does not decode", () => {
    const cov = new Coverage();
    expect(cov.filter("doc", "p", new Uint8Array([1, 2, 3]))).toBeUndefined();
    const { honest } = setup();
    const junk = new Uint8Array(40);
    expect(cov.filter("doc", "p", message(A.getHeads(honest), [junk]))).toBeUndefined();
  });

  it("does not deliver twice in a row a message it had to cut", () => {
    const { root, forged } = setup();
    const cov = new Coverage();
    const msg = message(A.getHeads(forged), newChanges(forged, root));
    expect(cov.filter("doc", "p", msg)).toBeDefined();
    expect(cov.filter("doc", "p", msg)).toBeUndefined();
    expect(cov.stats.repeated).toBe(1);
    expect(cov.filter("doc", "q", msg)).toBeDefined(); // another peer
    cov.forgetPeer("p"); // p reconnected: its sync state starts over
    expect(cov.filter("doc", "p", msg)).toBeDefined();
  });

  it("passes a message without changes (a reader's request)", () => {
    const cov = new Coverage();
    const msg = message([], []);
    expect(cov.filter("doc", "p", msg)).toBe(msg);
  });
});

describe("Coverage.revoked", () => {
  it("voids what only the revoked writer covered, until another writer attests it", () => {
    const { root, honest } = setup();
    const cov = new Coverage();
    const heads = A.getHeads(honest);
    const changes = newChanges(honest, root);
    cov.attested("x", "doc", heads);
    cov.revoked("x");
    expect(hashes(cov.filter("doc", "p", message(heads, changes)))).toEqual([]);
    cov.attested("y", "doc", heads); // the history accepted by y
    const msg = message(heads, changes);
    expect(cov.filter("doc", "p", msg)).toBe(msg);
  });

  it("keeps what another writer covers", () => {
    const { root, honest } = setup();
    const cov = new Coverage();
    const heads = A.getHeads(honest);
    cov.attested("x", "doc", heads);
    cov.attested("y", "doc", heads);
    cov.revoked("x");
    const msg = message(heads, newChanges(honest, root));
    expect(cov.filter("doc", "p", msg)).toBe(msg);
  });

  it("ignores an unknown writer", () => {
    expect(() => new Coverage().revoked("nobody")).not.toThrow();
  });
});
