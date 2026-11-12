// @vitest-environment node
// Block LF0a2 (gates/LF0a2.toml, criterion "no-freeze"): one PERF case per highlighted language, at
// 2, 10 and 50 MB, with the app's extensions (languageExtension). For each case:
//  - no single parse step (one advance() of the language's parse) over 50 ms, and no keystroke
//    (state.update, CodeMirror's synchronous 20 ms of parse included) over 50 ms;
//  - keystrokes at 10 %, 50 % and 90 %: the colours of the viewport (the edit + 1 500 chars) present
//    within 16 ms at p95;
//  - the whole file eventually coloured.
// PERF-HL-TYPST-OPEN opens the file (viewport at the top, where the app opens it): first colours
// within 16 ms, then the rest in slices. PERF-HL-TYPST-UNCLOSED has an unclosed `$` near the start:
// no step and no keystroke over 50 ms, the whole file eventually coloured; its viewport colours
// are block LF0b. PERF-HL-NESTED: Markdown fences and Typst raw blocks (no nested parser wired).
// Budgets are asserted with INKWELL_PERF=1 (machine at rest); otherwise the files are 100x smaller
// and the figures are only logged.
import { ensureSyntaxTree, type Language, LanguageSupport } from "@codemirror/language";
import { EditorState, type Extension, StateEffect } from "@codemirror/state";
import { NodeProp, type Parser, type PartialParse } from "@lezer/common";
import { describe, expect, it } from "vitest";
import { languageExtension } from "./highlight";
import { repeatTo, repoMarkdown, repoTypst } from "./highlight.testkit";

const PERF = process.env.INKWELL_PERF === "1";
const SCALE = PERF ? 1 : 0.01;
const STEP_MS = 50;
const VISIBLE_MS = 16;
const VIEW = 1500;
const now = () => performance.now();

/** The Language of an extension built by languageExtension. */
function languageOf(ext: Extension): Language {
  const found = [ext].flat(3).find((e) => e instanceof LanguageSupport) as
    | LanguageSupport
    | undefined;
  if (!found) throw new Error("no LanguageSupport in the extension");
  return found.language;
}

/** Records the duration of every advance() of the language's parses. Returns a stop function. */
function timeSteps(lang: Language, steps: number[]): () => void {
  const parser = lang.parser as Parser;
  const create = parser.createParse;
  parser.createParse = function (this: Parser, ...args: Parameters<Parser["createParse"]>) {
    const parse: PartialParse = create.apply(this, args);
    const advance = parse.advance.bind(parse);
    parse.advance = () => {
      const t = now();
      const r = advance();
      steps.push(now() - t);
      return r;
    };
    return parse;
  };
  return () => {
    parser.createParse = create;
  };
}

function max(xs: number[]): number {
  let m = 0;
  for (const x of xs) if (x > m) m = x;
  return m;
}

function p95(xs: number[]): number {
  const s = [...xs].sort((a, b) => a - b);
  return s[Math.min(s.length - 1, Math.floor(0.95 * s.length))] ?? 0;
}

const typstText = (size: number) => repeatTo(repoTypst(), size);
const mdText = (size: number) => repeatTo(repoMarkdown(), size);
const htmlBlock =
  '<div class="note">\n<style>\n.note { color: red; }\n</style>\n<script>\nconst x = [1, 2].map((n) => n * 2);\n</script>\n<span>inline <b>html</b></span>\n</div>\n\n';
const fence = "```js\nconst a = 1; // code\n```\n\n```rust\nfn main() {}\n```\n\n";
const raw = '```rust\nfn main() { println!("raw"); }\n```\n\n';
/** Puts `block` before every `every`-th paragraph, cut back to `size`. */
function interleave(text: string, block: string, every: number, size: number): string {
  return text
    .split("\n\n")
    .map((p, i) => (i % every === 0 ? block : "") + p)
    .join("\n\n")
    .slice(0, size);
}

interface Part {
  ext: Extension;
  text: (size: number) => string;
}
interface Case {
  id: string;
  title: (mb: number) => string;
  parts: Part[];
  /** Assert the viewport colours within 16 ms (not for the unclosed case: block LF0b). */
  viewport: boolean;
  /** Open the file: first colours, then the rest. */
  open?: boolean;
  /** No nested parser may be mounted in a fence or raw block. */
  noNested?: boolean;
}

