import type { Lang } from "../format/lang";

/**
 * Word counting. Markup is stripped per language, then words are counted: a token is a
 * run of letters/digits/marks, joined across `-`, `'`, `’` and across a decimal `.`/`,`
 * between digits ("peut-être", "aujourd'hui", "3,14" = 1 word). Scripts written without
 * spaces (CJK, Thai, …) are delegated to `Intl.Segmenter` (`isWordLike` segments only);
 * everything else takes a regex fast path with the same result at a fraction of the cost
 * (needed for the 5 MB budget, STA-12).
 */

const TOKEN = /[\p{L}\p{N}\p{M}]+(?:(?:['’-]|(?<=\p{N})[.,](?=\p{N}))[\p{L}\p{N}\p{M}]+)*/gu;
const UNSPACED =
  /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Thai}\p{Script=Lao}\p{Script=Khmer}\p{Script=Myanmar}]/u;

const segmenters = new Map<string, Intl.Segmenter>();
function segmenter(locale: string): Intl.Segmenter {
  let s = segmenters.get(locale);
  if (!s) {
    s = new Intl.Segmenter(locale, { granularity: "word" });
    segmenters.set(locale, s);
  }
  return s;
}

/** Counts words in already-stripped text. */
export function countPlainWords(text: string, locale = "fr"): number {
  let n = 0;
  for (const m of text.matchAll(TOKEN)) {
    if (!UNSPACED.test(m[0])) {
      n++;
      continue;
    }
    for (const seg of segmenter(locale).segment(m[0])) if (seg.isWordLike) n++;
  }
  return n;
}

const TYPST_KEYWORDS = new Set(["set", "show", "let", "import", "include"]);
const IDENT_START = /[A-Za-z_]/;
const IDENT_CHAR = /[\w-]/;
const SPECIAL = /[#$`/<@]/g;

/** Index just after the group opened at `i` (balanced, string-aware), or text end. */
function skipBalanced(s: string, i: number): number {
  const open = s[i];
  const close = open === "(" ? ")" : "}";
  let depth = 0;
  for (; i < s.length; i++) {
    const c = s[i];
    if (c === "\\") i++;
    else if (c === '"') {
      for (i++; i < s.length && s[i] !== '"'; i++) if (s[i] === "\\") i++;
    } else if (c === open) depth++;
    else if (c === close && --depth === 0) return i + 1;
  }
  return s.length;
}

function endOfLine(s: string, i: number): number {
  const j = s.indexOf("\n", i);
  return j < 0 ? s.length : j;
}

/**
 * Removes Typst non-prose: comments, raw text, math, labels, references and code
 * (`#func(args)`, `#set/#show/#let/#import …`). Text inside content brackets
 * `#f[…]` / `#f(..)[…]` is kept.
 */
export function stripTypst(src: string): string {
  let out = "";
  let pos = 0;
  SPECIAL.lastIndex = 0;
  for (let m = SPECIAL.exec(src); m; m = SPECIAL.exec(src)) {
    const i = m.index;
    const c = src[i];
    let end = -1; // end of the construct to drop; -1 = literal character
    if (c === "/") {
      if (src[i + 1] === "/" && src[i - 1] !== ":") end = endOfLine(src, i);
      else if (src[i + 1] === "*") {
        const j = src.indexOf("*/", i + 2);
        end = j < 0 ? src.length : j + 2;
      }
    } else if (c === "`") {
      let n = 1;
      while (src[i + n] === "`") n++;
      const j = src.indexOf("`".repeat(n), i + n);
      if (j >= 0) end = j + n;
      else if (n >= 3) end = src.length;
      else SPECIAL.lastIndex = i + n;
    } else if (c === "$") {
      if (src[i - 1] !== "\\") {
        const j = src.indexOf("$", i + 1);
        if (j >= 0) end = j + 1;
      }
    } else if (c === "<") {
      const lm = /^<[A-Za-z_][\w:.-]*>/.exec(src.slice(i, i + 80));
      if (lm) end = i + lm[0].length;
    } else if (c === "@") {
      if (!/[\p{L}\p{N}]/u.test(src[i - 1] ?? "")) {
        const rm = /^@\w[\w:.-]*/.exec(src.slice(i, i + 80));
        if (rm) end = i + rm[0].length;
      }
    } else if (c === "#" && src[i - 1] !== "\\") {
      let j = i + 1;
      if (src[j] === "(" || src[j] === "{") end = skipBalanced(src, j);
      else if (IDENT_START.test(src[j] ?? "")) {
        while (j < src.length && IDENT_CHAR.test(src[j] as string)) j++;
        // A trailing "-" is prose punctuation, not part of the name.
        while (src[j - 1] === "-") j--;
        if (TYPST_KEYWORDS.has(src.slice(i + 1, j))) end = endOfLine(src, j);
        else {
          // field access / method chain, then call arguments.
          for (;;) {
            if (src[j] === "." && IDENT_START.test(src[j + 1] ?? "")) {
              j++;
              while (j < src.length && /\w/.test(src[j] as string)) j++;
            } else if (src[j] === "(") j = skipBalanced(src, j);
            else break;
          }
          end = j;
        }
      }
    }
    if (end < 0) continue;
    out += `${src.slice(pos, i)} `;
    pos = end;
    SPECIAL.lastIndex = end;
  }
  return out + src.slice(pos);
}

/** Removes Markdown non-prose: code, HTML comments/tags, images, link URLs, bare URLs. */
export function stripMarkdown(src: string): string {
  return src
    .replace(/^ {0,3}(`{3,}|~{3,})[^\n]*\n[\s\S]*?(?:\n {0,3}\1[`~]*[ \t]*(?=\n|$)|$)/gm, " ")
    .replace(/<!--[\s\S]*?(?:-->|$)/g, " ")
    .replace(/(`+)[^`]([\s\S]*?[^`])?\1(?!`)/g, " ")
    .replace(/!\[[^\]]*\]\([^)]*\)/g, " ")
    .replace(/!\[[^\]]*\]\[[^\]]*\]/g, " ")
    .replace(/\[([^\]]*)\]\([^)]*\)/g, "$1 ")
    .replace(/\[([^\]]*)\]\[[^\]]*\]/g, "$1 ")
    .replace(/^ {0,3}\[[^\]]+\]:[^\n]*$/gm, " ")
    .replace(/<(?:https?:|mailto:)[^>\s]*>/g, " ")
    .replace(/<\/?[A-Za-z][^>\n]*>/g, " ")
    .replace(/\bhttps?:\/\/[^\s)]+/g, " ")
    .replace(/^[ \t]*\d+[.)][ \t]+/gm, "");
}

/** Strips language markup, leaving prose only. */
export function stripMarkup(text: string, lang: Lang): string {
  if (lang === "typst") return stripTypst(text);
  if (lang === "markdown") return stripMarkdown(text);
  return text;
}

/** Number of words in `text` once `lang` markup is removed. */
export function countWords(text: string, lang: Lang, locale = "fr"): number {
  return countPlainWords(stripMarkup(text, lang), locale);
}
