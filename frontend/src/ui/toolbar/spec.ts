import { type Block, formatLanguage, type Lang, type Mark, supports } from "../../editor/format";
import { registry } from "../../extensions";
import type { IconName } from "../icons/paths";

/**
 * What the formatting bar and its overflow menu know about each button. Each id is also the
 * feature `supports()` is asked about (the formula and the table are insertions), and the button
 * runs the command `format.<id>` of the registry: its label's chord comes from there.
 */
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
  /** A symbolic icon, or a typeset glyph (B, I, U, S, ∑) shown as in the mockup. */
  icon?: IconName;
  glyph?: string;
  /** Visual separation: items of one group sit together. */
  group: number;
  syntax: Partial<Record<Exclude<Lang, "plain">, string>>;
  /** Whether the item shows pressed when `activeMarks`/`activeBlock` say so. */
  toggle?: { mark: Mark } | { block: Block };
}

/** The registry command a bar item runs. */
export const commandOf = (item: BarItem): string => `format.${item.id}`;

export const BAR_ITEMS: readonly BarItem[] = [
  {
    id: "bold",
    glyph: "B",
    group: 1,
    syntax: { typst: "*texte*", markdown: "**texte**" },
    toggle: { mark: "bold" },
  },
  {
    id: "italic",
    glyph: "I",
    group: 1,
    syntax: { typst: "_texte_", markdown: "*texte*" },
    toggle: { mark: "italic" },
  },
  {
    id: "underline",
    glyph: "U",
    group: 1,
    syntax: { typst: "#underline[texte]" },
    toggle: { mark: "underline" },
  },
  {
    id: "strike",
    glyph: "S",
    group: 1,
    syntax: { typst: "#strike[texte]", markdown: "~~texte~~" },
    toggle: { mark: "strike" },
  },
  {
    id: "link",
    icon: "link",
    group: 2,
    syntax: { typst: '#link("url")[texte]', markdown: "[texte](url)" },
    toggle: { mark: "link" },
  },
  {
    id: "code",
    icon: "code",
    group: 2,
    syntax: { typst: "`texte`", markdown: "`texte`" },
    toggle: { mark: "code" },
  },
  {
    id: "codeblock",
    icon: "codeblock",
    group: 2,
    syntax: { typst: "```bash … ```", markdown: "```bash … ```" },
    toggle: { block: "codeblock" },
  },
  {
    id: "formula",
    glyph: "∑",
    group: 2,
    syntax: { typst: "$ x^2 $", markdown: "$x^2$" },
  },
  {
    id: "ul",
    icon: "ul",
    group: 3,
    syntax: { typst: "- texte", markdown: "- texte" },
    toggle: { block: "ul" },
  },
  {
    id: "ol",
    icon: "ol",
    group: 3,
    syntax: { typst: "+ texte", markdown: "1. texte" },
    toggle: { block: "ol" },
  },
  {
    id: "task",
    icon: "task",
    group: 3,
    syntax: { markdown: "- [ ] texte" },
    toggle: { block: "task" },
  },
  {
    id: "quote",
    icon: "quote",
    group: 3,
    syntax: { typst: "#quote[texte]", markdown: "> texte" },
    toggle: { block: "quote" },
  },
  {
    id: "table",
    icon: "table",
    group: 4,
    syntax: { typst: "#table(columns: 2, …)", markdown: "| a | b |" },
  },
];

/** The buttons a language's bar shows: only what the format can write. */
export function itemsFor(lang: Lang): BarItem[] {
  return BAR_ITEMS.filter((item) => supports(lang, item.id));
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

/** The language of a file that has a formatting bar; csv, bib, txt and the like have none. */
export function barLang(path: string): Exclude<Lang, "plain"> | undefined {
  const lang = formatLanguage(registry.typeOf(path)?.grammar);
  return lang === "plain" ? undefined : lang;
}
