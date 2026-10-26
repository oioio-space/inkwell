import type { Anchor } from "./protocol";

/**
 * The jump between the source and the preview, by content.
 *
 * Typst's command line gives no map from the source to the page. What it can say, reliably, is
 * where each heading and paragraph of the laid-out document landed and what it says (Go asks
 * `typst eval`, see typst.Engine.Anchors). This module reads the same headings and paragraphs
 * out of the source text and pairs them in order by their words. A jump is therefore exact to the
 * paragraph: it lands on the paragraph or heading the caret is in (or the one above), and a click
 * on a page lands on the paragraph or heading it hits. Code, `#set` rules and the like have no
 * words in the page and are skipped.
 */

export interface SourceBlock {
  /** 1-based first line of the heading or paragraph. */
  line: number;
  heading: boolean;
  /** Its words, reduced to lowercase letters and digits. */
  key: string;
}

/** A source block paired with the anchor of the laid-out document that shows it. */
export interface Pair {
  line: number;
  anchor: Anchor;
}

/** How many characters of a text are compared: enough to tell paragraphs apart, few enough to survive markup in the middle. */
const KEY = 28;
/** How many blocks ahead an anchor looks for its block before it is given up as generated text. */
const LOOKAHEAD = 60;
const MIN_KEY = 3;

/** Lowercase letters and digits only: markup, punctuation, quotes and spaces do not matter. */
export function normalize(text: string): string {
  return text.toLowerCase().replace(/[^\p{L}\p{N}]+/gu, "");
}

const HEADING = /^\s*(=+)\s+(.*)$/;
const STATEMENT = /^#(set|show|let|import|include|pagebreak|colbreak|bibliography|context)\b/;

function stripLine(line: string): string {
  return line
    .replace(/\/\/.*$/, "") // line comment
    .replace(/<[\w:.-]+>\s*$/, "") // a trailing label
    .replace(/^\s*(?:[-+]|\d+\.)\s+/, "") // a list marker
    .replace(/^\s*\/\s+/, "") // a term-list marker
    .replace(/#[a-zA-Z_][\w.-]*(?:\([^)]*\))?/g, " "); // #func(...) calls: no words in the page
}

/** The headings and paragraphs of a Typst or Markdown source, in order. */
export function sourceBlocks(source: string): SourceBlock[] {
  const blocks: SourceBlock[] = [];
  const lines = source.split(/\r\n|\r|\n/);
  let fence: string | undefined;
  let comment = false;
  let para: { line: number; text: string[] } | undefined;
  const flush = () => {
    if (para) {
      const key = normalize(para.text.join(" ")).slice(0, KEY);
      if (key.length >= MIN_KEY) blocks.push({ line: para.line, heading: false, key });
    }
    para = undefined;
  };
  lines.forEach((raw, i) => {
    const line = i + 1;
    const trimmed = raw.trim();
    const fenceMark = trimmed.match(/^(`{3,}|~{3,})/)?.[1];
    if (fence) {
      if (trimmed === fenceMark && fenceMark.length >= fence.length && fenceMark[0] === fence[0]) {
        fence = undefined;
      }
      return;
    }
    if (fenceMark) {
      flush();
      fence = fenceMark;
      return;
    }
    if (comment) {
      if (trimmed.includes("*/")) comment = false;
      return;
    }
    if (trimmed.startsWith("/*") && !trimmed.includes("*/")) {
      flush();
      comment = true;
      return;
    }
    if (trimmed === "") {
      flush();
      return;
    }
    if (STATEMENT.test(trimmed)) {
      flush(); // a rule or an import: no words in the page
      return;
    }
    const h = raw.match(HEADING);
    if (h?.[2] !== undefined) {
      flush();
      const key = normalize(stripLine(h[2])).slice(0, KEY);
      if (key.length >= MIN_KEY) blocks.push({ line, heading: true, key });
      return;
    }
    const md = trimmed.match(/^#{1,6}\s+(.*)$/); // Markdown heading
    if (md?.[1] !== undefined && !/^#[a-zA-Z_]/.test(trimmed)) {
      flush();
      const key = normalize(md[1]).slice(0, KEY);
      if (key.length >= MIN_KEY) blocks.push({ line, heading: true, key });
      return;
    }
    const text = stripLine(raw);
    para ??= { line, text: [] };
    para.text.push(text);
  });
  flush();
  return blocks;
}

const keyOf = (a: Anchor): string => normalize(a.text).slice(0, KEY);

const matches = (block: string, anchor: string): boolean => {
  const n = Math.min(block.length, anchor.length);
  return n >= MIN_KEY && block.slice(0, n) === anchor.slice(0, n);
};

/**
 * Pairs the anchors of the document with the blocks of the source, in order. An anchor that no
 * block ahead explains (text made by code, a title page, a bibliography) is skipped; so is a
 * block that no anchor explains.
 */
export function align(blocks: readonly SourceBlock[], anchors: readonly Anchor[]): Pair[] {
  const pairs: Pair[] = [];
  let at = 0;
  for (const anchor of anchors) {
    const key = keyOf(anchor);
    if (key.length < MIN_KEY) continue;
    for (let j = at; j < Math.min(blocks.length, at + LOOKAHEAD); j++) {
      const block = blocks[j];
      if (block && block.heading === anchor.heading && matches(block.key, key)) {
        pairs.push({ line: block.line, anchor });
        at = j + 1;
        break;
      }
    }
  }
  return pairs;
}

/** The pairs for a source and the anchors of its document. */
export const pairUp = (source: string, anchors: readonly Anchor[]): Pair[] =>
  align(sourceBlocks(source), anchors);

/**
 * The line of the heading or paragraph at `y` points of `page`: the last one that starts at or
 * above the point; for a point above every block of the page, the first block of the page.
 */
export function lineAt(pairs: readonly Pair[], page: number, y: number): number | undefined {
  const onPage = pairs.filter((p) => p.anchor.page === page);
  if (onPage.length === 0) return undefined;
  let hit = onPage[0];
  for (const p of onPage) if (p.anchor.y <= y + 1) hit = p;
  return hit?.line;
}

/** Where the paragraph or heading at (or above) `line` is in the preview. */
export function placeOf(
  pairs: readonly Pair[],
  line: number,
): { page: number; y: number } | undefined {
  if (pairs.length === 0) return undefined;
  let hit = pairs[0];
  for (const p of pairs) if (p.line <= line) hit = p;
  return hit ? { page: hit.anchor.page, y: hit.anchor.y } : undefined;
}
