import type { StateCommand } from "@codemirror/state";
import {
  type Block,
  type InsertItem,
  insert,
  type Lang,
  langOf,
  type Mark,
  setBlock,
  supports,
  toggleMark,
} from "../../editor/format";
import type { MessageKey } from "../../i18n";
import type { IconName } from "../icons/paths";

/** What the formatting bar and its overflow menu know about each button. */
export type BarId =
  | "bold"
  | "italic"
  | "underline"
  | "strike"
  | "link"
  | "code"
  | "codeblock"
  | "formula"
  | "ul"
  | "ol"
  | "task"
  | "quote"
  | "table";

export interface BarItem {
  id: BarId;
  label: MessageKey;
  /** A symbolic icon, or a typeset glyph (B, I, U, S, ∑) shown as in the mockup. */
  icon?: IconName;
  glyph?: string;
  /** Visual separation: items of one group sit together. */
  group: number;
  /** The shortcut as the keymap spells it (`Mod-Shift-x`); the label is built from it. */
  key?: string;
  syntax: Partial<Record<Exclude<Lang, "plain">, string>>;
  /** Whether the item shows pressed when `activeMarks`/`activeBlock` say so. */
  toggle?: { mark: Mark } | { block: Block };
  command: (lang: Lang) => StateCommand;
}

export const BAR_ITEMS: readonly BarItem[] = [
  {
    id: "bold",
    label: "fmt.bold",
    glyph: "B",
    group: 1,
    key: "Mod-b",
    syntax: { typst: "*texte*", markdown: "**texte**" },
    toggle: { mark: "bold" },
    command: (l) => toggleMark(l, "bold"),
  },
  {
    id: "italic",
    label: "fmt.italic",
    glyph: "I",
    group: 1,
    key: "Mod-i",
    syntax: { typst: "_texte_", markdown: "*texte*" },
    toggle: { mark: "italic" },
    command: (l) => toggleMark(l, "italic"),
  },
  {
    id: "underline",
    label: "fmt.underline",
    glyph: "U",
    group: 1,
    key: "Mod-u",
    syntax: { typst: "#underline[texte]" },
    toggle: { mark: "underline" },
    command: (l) => toggleMark(l, "underline"),
  },
  {
    id: "strike",
    label: "fmt.strike",
    glyph: "S",
    group: 1,
    key: "Mod-Shift-x",
    syntax: { typst: "#strike[texte]", markdown: "~~texte~~" },
    toggle: { mark: "strike" },
    command: (l) => toggleMark(l, "strike"),
  },
  {
    id: "link",
    label: "fmt.link",
    icon: "link",
    group: 2,
    key: "Mod-k",
    syntax: { typst: '#link("url")[texte]', markdown: "[texte](url)" },
    toggle: { mark: "link" },
    command: (l) => toggleMark(l, "link"),
  },
  {
    id: "code",
    label: "fmt.code",
    icon: "code",
    group: 2,
    key: "Mod-e",
    syntax: { typst: "`texte`", markdown: "`texte`" },
    toggle: { mark: "code" },
    command: (l) => toggleMark(l, "code"),
  },
  {
    id: "codeblock",
    label: "fmt.codeblock",
    icon: "codeblock",
    group: 2,
    key: "Mod-Shift-e",
    syntax: { typst: "```bash … ```", markdown: "```bash … ```" },
    toggle: { block: "codeblock" },
    command: (l) => setBlock(l, "codeblock"),
  },
  {
    id: "formula",
    label: "fmt.formula",
    glyph: "∑",
    group: 2,
    key: "Mod-m",
    syntax: { typst: "$ x^2 $", markdown: "$x^2$" },
    command: (l) => insert(l, "formula"),
  },
  {
    id: "ul",
    label: "fmt.ul",
    icon: "ul",
    group: 3,
    syntax: { typst: "- texte", markdown: "- texte" },
    toggle: { block: "ul" },
    command: (l) => setBlock(l, "ul"),
  },
  {
    id: "ol",
    label: "fmt.ol",
    icon: "ol",
    group: 3,
    syntax: { typst: "+ texte", markdown: "1. texte" },
    toggle: { block: "ol" },
    command: (l) => setBlock(l, "ol"),
  },
  {
    id: "task",
    label: "fmt.task",
    icon: "task",
    group: 3,
    syntax: { markdown: "- [ ] texte" },
    toggle: { block: "task" },
    command: (l) => setBlock(l, "task"),
  },
  {
    id: "quote",
    label: "fmt.quote",
    icon: "quote",
    group: 3,
    syntax: { typst: "#quote[texte]", markdown: "> texte" },
    toggle: { block: "quote" },
    command: (l) => setBlock(l, "quote"),
  },
  {
    id: "table",
    label: "fmt.table",
    icon: "table",
    group: 4,
    syntax: { typst: "#table(columns: 2, …)", markdown: "| a | b |" },
    command: (l) => insert(l, "table"),
  },
];

/** The feature `supports()` is asked about for each button (the formula is an insertion). */
const FEATURE: Record<BarId, Mark | Block | InsertItem> = {
  bold: "bold",
  italic: "italic",
  underline: "underline",
  strike: "strike",
  link: "link",
  code: "code",
  codeblock: "codeblock",
  formula: "formula",
  ul: "ul",
  ol: "ol",
  task: "task",
  quote: "quote",
  table: "table",
};

/** The buttons a language's bar shows: only what the format can write. */
export function itemsFor(lang: Lang): BarItem[] {
  return BAR_ITEMS.filter((item) => supports(lang, FEATURE[item.id]));
}

export const HEADINGS = ["h1", "h2", "h3", "h4"] as const;

/** The syntax shown beside a heading level in the menu. */
export const HEADING_SYNTAX: Record<
  Exclude<Lang, "plain">,
  Record<(typeof HEADINGS)[number], string>
> = {
  typst: { h1: "=", h2: "==", h3: "===", h4: "====" },
  markdown: { h1: "#", h2: "##", h3: "###", h4: "####" },
};

const isMac = () => typeof navigator !== "undefined" && /mac|iphone|ipad/i.test(navigator.platform);

/** `Mod-Shift-x` as people read it: "Ctrl+⇧+X" (⌘ on a Mac). */
export function shortcutLabel(key: string): string {
  return key
    .split("-")
    .map((part) => {
      if (part === "Mod") return isMac() ? "⌘" : "Ctrl";
      if (part === "Shift") return "⇧";
      return part.length === 1 ? part.toUpperCase() : part;
    })
    .join("+");
}

/** The language of a file that has a formatting bar; csv, bib, txt and the like have none. */
export function barLang(path: string): Exclude<Lang, "plain"> | undefined {
  const lang = langOf(path);
  return lang === "plain" ? undefined : lang;
}
