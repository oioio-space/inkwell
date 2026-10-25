import type { Lang } from "./lang";
import { markdown } from "./markdown";
import type { LangSyntax } from "./syntax";
import type { Block, Feature, InsertItem } from "./types";
import { typst } from "./typst";

/** The syntax tables of a language; plain text has none. */
export function syntaxOf(lang: Lang): LangSyntax | undefined {
  switch (lang) {
    case "typst":
      return typst;
    case "markdown":
      return markdown;
    default:
      return undefined;
  }
}

/** Whether the format can do this: the bar only shows what the file's format knows. */
export function supports(lang: Lang, feature: Feature): boolean {
  const syntax = syntaxOf(lang);
  if (!syntax) return false;
  return (
    feature in syntax.marks ||
    syntax.blocks.has(feature as Block) ||
    syntax.inserts.has(feature as InsertItem)
  );
}

/** A fence line opens or closes a fenced code block (Typst raw block, Markdown code block). */
export const isFence = (text: string): boolean => /^[ \t]{0,3}```/.test(text);
