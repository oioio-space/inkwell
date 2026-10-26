import { next as A } from "@automerge/automerge";
import { describe, expect, it } from "vitest";
import type { ChangeMark } from "../editor/changeGutter";
import { analyze } from "./analysis";
import { changeMarks, END, resolver, sinceHeads } from "./gutter";
import { claimsOf, history, sessionsBefore, stepsOf } from "./log";
import { authorBook } from "./people";
import { Device } from "./testkit";
import { blameOf, sessionAt, Tracker } from "./track";
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

describe("who wrote what (DIF-05)", () => {
  it("DIF-05 a large document: computed in slices, the thread is given back, hover is exact", async () => {
    const big = new Device("aa01").type("x".repeat(1_000_000), 0);
    const b = new Device("bb02", big.doc);
    for (let i = 0; i < 8; i++) {
      big.type(`a${i}`, 100 + i * 7, 700);
      b.type(`b${i}`, 600_000 + i * 7, 700);
    }
    big.sync(b);
    const sessions = history(big.doc);
    let ticks = 0;
    const timer = setInterval(() => ticks++, 1);
    const t0 = performance.now();
    const blame = await new Tracker().update(big.doc, sessions, { budget: 1 });
    clearInterval(timer);
    expect(performance.now() - t0).toBeLessThan(20_000);
    expect(ticks).toBeGreaterThan(0);
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
    const lines = Array.from({ length: 30_000 }, (_, i) => `ligne ${i}`);
    const a = new Device("aa01").type(`${lines.join("\n")}\n`, 0);
    // One session rewrites every line: 30 000 patches, then 30 000 pieces to follow.
    const edited = lines.map((l) => `${l} !`).join("\n");
    const b = new Device("bb02", a.doc);
    b.set(`${edited}\n`, 1);
    a.sync(b);
    const t0 = performance.now();
    const blame = await blameOf(a.doc, history(a.doc));
    const ms = performance.now() - t0;
    expect(blame.length).toBe(a.doc.text.length);
    expect(ms).toBeLessThan(4_000);
  }, 120_000);
});
