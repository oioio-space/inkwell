import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import { changeMarks } from "./gutter";
import { claimsOf, history, stepsOf } from "./log";
import { authorBook } from "./people";
import { Device } from "./testkit";
import { blameOf } from "./track";
import { actionMessage } from "./types";

describe("who is who (X-20): claims are only as good as their author", () => {
  it("a claim counts for the actor that made it, from the metadata of its own change", () => {
    const a = new Device("aa01").type("x", 0).claim("Alice", "peer-a");
    const claims = claimsOf(stepsOf(a.doc));
    expect(claims.aa01).toEqual({ name: "Alice", peer: "peer-a" });
  });

  it("a claim written in the document body for someone else's actor is ignored", () => {
    const alice = new Device("aa01").type("x", 0).claim("Alice", "peer-a");
    const mallory = new Device("bb02", alice.doc).type("y", 1);
    // Mallory rewrites what the document says about Alice's actor, and about her own.
    mallory.edit((d) => {
      d.meta ??= {};
      (d.meta as { actors?: Record<string, unknown> }).actors = {
        aa01: { name: "Mallory", peer: "peer-m", at: 0 },
      };
    });
    alice.sync(mallory);
    const claims = claimsOf(stepsOf(alice.doc));
    expect(claims.aa01?.name).toBe("Alice");
    expect(claims.bb02).toBeUndefined();
    expect(authorBook(claims, {})("bb02").known).toBe(false);
  });

  it("a claim message cannot speak for another actor: it is read from the change's own actor", () => {
    const alice = new Device("aa01").type("x", 0).claim("Alice", "peer-a");
    const mallory = new Device("bb02", alice.doc);
    // Mallory's change carries a claim: it is Mallory's, whatever peer it names.
    mallory.edit(
      () => undefined,
      1,
      actionMessage("claim", JSON.stringify({ name: "Alice", peer: "peer-a" })),
    );
    mallory.doc = A.emptyChange(mallory.doc, {
      message: actionMessage("claim", JSON.stringify({ name: "Alice", peer: "peer-a" })),
    });
    alice.sync(mallory);
    const claims = claimsOf(stepsOf(alice.doc));
    expect(claims.aa01).toEqual({ name: "Alice", peer: "peer-a" });
    expect(claims.bb02).toEqual({ name: "Alice", peer: "peer-a" });
    // ... and what Mallory writes is not mine just because it names my device.
    const book = authorBook(claims, { myActors: new Set(["aa01"]) });
    expect(book("aa01").mine).toBe(true);
    expect(book("bb02").mine).toBe(false);
  });

  it("a peer cannot hide its edits from my margin by claiming to be me", async () => {
    const alice = new Device("aa01").type("base\n", 0).claim("Alice", "peer-a");
    const left = alice.heads;
    const mallory = new Device("bb02", alice.doc);
    mallory.doc = A.emptyChange(mallory.doc, {
      message: actionMessage("claim", JSON.stringify({ name: "Alice", peer: "peer-a" })),
    });
    mallory.type("injecté\n", mallory.doc.text.length, 1);
    alice.sync(mallory);
    const sessions = history(alice.doc);
    const blame = await blameOf(alice.doc, sessions);
    const book = authorBook(claimsOf(stepsOf(alice.doc)), { myActors: new Set(["aa01"]) });
    const first = sessions.findIndex((s) => s.actor === "bb02" && !s.claimOnly);
    const marks = changeMarks(alice.doc, blame, Math.max(0, first), book);
    expect(marks.length).toBeGreaterThan(0);
    expect(left.length).toBeGreaterThan(0);
  });
});
