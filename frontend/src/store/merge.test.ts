// @vitest-environment node
import { next as A } from "@automerge/automerge";
import { Repo } from "@automerge/automerge-repo";
import { describe, expect, it } from "vitest";
import { CHUNK_CHARS } from "./limits";
import { integrate, undoIntegration } from "./merge";

type Doc = { text: string };

const repo = new Repo();
const open = (text: string) => repo.create<Doc>({ text });
const heads = (h: ReturnType<typeof open>) => A.getHeads(h.doc());
const at = (h: ReturnType<typeof open>, index: number, s: string, del = 0) =>
  h.change((d) => A.splice(d, ["text"], index, del, s));

describe("integrate (EXT-01..03, 12)", () => {
  it("EXT-01 takes an outside edit of a clean document, counts its lines and says where", () => {
    const h = open("a\nb\nc\n");
    const base = heads(h);
    const done = integrate(h, base, "a\nB\nc\nd\n");
    expect(h.doc().text).toBe("a\nB\nc\nd\n");
    expect(done?.lines).toBe(2); // line 2 changed, line 4 added
    const marked = done?.inserted.map((r) => h.doc().text.slice(r.from, r.to));
    expect(marked?.join("|")).toContain("B");
    expect(marked?.join("|")).toContain("d");
    // The document at the integration's own heads is the file.
    expect(A.view(h.doc(), done?.branch as A.Heads).text).toBe("a\nB\nc\nd\n");
  });

  it("does nothing when the file says what the document said at the base", () => {
    const h = open("a\n");
    expect(integrate(h, heads(h), "a\n")).toBeUndefined();
  });

  it("EXT-02 keeps what I typed elsewhere while the file changed", () => {
    const h = open("one\ntwo\nthree\n");
    const base = heads(h);
    at(h, 0, "MINE "); // I type on line 1
    integrate(h, base, "one\ntwo\nTHREE\n"); // the file changed line 3
    expect(h.doc().text).toBe("MINE one\ntwo\nTHREE\n");
  });

  it("EXT-03 keeps both insertions when they land in the same place", () => {
    const h = open("start\n");
    const base = heads(h);
    at(h, 5, " mine");
    integrate(h, base, "start theirs\n");
    const text = h.doc().text;
    expect(text).toContain("mine");
    expect(text).toContain("theirs");
    expect(text.startsWith("start")).toBe(true);
    expect(text.endsWith("\n")).toBe(true);
  });

  it("EXT-12 'Annuler' removes only the integration; another person's later typing stays", () => {
    const h = open("alpha\nbeta\ngamma\n");
    const base = heads(h);
    const done = integrate(h, base, "alpha\nbeta, edited outside\ngamma\nomega\n");
    if (!done) throw new Error("not integrated");
    // Another person types inside what was integrated, and before it.
    at(h, h.doc().text.indexOf("outside"), "[peer]");
    at(h, 0, ">> ");
    undoIntegration(h, done);
    expect(h.doc().text).toBe(">> alpha\nbeta[peer]\ngamma\n");
  });

  it("EXT-12 puts back what the integration removed", () => {
    const h = open("alpha\nbeta\ngamma\n");
    const done = integrate(h, heads(h), "alpha\ngamma\n");
    if (!done) throw new Error("not integrated");
    at(h, 0, "> "); // typed meanwhile, elsewhere
    undoIntegration(h, done);
    expect(h.doc().text).toBe("> alpha\nbeta\ngamma\n");
  });

  it("EXT-12 with nothing typed since, 'Annuler' gives back the text as it was", () => {
    const h = open("x\ny\nz\n");
    const before = h.doc().text;
    const done = integrate(h, heads(h), "x\nz\nw\n");
    if (!done) throw new Error("not integrated");
    undoIntegration(h, done);
    expect(h.doc().text).toBe(before);
  });

  it("X-15 leaves out the change of a locked zone and hands it back as a suggestion", () => {
    const h = open("title\nlocked line\nbody\n");
    const base = heads(h);
    const locked = { from: 6, to: 18 }; // "locked line\n"
    const done = integrate(h, base, "title\nlocked CHANGED\nbody edited\n", {
      skip: (r) => r.from < locked.to && r.to > locked.from,
    });
    expect(h.doc().text).toBe("title\nlocked line\nbody edited\n");
    expect(done?.skipped).toEqual([{ from: 6, to: 18, text: "locked CHANGED\n" }]);
  });

  it("X-15 changes only in a locked zone are all suggestions and the text is untouched", () => {
    const h = open("a\nb\n");
    const done = integrate(h, heads(h), "a\nB\n", { skip: () => true });
    expect(h.doc().text).toBe("a\nb\n");
    expect(done?.skipped).toHaveLength(1);
    expect(done?.lines).toBe(0);
  });

  it("PERF-04 a file that brings 100 Ki of new text comes in changes of at most 64 Ki, and what was typed meanwhile stays", () => {
    const h = open("head\ntail\n");
    const base = heads(h);
    at(h, 0, "MINE "); // typed meanwhile
    const lines = Array.from({ length: 12_000 }, (_, i) => `ligne ${i}`).join("\n");
    const before = A.getAllChanges(h.doc()).length;
    const done = integrate(h, base, `head\n${lines}\ntail\n`);
    expect(h.doc().text).toBe(`MINE head\n${lines}\ntail\n`);
    expect(lines.length).toBeGreaterThan(100 * 1024);
    const added = A.getAllChanges(h.doc()).slice(before);
    expect(added.length).toBeGreaterThanOrEqual(Math.ceil(lines.length / CHUNK_CHARS));
    for (const c of added) {
      const inserted = A.decodeChange(c).ops.filter(
        (op) => (op as { insert?: boolean }).insert,
      ).length;
      expect(inserted).toBeLessThanOrEqual(CHUNK_CHARS);
    }
    // The file's own text is the document as of the integration's branch.
    expect(A.view(h.doc(), done?.branch as A.Heads).text).toBe(`head\n${lines}\ntail\n`);
    // Undo takes it all back in one step, and keeps the typing.
    if (done) undoIntegration(h, done);
    expect(h.doc().text).toBe("MINE head\ntail\n");
  }, 30_000);
});
