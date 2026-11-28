import type { Lang } from "./lang";
import type { Block, Feature, InsertItem } from "./types";

/** Whether the format can do this: the bar only shows what the file's format knows. */
export function supports(lang: Lang | undefined, feature: Feature): boolean {
  const syntax = lang?.syntax;
  if (!syntax) return false;
  return (
    feature in syntax.marks ||
    syntax.blocks.has(feature as Block) ||
    syntax.inserts.has(feature as InsertItem)
  );
}

/** A fence line opens or closes a fenced code block (Typst raw block, Markdown code block). */
export const isFence = (text: string): boolean => /^[ \t]{0,3}```/.test(text);

/** What a feature writes in a language, shown beside its button or heading level. */
export const hintOf = (lang: Lang | undefined, feature: string): string | undefined =>
  lang?.syntax.hints[feature as Feature];
