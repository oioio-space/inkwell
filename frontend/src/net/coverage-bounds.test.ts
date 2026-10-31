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

function setup() {
  const root = A.from<Doc>({ text: "" });
  const honest = typed(typed(root, "a"), "b");
  const forged = typed(A.clone(root), "X");
  return { root, honest, forged };
}

describe("Coverage keeps bounded state (V2.4.1)", () => {
  /** What an unattested reader sends: a forged change under a made-up document id. */
  function flood(cov: Coverage, n: number, peer = "reader") {
    const { root, forged } = setup();
    const msg = message(A.getHeads(forged), newChanges(forged, root));
    for (let i = 0; i < n; i++) cov.filter(`invented-${peer}-${i}`, peer, msg);
  }

  it("creates no per-document state for documents nobody attested", () => {
    const cov = new Coverage();
    flood(cov, 300);
    expect(cov.size.docs).toBe(0);
    expect(cov.size.counted).toBe(0);
    expect(cov.size.rewritten).toBeLessThanOrEqual(256);
  });

  it("clears the per-peer state when the peer disconnects", () => {
    const cov = new Coverage();
    flood(cov, 50);
    expect(cov.size.rewritten).toBeGreaterThan(0);
    cov.forgetPeer("reader");
    expect(cov.size.rewritten).toBe(0);
  });

  it("bounds the rewrite memory across peers", () => {
    const cov = new Coverage();
    for (let p = 0; p < 40; p++) flood(cov, 200, `p${p}`);
    expect(cov.size.rewritten).toBeLessThanOrEqual(4096);
  });

  it("still ends the loop of a repeated rewritten message", () => {
    const cov = new Coverage();
    const { root, forged } = setup();
    const msg = message(A.getHeads(forged), newChanges(forged, root));
    expect(cov.filter("d", "p", msg)).toBeDefined();
    expect(cov.filter("d", "p", msg)).toBeUndefined();
    expect(cov.stats.repeated).toBe(1);
  });

  it("tracks attested documents and still passes covered changes", () => {
    const { honest } = setup();
    const cov = new Coverage();
    flood(cov, 100);
    cov.attested("w", "doc", A.getHeads(honest));
    expect(cov.size.docs).toBe(1);
    const msg = message(A.getHeads(honest), newChanges(honest, A.init()));
    expect(cov.filter("doc", "p", msg)).toBe(msg);
  });

  it("caps the documents tracked, globally and per signer", () => {
    const cov = new Coverage(undefined, { maxDocs: 5, maxDocsPerSigner: 3 });
    for (let i = 0; i < 10; i++) cov.attested("a", `d${i}`, ["h"]);
    expect(cov.size.docs).toBe(3);
    for (let i = 0; i < 10; i++) cov.attested("b", `e${i}`, ["h"]);
    expect(cov.size.docs).toBe(5);
  });

  it("forgets a document when its last signer is revoked", () => {
    const cov = new Coverage();
    cov.attested("a", "d1", ["h"]);
    cov.attested("b", "d1", ["h"]);
    cov.revoked("a");
    expect(cov.size.docs).toBe(1);
    cov.revoked("b");
    expect(cov.size.docs).toBe(0);
  });
});

it("keeps global digest bound when the oldest peer sends again", () => {
  const cov = new Coverage();
  const forged = A.from({ text: "forged" });
  const msg = A.encodeSyncMessage({
    heads: A.getHeads(forged),
    need: [],
    have: [],
    changes: A.getAllChanges(forged),
  });
  cov.filter("first", "oldest", msg);
  for (let p = 0; p < 16; p++) for (let d = 0; d < 255; d++) cov.filter(`d${d}`, `p${p}`, msg);
  for (let d = 0; d < 30; d++) cov.filter(`again${d}`, "oldest", msg);
  expect(cov.size.rewritten).toBeLessThanOrEqual(4096);
});
