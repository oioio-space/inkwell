import { redo, undo } from "@codemirror/commands";
import type { StateCommand } from "@codemirror/state";
import { activeBlock } from "./active";
import { setBlock, toggleMark } from "./commands";
import { insert } from "./insert";
import type { Lang } from "./lang";
import { supports } from "./support";
import type { Block, Feature } from "./types";

/**
 * A heading level, or a paragraph again when the line is already at that level (spec §8.2: the
 * same chord a second time; `Mod-Alt-0`, a Ctrl+Alt chord, is gone, G-98).
 */
const heading =
  (lang: Lang, level: Block): StateCommand =>
  (target) =>
    setBlock(lang, activeBlock(target.state, lang) === level ? "paragraph" : level)(target);

/** What each `format.<feature>` command does to the text, in a given language. */
const COMMANDS = {
  bold: (l) => toggleMark(l, "bold"),
  italic: (l) => toggleMark(l, "italic"),
  underline: (l) => toggleMark(l, "underline"),
  strike: (l) => toggleMark(l, "strike"),
  link: (l) => toggleMark(l, "link"),
  code: (l) => toggleMark(l, "code"),
  codeblock: (l) => setBlock(l, "codeblock"),
  formula: (l) => insert(l, "formula"),
  ul: (l) => setBlock(l, "ul"),
  ol: (l) => setBlock(l, "ol"),
  task: (l) => setBlock(l, "task"),
  quote: (l) => setBlock(l, "quote"),
  table: (l) => insert(l, "table"),
  h1: (l) => heading(l, "h1"),
  h2: (l) => heading(l, "h2"),
  h3: (l) => heading(l, "h3"),
  h4: (l) => heading(l, "h4"),
  paragraph: (l) => setBlock(l, "paragraph"),
} as const satisfies Partial<Record<Feature, (lang: Lang) => StateCommand>>;

export type FormatFeature = keyof typeof COMMANDS;
export const FORMAT_FEATURES = Object.keys(COMMANDS) as FormatFeature[];

/** The features a language can write: the `formats` of the `when` context. */
export const formatsOf = (lang: Lang): Set<string> =>
  new Set(FORMAT_FEATURES.filter((f) => supports(lang, f)));

/**
 * The text surface's handlers of the `format.*` commands (ext/format), only for what the language
 * supports: Ctrl+U is not taken in Markdown (no underline), so the key stays CodeMirror's.
 */
export function formatHandlers(
  lang: Lang,
  run: (command: StateCommand) => boolean,
): Record<string, () => boolean> {
  return Object.fromEntries(
    FORMAT_FEATURES.filter((f) => supports(lang, f)).map((f) => [
      `format.${f}`,
      () => run(COMMANDS[f](lang)),
    ]),
  );
}

/**
 * The text editor surface's handlers (`text.editor`): the formatting commands of its language,
 * and undo and redo as the toolbar or the palette ask for them (the keys stay the editor's own,
 * `delegated: widget`). `undoOutside` takes back an outside edit just taken in first.
 */
export function textHandlers(
  lang: Lang,
  run: (command: StateCommand) => boolean,
  undoOutside: () => boolean = () => false,
): Record<string, () => boolean> {
  return {
    ...formatHandlers(lang, run),
    "app.undo": () => undoOutside() || run(undo),
    "app.redo": () => run(redo),
  };
}
