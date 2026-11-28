// The built-in grammars as the app loads them through the registry (ADR 0027), at once, for the
// tests that need a language without the registry's lazy loading.
import type { LanguageSupport } from "@codemirror/language";
import type { Extension } from "@codemirror/state";
import { grammar as markdownGrammar } from "../ext/markdown/native/markdown";
import { grammar as typstGrammar } from "../ext/typst/native/typst";
import { LANGUAGE_TYPE } from "../language/client";
import type { Lang } from "./format";
import { MARKDOWN, TYPST } from "./format/testkit";
import { withPalette } from "./highlight";
import { languageOf } from "./setup";

const made = new Map<Lang | undefined, LanguageSupport | undefined>();

/** The grammar of a formatting language, one instance per language as in the app. */
export function supportFor(lang: Lang | undefined): LanguageSupport | undefined {
  if (!made.has(lang))
    made.set(
      lang,
      lang === TYPST
        ? typstGrammar()
        : lang === MARKDOWN
          ? markdownGrammar({ codeLanguages: [] })
          : undefined,
    );
  return made.get(lang);
}

/** The parser and colours of a language. */
export const languageExtension = (lang: Lang | undefined): Extension =>
  withPalette(supportFor(lang));

/** Everything that depends on the language: parser, colours, writing help (Typst's file type). */
export const forLang = (lang: Lang | undefined): Extension =>
  languageOf(supportFor(lang), lang === TYPST ? LANGUAGE_TYPE : undefined);
