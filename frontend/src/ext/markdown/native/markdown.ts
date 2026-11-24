import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import type { LanguageDescription, LanguageSupport } from "@codemirror/language";

/**
 * GFM (tables, task lists, strikethrough); no Enter binding, the format keymap owns typing. Fenced
 * blocks are coloured by the registry's grammars, the same ones that colour code files.
 */
export function grammar(ctx: { codeLanguages: readonly LanguageDescription[] }): LanguageSupport {
  return markdown({
    base: markdownLanguage,
    addKeymap: false,
    codeLanguages: [...ctx.codeLanguages],
  });
}
