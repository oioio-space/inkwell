// @vitest-environment node
import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import { Coverage, changeChars } from "./coverage";

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

describe("Coverage.filter growth cap (ADR 0028)", () => {
  /** A covered peer message carrying the changes `doc` has over `base`. */
  function from(cov: Coverage, doc: A.Doc<Doc>, base: A.Doc<Doc>): Uint8Array | undefined {
    cov.attested("w", "doc", A.getHeads(doc));
    return cov.filter("doc", "p", message(A.getHeads(doc), newChanges(doc, base)));
  }

  it("drops a covered change that alone would take the text past the limit, before the Repo", () => {
    const cov = new Coverage(100);
    const big = typed(A.from<Doc>({ text: "" }), "x".repeat(150));
    expect(from(cov, big, A.init())).toBeUndefined();
    expect(cov.stats.dropped).toBe(1);
  });

  it("counts what several messages add: the second one that crosses the limit is dropped", () => {
    const cov = new Coverage(100);
    const root = A.from<Doc>({ text: "" });
    const one = typed(root, "x".repeat(60));
    const two = typed(one, "y".repeat(60));
    expect(from(cov, one, A.init())).toBeDefined();
    expect(from(cov, two, one)).toBeUndefined();
  });

  it("deletions give the room back: net growth is what counts", () => {
    const cov = new Coverage(100);
    const root = A.from<Doc>({ text: "" });
    const one = typed(root, "x".repeat(60));
    const cut = A.change(one, (d) => A.splice(d, ["text"], 0, 50, ""));
    const two = typed(cut, "y".repeat(60));
    expect(from(cov, one, A.init())).toBeDefined();
    expect(from(cov, cut, one)).toBeDefined();
    expect(from(cov, two, cut)).toBeDefined();
  });

  it("drops a whole-document fragment whose text is past the limit", () => {
    const cov = new Coverage(100);
    const doc = typed(A.from<Doc>({ text: "" }), "x".repeat(150));
    cov.attested("w", "doc", A.getHeads(doc));
    const msg = message(A.getHeads(doc), [A.save(doc)]);
    expect(cov.filter("doc", "p", msg)).toBeUndefined();
  });

  it("a change or a document sent again after a reconnect is not counted twice", () => {
    const cov = new Coverage(100);
    const one = typed(A.from<Doc>({ text: "" }), "x".repeat(60));
    expect(from(cov, one, A.init())).toBeDefined();
    expect(from(cov, one, A.init())).toBeDefined(); // the same change again
    const msg = message(A.getHeads(one), [A.save(one)]);
    expect(cov.filter("doc", "p", msg)).toBeDefined();
    expect(cov.filter("doc", "p", message(A.getHeads(one), [A.save(one)]))).toBeDefined();
  });

  it("a whole document taken in counts at its own length: the cap is the limit, not twice it", () => {
    const cov = new Coverage(100);
    const doc = typed(A.from<Doc>({ text: "" }), "x".repeat(60));
    cov.attested("w", "doc", A.getHeads(doc));
    expect(cov.filter("doc", "p", message(A.getHeads(doc), [A.save(doc)]))).toBeDefined();
    const more = typed(A.clone(doc), "y".repeat(60)); // 120 characters in all
    expect(from(cov, more, doc)).toBeUndefined();
    const small = typed(A.clone(doc), "y".repeat(30)); // 90: fine
    expect(from(cov, small, doc)).toBeDefined();
  });

  it("the count matches the editor's length (UTF-16) with emoji, joined sequences and multi-character splices", () => {
    const samples = ["a😀b", "👨‍👩‍👧 famille", "é🇫🇷", "😀".repeat(50), "plain"];
    for (const sample of samples) {
      const doc = typed(A.from<Doc>({ text: "" }), sample);
      const change = newChanges(doc, A.init()).at(-1);
      expect(changeChars(change as Uint8Array), sample).toBe(sample.length);
    }
    // A splice that inserts several characters at once, and one that removes some.
    let doc = typed(A.from<Doc>({ text: "" }), "début😀fin");
    const base = doc;
    doc = A.change(doc, (d) => A.splice(d, ["text"], 5, 0, "👍👍ab"));
    const [added] = newChanges(doc, base);
    expect(changeChars(added as Uint8Array)).toBe("👍👍ab".length);
    const cut = A.change(doc, (d) => A.splice(d, ["text"], 0, 3, ""));
    const [removed] = newChanges(cut, doc);
    expect(changeChars(removed as Uint8Array)).toBe(-3);
  });

  it("an emoji flood is counted at its UTF-16 size against the cap", () => {
    const cov = new Coverage(100);
    const doc = typed(A.from<Doc>({ text: "" }), "😀".repeat(60)); // 120 units, 60 scalars
    expect(from(cov, doc, A.init())).toBeUndefined();
  });

  it("an ordinary change is untouched by the default limit", () => {
    const { honest } = setup();
    const cov = new Coverage();
    cov.attested("w", "doc", A.getHeads(honest));
    const msg = message(A.getHeads(honest), newChanges(honest, A.init()));
    expect(cov.filter("doc", "p", msg)).toBe(msg);
  });
});
