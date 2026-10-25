import type { Lang } from "./lang";
import { graphemeCount } from "./position";
import { countPlainWords, stripMarkup } from "./words";

/** Average silent reading speed in French, words per minute. */
export const READING_WPM_FR = 230;

export interface BaseStats {
  words: number;
  /** Grapheme clusters, whitespace included. */
  chars: number;
  charsNoSpaces: number;
  sentences: number;
  /** Blank-line separated blocks that contain at least one word. */
  paragraphs: number;
  /** Whole minutes, rounded up; 0 for an empty text. */
  readingMinutes: number;
}

export interface Stats extends BaseStats {
  /** Statistics of the selected text only; absent when nothing is selected. */
  selection?: BaseStats;
}

export interface Selection {
  from: number;
  to: number;
}

const HAS_WORD = /[\p{L}\p{N}]/u;
const SENTENCE_END = /[.!?…]+(?:\s+|$)/u;
const PARAGRAPH_BREAK = /\n[ \t]*\n/;

function baseStats(text: string, lang: Lang, locale: string): BaseStats {
  const prose = stripMarkup(text, lang);
  const words = countPlainWords(prose, locale);
  return {
    words,
    chars: graphemeCount(text),
    charsNoSpaces: graphemeCount(text.replace(/\s+/gu, "")),
    sentences: prose.split(SENTENCE_END).filter((s) => HAS_WORD.test(s)).length,
    paragraphs: prose.split(PARAGRAPH_BREAK).filter((s) => HAS_WORD.test(s)).length,
    readingMinutes: Math.ceil(words / READING_WPM_FR),
  };
}

/**
 * Document statistics, plus those of `selection` when it is non-empty. The selection is
 * stripped of markup on its own, so a selection cutting a construct in half is approximate.
 */
export function stats(text: string, lang: Lang, selection?: Selection, locale = "fr"): Stats {
  const result: Stats = baseStats(text, lang, locale);
  if (selection && selection.to !== selection.from) {
    const from = Math.min(selection.from, selection.to);
    const to = Math.max(selection.from, selection.to);
    result.selection = baseStats(text.slice(from, to), lang, locale);
  }
  return result;
}

export type CountUnit = "mot" | "caractère" | "phrase" | "paragraphe" | "minute";

const number = new Intl.NumberFormat("fr");
const plural = new Intl.PluralRules("fr");

/** "0 mot", "1 mot", "1 284 mots": French plural (0 and 1 singular), narrow no-break thousands. */
export function formatCount(n: number, unit: CountUnit): string {
  return `${number.format(n)} ${plural.select(n) === "one" ? unit : `${unit}s`}`;
}
