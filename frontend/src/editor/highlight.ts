import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { HighlightStyle, LanguageSupport, syntaxHighlighting } from "@codemirror/language";
import type { Extension } from "@codemirror/state";
import { tags } from "@lezer/highlight";
import { typst_lezer, typstTags } from "codemirror-lang-typst/lezer";
import type { Lang } from "./format";

/**
 * The colours are CSS variables (`--hl-*` in theme/tokens.css): one Adwaita palette, light and
 * dark, that the theme switch changes without rebuilding the editor.
 */
export const highlightStyle = HighlightStyle.define([
  { tag: tags.heading, color: "var(--hl-heading)", fontWeight: "700" },
  { tag: tags.strong, fontWeight: "700" },
  { tag: tags.emphasis, fontStyle: "italic" },
  { tag: tags.strikethrough, textDecoration: "line-through" },
  { tag: tags.comment, color: "var(--hl-comment)" },
  // `#`, field names and identifiers spliced into the text; math identifiers too.
  { tag: typstTags.interpolated, color: "var(--hl-function)" },
  { tag: tags.function(tags.variableName), color: "var(--hl-function)" },
  { tag: tags.propertyName, color: "var(--hl-function)" },
  { tag: tags.string, color: "var(--hl-string)" },
  { tag: typstTags.mathDelimiter, color: "var(--hl-string)" },
  // Labels, references and escapes: Typst maps all three to `escape`.
  { tag: tags.escape, color: "var(--hl-label)" },
  { tag: tags.keyword, color: "var(--hl-keyword)" },
  { tag: [tags.number, tags.bool], color: "var(--hl-number)" },
  { tag: [tags.link, tags.url], color: "var(--hl-link)", textDecoration: "underline" },
  { tag: [tags.monospace, tags.quote], color: "var(--hl-raw)" },
  { tag: typstTags.listMarker, color: "var(--hl-marker)" },
  { tag: [tags.processingInstruction, tags.contentSeparator], color: "var(--hl-comment)" },
  { tag: tags.invalid, color: "var(--hl-keyword)" },
]);

const palette = syntaxHighlighting(highlightStyle);

let typstSupport: LanguageSupport | undefined;
/**
 * Typst through the WASM-free Lezer parser (our CSP forbids WASM). Only the language is used:
 * its bundled colours, Enter/indent services and linter would change how typing behaves.
 */
function typstLanguage(): LanguageSupport {
  typstSupport ??= new LanguageSupport(typst_lezer().language);
  return typstSupport;
}

/** The parser and colours for a language, ready to put in a Compartment. */
export function languageExtension(lang: Lang): Extension {
  switch (lang) {
    case "typst":
      return [typstLanguage(), palette];
    case "markdown":
      // GFM (tables, task lists, strikethrough); no Enter binding, the format keymap owns typing.
      return [markdown({ base: markdownLanguage, addKeymap: false }), palette];
    default:
      return [];
  }
}
