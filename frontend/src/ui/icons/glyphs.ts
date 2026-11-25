/**
 * Icons of the set drawn as a typeset letter rather than a path (B, I, U, S, ∑ of the mockup):
 * a command names them as any icon (`format.bold` is `glyph-bold`), so no bar keeps a table.
 */
const GLYPHS: Readonly<Record<string, { letter: string; style: string }>> = {
  "glyph-bold": { letter: "B", style: "g-bold" },
  "glyph-italic": { letter: "I", style: "g-italic" },
  "glyph-underline": { letter: "U", style: "g-underline" },
  "glyph-strike": { letter: "S", style: "g-strike" },
  "glyph-formula": { letter: "∑", style: "g-formula" },
};

/** A glyph icon's letter and its typeset style, or undefined for a path icon. */
export const glyphOf = (icon: string | undefined): { letter: string; style: string } | undefined =>
  icon === undefined ? undefined : GLYPHS[icon];
