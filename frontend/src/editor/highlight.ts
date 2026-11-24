import { HighlightStyle, type LanguageSupport, syntaxHighlighting } from "@codemirror/language";
import type { Extension } from "@codemirror/state";
import { tags } from "@lezer/highlight";
import { typstTags } from "codemirror-lang-typst/lezer";

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

/** A grammar's parser with the colours, ready to put in a Compartment; nothing for plain text. */
export function withPalette(support: LanguageSupport | undefined): Extension {
  return support ? [support, palette] : [];
}