const typst = languageExtension("typst");
const md = languageExtension("markdown");
const within = (id: string) => (mb: number) => `${id} within budget at ${mb} MB`;
const CASES: Case[] = [
  {
    id: "PERF-HL-TYPST",
    title: within("PERF-HL-TYPST"),
    parts: [{ ext: typst, text: typstText }],
    viewport: true,
  },
  {
    id: "PERF-HL-TYPST-OPEN",
    title: within("PERF-HL-TYPST-OPEN"),
    parts: [{ ext: typst, text: typstText }],
    viewport: true,
    open: true,
  },
  {
    id: "PERF-HL-TYPST-UNCLOSED",
    title: (mb) => `PERF-HL-TYPST-UNCLOSED never freezes at ${mb} MB`,
    parts: [
      {
        ext: typst,
        text: (size) => {
          const t = typstText(size);
          const at = t.indexOf("\n", Math.floor(t.length / 100)) + 1;
          return `${t.slice(0, at)}$ unclosed\n${t.slice(at, size - 11)}`;
        },
      },
    ],
    viewport: false,
  },
  {
    id: "PERF-HL-MD",
    title: within("PERF-HL-MD"),
    parts: [{ ext: md, text: mdText }],
    viewport: true,
  },
  {
    id: "PERF-HL-MD-HTML",
    title: within("PERF-HL-MD-HTML"),
    parts: [{ ext: md, text: (size) => interleave(mdText(size), htmlBlock, 8, size) }],
    viewport: true,
  },
  {
    id: "PERF-HL-NESTED",
    title: within("PERF-HL-NESTED"),
    parts: [
      { ext: md, text: (size) => interleave(mdText(size), fence, 6, size) },
      { ext: typst, text: (size) => interleave(typstText(size), raw, 6, size) },
    ],
    viewport: true,
    noNested: true,
  },
];

/** Runs one part of a case at one size; returns the budget misses. */
function run(c: Case, part: Part, size: number): string[] {
  const text = part.text(size);
  const steps: number[] = [];
  const stop = timeSteps(languageOf(part.ext), steps);
  const updates: number[] = [];
  const visible: number[] = [];
  try {
    // Opening: the language is added to a state that already holds the text (splitting the
    // text into lines is the editor's cost, not highlighting's).
    let state = EditorState.create({ doc: text });
    let t = now();
    state = state.update({ effects: StateEffect.appendConfig.of(part.ext) }).state;
    const open = now() - t;
    if (c.open) {
      updates.push(open);
      t = now();
      ensureSyntaxTree(state, Math.min(text.length, VIEW), 1e9);
      visible.push(open + now() - t);
    }
    // The rest of the file, in parse steps as CodeMirror's worker does, then the keystrokes.
    let tree = ensureSyntaxTree(state, text.length, 1e9);
    for (const frac of [0.1, 0.5, 0.9]) {
      for (let r = 0; r < 7; r++) {
        const pos = Math.min(state.doc.length, Math.floor(frac * state.doc.length) + r * 997);
        t = now();
        state = state.update({ changes: { from: pos, insert: "x" } }).state;
        const u = now() - t;
        updates.push(u);
        if (c.viewport) {
          t = now();
          ensureSyntaxTree(state, Math.min(state.doc.length, pos + VIEW), 1e9);
          visible.push(u + now() - t);
        }
        tree = ensureSyntaxTree(state, state.doc.length, 1e9);
      }
    }
    const misses: string[] = [];
    if (!tree || tree.length !== state.doc.length)
      misses.push(`coloured up to ${tree?.length ?? 0} of ${state.doc.length}`);
    if (c.noNested) {
      // Fences and raw blocks get no nested parser (Markdown's HTML is nested by design).
      let mounted = 0;
      tree?.iterate({
        enter: (n) => {
          if (
            n.tree?.prop(NodeProp.mounted) &&
            /^(FencedCode|CodeText|CodeBlock|Raw.*)$/.test(n.name)
          )
            mounted++;
        },
      });
      if (mounted) misses.push(`${mounted} nested parsers mounted in code blocks`);
    }
    const maxStep = max(steps);
    const maxUpdate = max(updates);
    if (maxStep > STEP_MS) misses.push(`parse step ${maxStep.toFixed(1)} ms > ${STEP_MS}`);
    if (maxUpdate > STEP_MS) misses.push(`keystroke ${maxUpdate.toFixed(1)} ms > ${STEP_MS}`);
    if (visible.length && p95(visible) > VISIBLE_MS)
      misses.push(`visible colours p95 ${p95(visible).toFixed(1)} ms > ${VISIBLE_MS}`);
    console.log(
      `${c.id} ${(size / 1e6).toFixed(2)} MB: ${steps.length} steps, max ${maxStep.toFixed(1)} ms; keystroke max ${maxUpdate.toFixed(1)} ms; visible p95 ${p95(visible).toFixed(1)} ms; ${misses.length ? misses.join("; ") : "ok"}`,
    );
    return misses;
  } finally {
    stop();
  }
}

describe("LF0a2 highlighting never freezes", () => {
  for (const c of CASES)
    for (const mb of [2, 10, 50])
      it(c.title(mb), () => {
        const misses = c.parts.flatMap((part) => run(c, part, Math.round(mb * 1e6 * SCALE)));
        if (PERF) expect(misses).toEqual([]);
      }, 900_000);
});
