import type { EditorState } from "@codemirror/state";
import { fenceAt } from "./fence";
import type { Lang } from "./lang";
import { syntaxOf } from "./support";
import type { Block, Mark } from "./types";

/**
 * The marks in force at the main selection. Inside inline code only "code" is reported (so
 * its button shows pressed); inside a code block or a formula nothing is: marks are not
 * interpreted there. Marks never span lines.
 */
export function activeMarks(state: EditorState, lang: Lang): Set<Mark> {
  const active = new Set<Mark>();
  const syntax = syntaxOf(lang);
  if (!syntax) return active;
  const range = state.selection.main;
  const line = state.doc.lineAt(range.head);
  if (fenceAt(state.doc, line.number)) return active;
  const from = Math.max(range.from, line.from) - line.from;
  const to = Math.min(range.to, line.to) - line.from;
  const scan = syntax.scan(line.text);
  const inside = scan.verbatim.find(([a, b]) => a < from && to < b);
  if (inside) {
    if (line.text[inside[0]] === "`") active.add("code");
    return active;
  }
  for (const span of scan.spans) {
    if (span.open[1] <= from && to <= span.close[0]) active.add(span.mark);
  }
  return active;
}

/** The block type of the line at the main selection. */
export function activeBlock(state: EditorState, lang: Lang): Block {
  const syntax = syntaxOf(lang);
  if (!syntax) return "paragraph";
  const line = state.doc.lineAt(state.selection.main.head);
  if (fenceAt(state.doc, line.number)) return "codeblock";
  return syntax.classify(line.text).block;
}

const MARKS: ReadonlySet<string> = new Set<Mark>([
  "bold",
  "italic",
  "underline",
  "strike",
  "code",
  "link",
]);
/** The blocks a bar button stands for (a heading is the heading menu's, a paragraph is none). */
const TOGGLED_BLOCKS: ReadonlySet<string> = new Set<Block>([
  "codeblock",
  "ul",
  "ol",
  "task",
  "quote",
]);

/**
 * Whether a feature's button shows pressed at the caret: a mark that applies, the block the caret
 * is in; undefined for what is no toggle (a formula, a table are insertions).
 */
export function featureState(
  state: EditorState,
  lang: Lang,
): (feature: string) => boolean | undefined {
  const marks = activeMarks(state, lang);
  const block = activeBlock(state, lang);
  return (feature) =>
    MARKS.has(feature)
      ? marks.has(feature as Mark)
      : TOGGLED_BLOCKS.has(feature)
        ? block === feature
        : undefined;
}
