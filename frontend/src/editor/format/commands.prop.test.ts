import { next as A } from "@automerge/automerge";
import { EditorSelection, EditorState } from "@codemirror/state";
import fc from "fast-check";
import { describe, expect, it } from "vitest";
import { removeEmptyDelimiters } from "./cleanup";
import { setBlock, toggleMark } from "./commands";
import type { Lang } from "./lang";
import { run } from "./testkit";
import { snapToGrapheme, wordAt } from "./text";
import type { Block, Mark } from "./types";

// Text and selections are random, over an alphabet that holds no markup character: what the
// properties talk about is the commands' bookkeeping (selection extents, graphemes, line
// breaks, multi-cursor), not the ambiguity of stray delimiters, which the example tests cover.

const UNITS = [
  "a",
  "b",
  "z",
  "7",
  " ",
  " ",
  "\n",
  "\n\n",
  "é",
  "é",
  "你",
  "今日",
  "م",
  "👨‍👩‍👧",
  "🇫🇷",
];

const textArb = fc
  .array(fc.constantFrom(...UNITS), { maxLength: 30 })
  .map((parts) => parts.join(""));

/** A document with a selection of one to three ranges at arbitrary code-unit offsets. */
const stateArb = fc
  .tuple(textArb, fc.array(fc.tuple(fc.nat(1000), fc.nat(1000)), { minLength: 1, maxLength: 3 }))
  .map(([doc, pairs]) => {
    const at = (n: number) => n % (doc.length + 1);
    const selection = EditorSelection.create(
      pairs.map(([a, b]) => EditorSelection.range(at(a), at(b))),
    );
    return EditorState.create({
      doc,
      selection,
      extensions: EditorState.allowMultipleSelections.of(true),
    });
  });

const MARKS: [Lang, Mark][] = [
  ["typst", "bold"],
  ["typst", "italic"],
  ["typst", "underline"],
  ["typst", "strike"],
  ["typst", "code"],
  ["markdown", "bold"],
  ["markdown", "italic"],
  ["markdown", "strike"],
  ["markdown", "code"],
];
const markArb = fc.constantFrom(...MARKS);

const BLOCKS: Block[] = ["h1", "h2", "h3", "h4", "paragraph", "ul", "ol", "quote", "codeblock"];
const blockCase = fc.tuple(fc.constantFrom<Lang>("typst", "markdown"), fc.constantFrom(...BLOCKS));

