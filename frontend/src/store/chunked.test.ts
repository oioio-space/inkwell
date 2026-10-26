// @vitest-environment node
import { next as A } from "@automerge/automerge";
import { Repo } from "@automerge/automerge-repo";
import { describe, expect, it } from "vitest";
import { applyEdits } from "./chunked";
import { CHUNK_CHARS } from "./limits";

type Doc = { text: string };
const repo = new Repo();
const open = (text: string) => repo.create<Doc>({ text });
const changeCount = (h: ReturnType<typeof open>) => A.getAllChanges(h.doc()).length;
/** The text each change inserted, in characters (the change count's weight). */
const sizes = (h: ReturnType<typeof open>) =>
  A.getAllChanges(h.doc()).map((c) =>
    A.decodeChange(c).ops.reduce((n, op) => n + ((op as { insert?: boolean }).insert ? 1 : 0), 0),
  );

describe("PERF-04 a large addition goes in changes of at most 64 Ki", () => {
  it("small edits stay ONE change, as before", () => {
    const h = open("hello world");
    const before = changeCount(h);
    applyEdits(h, [
      { at: 0, remove: 5, put: "HELLO" },
      { at: 11, remove: 0, put: "!" },
    ]);
    expect(h.doc().text).toBe("HELLO world!");
    expect(changeCount(h) - before).toBe(1);
  });

  it("a paste of 300 Ki comes in changes of at most 64 Ki characters, and the text is intact", () => {
    const h = open("<>");
    const before = changeCount(h);
    const big = "abcdefghijklmnopqrstuvwxyz".repeat(Math.ceil((300 * 1024) / 26));
    applyEdits(h, [{ at: 1, remove: 0, put: big }]);
    expect(h.doc().text).toBe(`<${big}>`);
    const added = sizes(h).slice(before);
    expect(added).toHaveLength(Math.ceil(big.length / CHUNK_CHARS));
    for (const n of added) expect(n).toBeLessThanOrEqual(CHUNK_CHARS);
  }, 30_000);

  it("the deletion goes with the first piece; edits around the big one keep their place", () => {
    const h = open("AAA|BBB|CCC");
    const big = "z".repeat(CHUNK_CHARS + 10);
    // Application order: from the end of the text to its start, as merge.ts does.
    applyEdits(h, [
      { at: 11, remove: 0, put: "!" },
      { at: 4, remove: 3, put: big },
      { at: 0, remove: 3, put: "aaa" },
    ]);
    expect(h.doc().text).toBe(`aaa|${big}|CCC!`);
  });

  it("never splits a character outside the BMP", () => {
    const h = open("");
    const big = `${"x".repeat(CHUNK_CHARS - 1)}😀tail`;
    applyEdits(h, [{ at: 0, remove: 0, put: big }]);
    expect(h.doc().text).toBe(big);
  });

  it("no edit at all changes nothing", () => {
    const h = open("same");
    const before = changeCount(h);
    applyEdits(h, []);
    expect(changeCount(h)).toBe(before);
  });
});
