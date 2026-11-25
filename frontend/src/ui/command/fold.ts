/**
 * How the palette and the shortcuts window compare texts (docs/plans/2026-11-25-ex0b2-palette.md
 * §1, rule P-1): compatibility decomposition (« … » becomes « ... »), marks removed, lower case,
 * the ligatures spelled out, guillemets and double quotes dropped, and every word separator
 * (space ' ’ - / : ( )) one space. A folded text is its words joined by single spaces.
 */
const LIGATURES: Readonly<Record<string, string>> = { œ: "oe", æ: "ae", ß: "ss" };

/** The most of a query that is compared: 64 characters, 8 words. */
export const QUERY_CHARS = 64;
export const QUERY_WORDS = 8;

export function fold(text: string): string {
  return text
    .normalize("NFKD")
    .replace(/\p{M}+/gu, "")
    .toLowerCase()
    .replace(/[œæß]/g, (l) => LIGATURES[l] ?? l)
    .replace(/[«»"“”„‹›]/g, "")
    .replace(/[\s'’\-/:()]+/g, " ")
    .trim();
}

/** A query as it is compared: folded, then cut at 64 characters and 8 words. */
export function foldQuery(text: string): string {
  return fold(text).slice(0, QUERY_CHARS).trim().split(" ").slice(0, QUERY_WORDS).join(" ");
}