describe("properties", () => {
  it("FMT-27 applying then removing a mark gives back the original text", () => {
    fc.assert(
      fc.property(stateArb, markArb, (state, [lang, mark]) => {
        const toggle = toggleMark(lang, mark);
        const once = run(state, toggle);
        const twice = run(once.state, toggle);
        expect(twice.state.doc.toString()).toBe(state.doc.toString());
      }),
      { numRuns: 1000 },
    );
  });

  it("FMT-27 the same holds for a block that toggles (lists, quote)", () => {
    const toggled: Block[] = ["ul", "ol", "quote", "codeblock"];
    fc.assert(
      fc.property(
        stateArb,
        fc.constantFrom<Lang>("typst", "markdown"),
        fc.constantFrom(...toggled),
        (state, lang, block) => {
          if (lang === "typst" && block === "quote" && /\n/.test(state.doc.toString())) {
            // A multi-line Typst quote is a bracket pair around lines: covered by example tests.
            return;
          }
          const command = setBlock(lang, block);
          const once = run(state, command);
          // Applying (or unfencing) must be undone by the same command.
          const twice = run(once.state.update({ selection: once.state.selection }).state, command);
          if (block === "codeblock") {
            // Fences cannot be told from content once the caret lies elsewhere: only check the
            // text kept its lines.
            expect(
              twice.state.doc
                .toString()
                .replace(/```\n?/g, "")
                .replace(/\n{2,}/g, "\n")
                .trim(),
            ).toBe(
              state.doc
                .toString()
                .replace(/\n{2,}/g, "\n")
                .trim(),
            );
            return;
          }
          expect(twice.state.doc.toString()).toBe(state.doc.toString());
        },
      ),
      { numRuns: 500 },
    );
  });

  it("FMT-28 a mark changes nothing beyond its selections, give or take the delimiters", () => {
    const DELIMITER = 20; // longest delimiter: `#underline[` plus code padding
    fc.assert(
      fc.property(stateArb, markArb, (state, [lang, mark]) => {
        const done = run(state, toggleMark(lang, mark));
        const zones = state.selection.ranges.map((r) => {
          const line = state.doc.lineAt(r.head);
          const wordA = wordAt(line.text, r.from - line.from);
          const wordB = wordAt(line.text, r.to - line.from);
          return [
            Math.min(r.from, wordA ? line.from + wordA[0] : r.from) - DELIMITER,
            Math.max(r.to, wordB ? line.from + wordB[1] : r.to) + DELIMITER,
          ] as const;
        });
        for (const tr of done.transactions) {
          tr.changes.iterChangedRanges((from, to) => {
            expect(zones.some(([a, b]) => a <= from && to <= b)).toBe(true);
          });
        }
      }),
      { numRuns: 1000 },
    );
  });

  it("FMT-28 a block change stays within the lines of the selections", () => {
    fc.assert(
      fc.property(stateArb, blockCase, (state, [lang, block]) => {
        const done = run(state, setBlock(lang, block));
        const zones = state.selection.ranges.map(
          (r) => [state.doc.lineAt(r.from).from, state.doc.lineAt(r.to).to] as const,
        );
        for (const tr of done.transactions) {
          tr.changes.iterChangedRanges((from, to) => {
            expect(zones.some(([a, b]) => a <= from && to <= b)).toBe(true);
          });
        }
      }),
      { numRuns: 500 },
    );
  });

  it("FMT-28 a command is at most one transaction", () => {
    fc.assert(
      fc.property(stateArb, markArb, (state, [lang, mark]) => {
        expect(run(state, toggleMark(lang, mark)).dispatched).toBeLessThanOrEqual(1);
      }),
      { numRuns: 300 },
    );
  });

  it("never splits a grapheme: every inserted delimiter sits on a boundary", () => {
    const graphemes = new Intl.Segmenter(undefined, { granularity: "grapheme" });
    fc.assert(
      fc.property(stateArb, markArb, (state, [lang, mark]) => {
        const after = run(state, toggleMark(lang, mark)).state.doc.toString();
        // No cluster mixes a delimiter with a letter: delimiters are ASCII punctuation, so a
        // cluster that holds one must be only delimiter characters.
        for (const { segment } of graphemes.segment(after)) {
          if (/[*_~`#[\]]/.test(segment) && /\P{ASCII}/u.test(segment)) {
            throw new Error(`delimiter fused with text in ${JSON.stringify(segment)}`);
          }
        }
      }),
      { numRuns: 500 },
    );
  });
});

describe("X-03 concurrent mark and deletion", () => {
  // Markdown italic is left out: an emptied `*x*` leaves `**`, which cannot be told from text.
  const cases = MARKS.filter(([lang, mark]) => !(lang === "markdown" && mark === "italic"));
  const DELIMS = /#underline\[|#strike\[|[\]*_~`]/g;

  it("A marks a word while B deletes it: the merge leaves no empty delimiter pair", () => {
    fc.assert(
      fc.property(
        stateArb,
        fc.constantFrom(...cases),
        fc.tuple(fc.nat(1000), fc.nat(1000)),
        (state, [lang, mark], [x, y]) => {
          const original = state.doc.toString();
          const at = (n: number) => n % (original.length + 1);
          // B deletes whole characters, as a person would.
          const from = snapToGrapheme(original, Math.min(at(x), at(y)), -1);
          const to = snapToGrapheme(original, Math.max(at(x), at(y)), 1);

          const base = A.from({ text: original });
          let alice = A.clone(base);
          let bob = A.clone(base);

          const done = run(state, toggleMark(lang, mark));
          const edits: [number, number, string][] = [];
          for (const tr of done.transactions) {
            tr.changes.iterChanges((a, b, _c, _d, inserted) => {
              edits.push([a, b, inserted.toString()]);
            });
          }
          alice = A.change(alice, (d) => {
            for (const [a, b, ins] of edits.reverse()) A.splice(d, ["text"], a, b - a, ins);
          });
          bob = A.change(bob, (d) => {
            A.splice(d, ["text"], from, to - from, "");
          });

          const left = A.merge(A.clone(alice), bob).text;
          const right = A.merge(A.clone(bob), alice).text;
          expect(left).toBe(right); // both people converge

          const cleaned = removeEmptyDelimiters(left, lang);
          // Nothing was lost: every letter B did not delete is still there.
          expect(cleaned.replace(DELIMS, "")).toBe(original.slice(0, from) + original.slice(to));
          // Cleaning is a fixed point: no empty pair is left behind.
          expect(removeEmptyDelimiters(cleaned, lang)).toBe(cleaned);
          // Pairs come in twos: no delimiter was left alone.
          const delimiters = cleaned.match(DELIMS)?.length ?? 0;
          expect(delimiters % 2).toBe(0);
          if (lang === "typst") expect(cleaned).not.toMatch(/\*\*|__|#underline\[\]|#strike\[\]/);
          if (lang === "markdown") expect(cleaned).not.toMatch(/\*\*\*\*|~~~~/);
        },
      ),
      { numRuns: 1000 },
    );
  }, 60_000);

  it("A marks a word while B deletes it: the example from the plan", () => {
    const base = A.from({ text: "un mot ici" });
    let alice = A.clone(base);
    let bob = A.clone(base);
    alice = A.change(alice, (d) => {
      A.splice(d, ["text"], 6, 0, "**");
      A.splice(d, ["text"], 3, 0, "**");
    });
    bob = A.change(bob, (d) => {
      A.splice(d, ["text"], 3, 3, "");
    });
    const merged = A.merge(alice, bob).text;
    expect(merged).toBe("un **** ici");
    expect(removeEmptyDelimiters(merged, "markdown")).toBe("un  ici");
  });
});
