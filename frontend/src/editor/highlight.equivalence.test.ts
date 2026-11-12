// @vitest-environment node
// Block LF0a2 (gates/LF0a2.toml, criterion "equivalence"): the incremental parsers give exactly the
// colours and the tree of a full parse. Typst: the patched codemirror-lang-typst against the
// package's original one-shot path (still used for multi-range parses), comparing what the editor
// paints (highlightTree spans and classes), the per-node tags and the tree. Markdown: the app's
// Markdown parser, reusing fragments, against a fresh parse. Small chunk and window sizes make every
// reuse path, resynchronisation and window overflow happen on small files.
import { mkdirSync, readFileSync, realpathSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { ensureSyntaxTree, Language, LanguageSupport } from "@codemirror/language";
import { EditorState } from "@codemirror/state";
import { type Parser, type Tree, TreeFragment } from "@lezer/common";
import {
  type TypstLezerParser,
  typst_lezer,
  typstLezerLanguageData,
  typstTags,
} from "codemirror-lang-typst/lezer";
import { describe, expect, it } from "vitest";
import {
  type CorpusFile,
  firstDiff,
  type MarkdownCase,
  markdownCases,
  mounts,
  nodeTags,
  randomChange,
  repoMarkdown,
  repoTypst,
  rng,
  spans,
  structure,
  typstSuite,
} from "./highlight.testkit";

const SEEDS = 6;
const LONG = 1_700_000;

/**
 * The first difference between two trees (nested-language trees, painted spans, tags, structure), or
 * null. Exact, except inside a nested tree whose own parse recovered from a syntax error: there, what
 * reaches past its first error node (the earlier of both trees) is not compared. @lezer/lr, which
 * parses the nested HTML, JavaScript and CSS and is not patched, recovers differently after an edit
 * or a resumed parse than in a full one, unpatched packages included (testdata/highlight/
 * markdown-cases.json, test "LF0 upstream LR recovery is not incremental-stable"). The nested trees'
 * kinds, ranges and clean/recovered state are compared exactly, so a clean one is compared in full.
 */
function compare(ref: Tree, got: Tree): string | null {
  const [mr, mg] = [mounts(ref), mounts(got)];
  const dm = firstDiff(
    mr.map((m) => m.key),
    mg.map((m) => m.key),
  );
  if (dm) return `mounts #${dm.i}: ${dm.a} | ${dm.b}`;
  const recovered = mr.flatMap((m, i) =>
    m.firstError === null
      ? []
      : [
          {
            from: m.from,
            to: m.to,
            err: Math.min(m.firstError, mg[i]?.firstError ?? m.firstError),
          },
        ],
  );
  const compared = (entry: string) => {
    const r = /(?:^|@)(\d+)-(\d+)/.exec(entry);
    const [from, to] = [Number(r?.[1]), Number(r?.[2])];
    return !recovered.some((m) => from >= m.from && to <= m.to && to > m.err);
  };
  for (const [what, a, b] of [
    ["spans", spans(ref), spans(got)],
    ["tags", nodeTags(ref, typstTags), nodeTags(got, typstTags)],
    ["structure", structure(ref), structure(got)],
  ] as const) {
    const d = firstDiff(a.filter(compared), b.filter(compared));
    if (d) return `${what} #${d.i}: ${d.a} | ${d.b}`;
  }
  return null;
}

function apply(text: string, c: { from: number; to: number; insert: string }): string {
  return text.slice(0, c.from) + c.insert + text.slice(c.to);
}

function fragmentsAfter(tree: Tree, c: { from: number; to: number; insert: string }) {
  return TreeFragment.applyChanges(TreeFragment.addTree(tree), [
    { fromA: c.from, toA: c.to, fromB: c.from, toB: c.from + c.insert.length },
  ]);
}

interface Harness {
  /** The parser under test (incremental). */
  parser: Parser;
  /** The same language, for an EditorState (partial parses, stopAt, pauses). */
  support: LanguageSupport;
  /** The reference full parse of a text. */
  reference: (text: string) => Tree;
}

/**
 * Random edits on every file: a direct chain (parse with fragments) and a CodeMirror chain,
 * each step compared with the reference. Returns the edit count and the failures.
 */
function randomEdits(h: Harness, files: CorpusFile[], pieces: readonly string[], seed: number) {
  const rand = rng(seed);
  let edits = 0;
  const failures: string[] = [];
  for (const file of files) {
    let text = file.text;
    let tree = h.parser.parse(text);
    for (let e = 0; e < 25; e++) {
      const c = randomChange(rand, text.length, pieces);
      text = apply(text, c);
      tree = h.parser.parse(text, fragmentsAfter(tree, c));
      edits++;
      const d = compare(h.reference(text), tree);
      if (d) {
        failures.push(`${file.name} seed ${seed} direct #${e}: ${d}`);
        break;
      }
    }
    let state = EditorState.create({ doc: file.text, extensions: h.support });
    for (let e = 0; e < 15; e++) {
      state = state.update({ changes: randomChange(rand, state.doc.length, pieces) }).state;
      edits++;
      if (e % 3 !== 2) continue; // let edits pile up on partial trees
      const tree2 = ensureSyntaxTree(state, state.doc.length, 1e9);
      const d = tree2 ? compare(h.reference(state.doc.toString()), tree2) : "no tree";
      if (d) {
        failures.push(`${file.name} seed ${seed} editor #${e}: ${d}`);
        break;
      }
    }
  }
  return { edits, failures };
}

// Edits that change Typst's or Markdown's structure, separated by "¦".
const typstPieces = [
  ...'[¦]¦(¦)¦{¦}¦$¦*¦_¦`¦```¦#¦= ¦- ¦+ ¦/ t: ¦//¦/*¦*/¦"¦<l>¦@r¦\\¦  ¦a¦word ¦é¦else '.split("¦"),
  "\n",
  "\n\n",
  "#let x = 1\n",
  "#if x [a]\n",
];
const markdownPieces = [
  ..."#¦# ¦- ¦* ¦1. ¦> ¦```¦~~~¦`¦*¦_¦**¦~~¦  ¦    ¦[a](b)¦![i](u)¦|¦<div>¦</div>¦<script>¦</script>¦<!--¦-->¦&amp;¦\\¦a¦word ¦é¦- [ ] ".split(
    "¦",
  ),
  "\n",
  "\n\n",
  "| a | b |\n",
  "---\n",
];

/** The patched Typst parser (tuned small) and the package's original path as reference. */
function typstHarness(tuning: Parameters<TypstLezerParser["tune"]>[0]): Harness {
  const base = typst_lezer().language.parser as TypstLezerParser;
  const parser = base.tune(tuning);
  return {
    parser,
    support: new LanguageSupport(new Language(typstLezerLanguageData, parser, [], "typst")),
    // Two ranges take the package's original one-shot parse (kept for multi-range parses).
    reference: (text) =>
      base.parse(text, undefined, [
        { from: 0, to: 0 },
        { from: 0, to: text.length },
      ]),
  };
}

const SMALL = { chunk: 64, window: 256, sliceMs: 0, big: 300 };

const md = markdown({ base: markdownLanguage, addKeymap: false });

/**
 * A fixed case's reference and result trees: a full parse of the final text, and either one edit
 * after a full parse or a parse stopped at `stopAt` then resumed from its partial tree (what
 * CodeMirror does when a parse runs out of its time budget). `skip` drops that many leading
 * characters (to run the nested language's parser on its region alone).
 */
function markdownCase(
  k: MarkdownCase,
  parser: Parser = md.language.parser,
  skip = 0,
): [Tree, Tree] {
  const text = k.text.slice(skip);
  if (k.edit) {
    const c = { ...k.edit, from: k.edit.from - skip, to: k.edit.to - skip };
    const after = apply(text, c);
    return [parser.parse(after), parser.parse(after, fragmentsAfter(parser.parse(text), c))];
  }
  const partial = parser.startParse(text);
  partial.stopAt((k.stopAt ?? 0) - skip);
  let tree: Tree | null = null;
  while (!tree) tree = partial.advance();
  return [parser.parse(text), parser.parse(text, TreeFragment.addTree(tree, [], true))];
}

describe("LF0a2 highlighting equivalence", () => {
  const typstFiles = [...typstSuite(), ...repoTypst()];
  const allTypst = { name: "all", text: typstFiles.map((f) => f.text).join("\n\n") };

  it(
    "LF0 Typst patched parser equals the original on the corpus",
    () => {
      const failures: string[] = [];
      for (const tuning of [SMALL, {}]) {
        const h = typstHarness(tuning);
        for (const f of [...typstFiles, allTypst]) {
          const d = compare(h.reference(f.text), h.parser.parse(f.text));
          if (d) failures.push(`${f.name}: ${d}`);
        }
      }
      expect(failures).toEqual([]);
    },
    LONG,
  );

  it(
    "LF0 Typst patched parser equals the original on random edits",
    () => {
      const h = typstHarness(SMALL);
      let edits = 0;
      const failures: string[] = [];
      // At least 6 seeds, and seeds until 50 000 edits whatever the corpus size.
      for (let seed = 1; seed <= SEEDS || edits < 50_000; seed++) {
        const r = randomEdits(h, typstFiles, typstPieces, seed);
        edits += r.edits;
        failures.push(...r.failures);
      }
      expect(failures).toEqual([]);
      expect(edits).toBeGreaterThanOrEqual(50_000);
    },
    LONG,
  );

  it(
    "LF0 the window-guard mutant is caught",
    async () => {
      // A string that runs past the text window: without the guard, error recovery re-reads its
      // quote as markup and later points look valid.
      const text = `${"#let a = 1\n".repeat(5)}#let s = "abc\n${"more text\n".repeat(30)}end"\n${"tail\n".repeat(10)}`;
      const sample = (mod: { typst_lezer: typeof typst_lezer }) => {
        const base = mod.typst_lezer().language.parser as TypstLezerParser;
        const ref = base.parse(text, undefined, [
          { from: 0, to: 0 },
          { from: 0, to: text.length },
        ]);
        return compare(ref, base.tune({ chunk: 1, window: 256 }).parse(text));
      };
      expect(sample({ typst_lezer })).toBeNull();
      const src = createRequire(import.meta.url)
        .resolve("codemirror-lang-typst/lezer")
        .replace(/\.cjs$/, ".js");
      const guard = "        if (this.touchedEnd) {";
      const code = readFileSync(src, "utf8");
      expect(code).toContain(guard);
      const dir = fileURLToPath(new URL("../../test-results/lf0-mutant/", import.meta.url));
      mkdirSync(dir, { recursive: true });
      // The mutant resolves the package's own dependencies, as the same module instances.
      const deps = dirname(dirname(dirname(realpathSync(src))));
      rmSync(`${dir}node_modules`, { force: true });
      symlinkSync(deps, `${dir}node_modules`);
      writeFileSync(`${dir}lezer-mutant.js`, code.replace(guard, "        if (false) {"));
      const mutant = await import(/* @vite-ignore */ `${dir}lezer-mutant.js`);
      expect(sample(mutant)).not.toBeNull();
    },
    LONG,
  );

  const mdHarness: Harness = {
    parser: md.language.parser,
    support: md,
    reference: (text) => md.language.parser.parse(text),
  };
  const mdFiles = repoMarkdown();

  it(
    "LF0 Markdown fixed parser equals the original on the corpus",
    () => {
      const failures: string[] = [];
      for (const f of mdFiles) {
        // Reparse the file after an edit at its middle and an undo: the result must equal a fresh parse.
        const mid = f.text.length >> 1;
        const c = { from: mid, to: mid, insert: "x" };
        const once = mdHarness.parser.parse(apply(f.text, c));
        const back = { from: mid, to: mid + 1, insert: "" };
        const tree = mdHarness.parser.parse(f.text, fragmentsAfter(once, back));
        const d = compare(mdHarness.reference(f.text), tree);
        if (d) failures.push(`${f.name}: ${d}`);
      }
      // The minimised cases the random edits found, fixed so they never depend on docs that change.
      for (const k of markdownCases()) {
        const d = compare(...markdownCase(k));
        if (d) failures.push(`${k.name}: ${d}`);
      }
      expect(mdFiles.length).toBeGreaterThan(20);
      expect(failures).toEqual([]);
    },
    LONG,
  );

  it(
    "LF0 upstream LR recovery is not incremental-stable",
    () => {
      // Why compare() skips what follows the first error of a recovered nested tree: on the fixed
      // cases, the nested JavaScript differs there on its own, with nothing of Markdown or
      // parseMixed in the way (@lezer/javascript and @lezer/lr are not patched). If an upgrade makes
      // this test fail, upstream became stable: drop the exception from compare().
      for (const k of markdownCases()) {
        const [ref, got] = markdownCase(k);
        // Compared exactly, the painted colours differ: compare() accepts them only by the exception.
        expect(firstDiff(spans(ref), spans(got)), k.name).not.toBeNull();
        const script = mounts(ref).find((m) => m.key.startsWith("Script@"));
        expect(script?.firstError, k.name).not.toBeNull();
        if (!script) continue;
        const [jsRef, jsGot] = markdownCase(k, script.parser, script.from);
        expect(jsRef.toString(), k.name).not.toBe(jsGot.toString());
      }
    },
    LONG,
  );

  it(
    "LF0 Markdown fixed parser equals the original on random edits",
    () => {
      let edits = 0;
      const failures: string[] = [];
      // At least 6 seeds, and seeds until 50 000 edits whatever the corpus size.
      for (let seed = 1; seed <= SEEDS || edits < 50_000; seed++) {
        const r = randomEdits(mdHarness, mdFiles, markdownPieces, seed);
        edits += r.edits;
        failures.push(...r.failures);
      }
      expect(failures).toEqual([]);
      expect(edits).toBeGreaterThanOrEqual(50_000);
    },
    LONG,
  );
});
