// The built-in grammars as the app loads them through the registry (ADR 0027), at once, for the
// tests that need a language without the registry's lazy loading.
import type { LanguageSupport } from "@codemirror/language";
import type { Extension } from "@codemirror/state";
import { grammar as markdownGrammar } from "../ext/markdown/native/markdown";
import { grammar as typstGrammar } from "../ext/typst/native/typst";
import type { Lang } from "./format";
import { withPalette } from "./highlight";
import { languageOf } from "./setup";

const made = new Map<Lang, LanguageSupport | undefined>();

/** The grammar of a formatting language, one instance per language as in the app. */
export function supportFor(lang: Lang): LanguageSupport | undefined {
  if (!made.has(lang))
    made.set(
      lang,
      lang === "typst"
        ? typstGrammar()
        : lang === "markdown"
          ? markdownGrammar({ codeLanguages: [] })
          : undefined,
    );
  return made.get(lang);
}

/** The parser and colours of a language. */
export const languageExtension = (lang: Lang): Extension => withPalette(supportFor(lang));

/** Everything that depends on the language: parser, colours, formatting keys. */
export const forLang = (lang: Lang): Extension => languageOf(lang, supportFor(lang));
