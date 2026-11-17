import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import type { ChangeMark } from "../editor/changeGutter";
import { analyze } from "./analysis";
import { changeMarks, END, resolver, sinceHeads } from "./gutter";
import { claimsOf, history, sessionsBefore, stepsOf } from "./log";
import { authorBook } from "./people";
import { SPLIT_AT } from "./pieces";
import { Device } from "./testkit";
import { blameOf, sessionAt, Tracker, type TrackOptions } from "./track";
import { clock } from "./when";

const BASE = "ligne un\nligne deux\nligne trois\nligne quatre\n";

/** Alice writes BASE, Bob joins; returns both, and the heads Alice had when she left. */
function setup() {
  const alice = new Device("aa01").type(BASE, 0).claim("Alice");
  const bob = new Device("bb02", alice.doc).claim("Marie");
  alice.sync(bob);
  return { alice, bob, left: alice.heads };
}

const lineOf = (text: string, pos: number) => text.slice(0, pos).split("\n").length;

async function marksFor(alice: Device, left: string[]): Promise<ChangeMark[]> {
  const doc = alice.doc;
  const steps = stepsOf(doc);
  const sessions = history(doc);
  const blame = await blameOf(doc, sessions);
  const since = sessionsBefore(sessions, steps, left);
  return changeMarks(
    doc,
    blame,
    since,
    authorBook(claimsOf(stepsOf(doc)), { myActors: new Set(["aa01"]) }),
  );
}

describe("change gutter since the last visit (HIS-01 to HIS-05)", () => {
  it("HIS-01 first opening: nothing is marked, the last visit is now", async () => {
    const { alice } = setup();
    expect(sinceHeads(alice.doc, undefined)).toEqual(alice.heads);
    expect(await marksFor(alice, alice.heads)).toEqual([]);
    // Heads this document does not have (another project) count as a first visit too.
    const foreign = { heads: ["0".repeat(64)], at: 1 };
    expect(sinceHeads(alice.doc, foreign)).toEqual(alice.heads);
  });

  it("HIS-02 a person adds, modifies and removes while I was away: three kinds, right lines", async () => {
    const { alice, bob, left } = setup();
    bob.type("ligne zéro\n", 0, 1);
    bob.set(bob.doc.text.replace("ligne deux", "ligne 2"), 1);
    bob.set(bob.doc.text.replace("ligne quatre\n", ""), 1);
    alice.sync(bob);

    const marks = await marksFor(alice, left);
    const text = alice.doc.text;
    expect(text).toBe("ligne zéro\nligne un\nligne 2\nligne trois\n");
    const resolve = resolver(alice.doc);
    const at = (m: ChangeMark) => lineOf(text, resolve(m.from) as number);
    expect(marks.map((m) => [m.kind, at(m)])).toEqual([
      ["added", 1],
      ["modified", 3],
      ["deleted", 5],
    ]);
    // The minimap reads the same marks: one entry per change, all inside the text.
    for (const m of marks) expect(resolve(m.to)).toBeLessThanOrEqual(text.length);
  });

  it("HIS-03 by person: the colour of the author, and « Marie, 14:10 » on hover", async () => {
    const { alice, bob, left } = setup();
    bob.type("ajout\n", 0, 1);
    alice.sync(bob);
    const [mark] = await marksFor(alice, left);
    const author = authorBook(claimsOf(stepsOf(alice.doc)), { myActors: new Set(["aa01"]) })(
      "bb02",
    );
    expect(author.name).toBe("Marie");
    expect(mark?.person).toBe(author.index);
    const session = history(alice.doc).find((s) => s.actor === "bb02" && !s.action);
    expect(mark?.label).toContain(`Marie, ${clock(session?.end ?? 0)}`);
  });

  it("HIS-04 my own typing is not marked", async () => {
    const { alice, bob, left } = setup();
    alice.type("à moi\n", 0, 1);
    bob.type("à lui\n", bob.doc.text.length, 1);
    alice.sync(bob);
    const marks = await marksFor(alice, left);
    const text = alice.doc.text;
    const resolve = resolver(alice.doc);
    expect(marks).toHaveLength(1);
    const from = resolve((marks[0] as ChangeMark).from) as number;
    expect(text.slice(from, from + 6)).toBe("à lui\n");
  });

  it("HIS-05 the marks follow their text when it moves, not the line numbers", async () => {
    const { alice, bob, left } = setup();
    bob.type("NEUF\n", BASE.indexOf("ligne trois"), 1);
    alice.sync(bob);
    const [mark] = await marksFor(alice, left);
    const before = resolver(alice.doc)((mark as ChangeMark).from) as number;
    expect(alice.doc.text.slice(before, before + 4)).toBe("NEUF");
    const lineBefore = lineOf(alice.doc.text, before);
    // Five lines are added above it: the mark is on the same words, three lines further down.
    alice.type("a\nb\nc\nd\ne\n", 0, 1);
    const after = resolver(alice.doc)((mark as ChangeMark).from) as number;
    expect(alice.doc.text.slice(after, after + 4)).toBe("NEUF");
    expect(lineOf(alice.doc.text, after) - lineBefore).toBe(5);
  });

  it("HIS-01 a person's earlier typing, minutes before the last visit, is not marked as new", async () => {
    const { alice, bob } = setup();
    bob.type("avant mon départ\n", bob.doc.text.length, 1);
    alice.sync(bob);
    const left = alice.heads;
    // Marie, the same person, goes on typing two minutes later: one session for the people, but
    // the visit cuts it in two, so only what she wrote after it is marked.
    bob.type("après mon départ\n", bob.doc.text.length, 120);
    alice.sync(bob);
    const result = await analyze(alice.doc, new Tracker(), left);
    const marks = changeMarks(
      alice.doc,
      result.blame,
      result.sinceIdx,
      authorBook(claimsOf(stepsOf(alice.doc)), { myActors: new Set(["aa01"]) }),
    );
    const text = alice.doc.text;
    const resolve = resolver(alice.doc);
    const marked = marks.map((m) =>
      text.slice(resolve(m.from) as number, (resolve(m.to) as number) + 1),
    );
    expect(marked.join("")).toContain("après mon départ");
    expect(marked.join("")).not.toContain("avant mon départ");
  });

  it("a removal at the very end uses the end cursor", async () => {
    const { alice, bob, left } = setup();
    bob.set(BASE.replace("ligne quatre\n", ""), 1);
    alice.sync(bob);
    const [mark] = await marksFor(alice, left);
    expect(mark?.kind).toBe("deleted");
    expect(mark?.from).toBe(END);
  });
});

/**
 * The most the piece list may work to follow `runs` inserted runs (FE1, counts, not the clock):
 * each run finds one place, walking at most every block and one block of at most
 * 2 × SPLIT_AT + 2 pieces, then shifts that block and splits it at most once; the open session's
 * copy and the summary walk the list once more each. Each run adds at most two pieces, and every
 * block but the last holds at least SPLIT_AT. A walk of the text for each run costs the number of
 * pieces each time instead: for thirty thousand runs, a dozen times this bound and more.
 */
function workBound(runs: number): number {
  const pieces = 2 * runs + 1;
  const blocks = Math.ceil(pieces / SPLIT_AT) + 1;
  return runs * (2 * blocks + 6 * SPLIT_AT + 6) + 3 * (blocks + pieces);
}

/** Follows the sessions with a new tracker; returns the blame and what it cost, counted. */
async function counted(doc: A.Doc<{ text: string }>, opts?: TrackOptions) {
  const tracker = new Tracker();
  const blame = await tracker.update(doc, history(doc), opts);
  return { blame, ...tracker.work };
}

describe("who wrote what (DIF-05)", () => {
  it("DIF-05 a large document: computed in slices, the thread is given back, hover is exact", async () => {
    const big = new Device("aa01").type("x".repeat(1_000_000), 0);
    const b = new Device("bb02", big.doc);
    for (let i = 0; i < 8; i++) {
      big.type(`a${i}`, 100 + i * 7, 700);
      b.type(`b${i}`, 600_000 + i * 7, 700);
    }
    big.sync(b);
    let ticks = 0;
    const timer = setInterval(() => ticks++, 1);
    const { blame, finds, steps } = await counted(big.doc, { budget: 1 });
    clearInterval(timer);
    expect(ticks).toBeGreaterThan(0);
    // The cost, counted: each session is followed once, so each run typed (the million characters,
    // then sixteen) finds its place once, and the whole costs what seventeen runs may cost. Following
    // the sessions again from the first at each one finds 137 places.
    expect(finds, "DIF-05 each run found once").toBe(17);
    expect(steps, "DIF-05 work").toBeLessThanOrEqual(workBound(17));
    expect(blame.length).toBe(big.doc.text.length);
    // Exact: the character at an index is attributed to the session that wrote it.
    const at = big.doc.text.indexOf("b7");
    expect(blame.sessions[sessionAt(blame, at)]?.actor).toBe("bb02");
    const mine = big.doc.text.indexOf("a3");
    expect(blame.sessions[sessionAt(blame, mine)]?.actor).toBe("aa01");
    // The spans tile the text without gap.
    let pos = 0;
    for (const sp of blame.spans) {
      expect(sp.from).toBe(pos);
      pos = sp.to;
    }
    expect(pos).toBe(blame.length);
  }, 60_000);

  it("is incremental: only the new sessions are followed again", async () => {
    const a = new Device("aa01").type("un", 0, 1);
    const tracker = new Tracker();
    await tracker.update(a.doc, history(a.doc));
    a.type(" deux", 2, 3600);
    const blame = await tracker.update(a.doc, history(a.doc));
    expect(blame.spans.map((s) => a.doc.text.slice(s.from, s.to))).toEqual(["un", " deux"]);
    expect(A.getHeads(a.doc)).toHaveLength(1);
  });

  it("many scattered edits in one session do not cost a scan of the text for each (no quadratic cost)", async () => {
    // Lines of one character: what the list follows is the number of runs and pieces, not the
    // length of the text, and Automerge's own cost of typing grows with that length (FE1: 30 000
    // edits of « ligne N » cost 4 s to build here, through `updateText` 17 s, then 4 s to merge).
    const lines = 30_000;
    const a = new Device("aa01").type("x\n".repeat(lines), 0);
    // One session edits every line, from the last: 30 000 patches, then 60 000 pieces to follow.
    const b = new Device("bb02", a.doc).edit((d) => {
      for (let i = lines; i > 0; i--) A.splice(d, ["text"], 2 * i - 1, 0, " !");
    });
    const { blame, finds, steps } = await counted(b.doc);
    expect(blame.length).toBe(b.doc.text.length);
    expect(b.doc.text).toBe("x !\n".repeat(lines));
    expect(blame.spans).toHaveLength(2 * lines + 1);
    // The text typed, then one place found for each of the 30 000 edits.
    expect(finds, "no quadratic cost: each run found once").toBe(1 + lines);
    expect(steps, "no quadratic cost: work").toBeLessThanOrEqual(workBound(1 + lines));
  }, 120_000);
});
