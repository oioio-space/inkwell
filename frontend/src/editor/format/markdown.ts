import {
  codeSpec,
  Emphasis,
  enclosingLink,
  type LangSyntax,
  type LineClass,
  type LineScan,
  looksLikeUrl,
  type MarkSpec,
  runSpec,
  type Span,
  verbatimEnd,
} from "./syntax";
import { leadRun } from "./text";
import type { Block, InsertItem, Mark } from "./types";

/** A link destination: angle brackets when it holds spaces or parentheses. */
export const markdownDest = (url: string): string =>
  /[\s()<>]/.test(url) ? `<${url.replace(/[<>\\]/g, "\\$&")}>` : url;

/** Text for a link label or image alt: brackets and backslashes escaped. */
export const markdownLabel = (s: string): string => s.replace(/[[\]\\]/g, "\\$&");

function matchBracket(line: string, open: number): number {
  let depth = 0;
  for (let i = open; i < line.length; i++) {
    const ch = line[i];
    if (ch === "\\") i++;
    else if (ch === "[") depth++;
    else if (ch === "]" && --depth === 0) return i;
  }
  return -1;
}

function scan(line: string): LineScan {
  const emphasis = new Emphasis(line, {
    "*": { m1: "italic", m2: "bold" },
    _: { m1: "italic", m2: "bold", wordSafe: true },
    "~": { m2: "strike" },
  });
  const verbatim: [number, number][] = [];
  const links: Span[] = [];
  const skip = new Map<number, number>();
  for (let i = 0; i < line.length; ) {
    const jump = skip.get(i);
    if (jump !== undefined) {
      i = jump;
      continue;
    }
    const ch = line[i];
    if (ch === "\\") {
      i += 2;
    } else if (ch === "`" || ch === "$") {
      const end = verbatimEnd(line, i);
      if (end < 0) {
        i += ch === "`" ? leadRun(line.slice(i), "`") : 1;
        continue;
      }
      verbatim.push([i, end]);
      i = end;
    } else if (ch === "*" || ch === "_" || ch === "~") {
      const n = leadRun(line.slice(i), ch);
      emphasis.run(ch, i, n);
      i += n;
    } else if (ch === "[") {
      const close = matchBracket(line, i);
      const end = close >= 0 && line[close + 1] === "(" ? line.indexOf(")", close + 2) : -1;
      if (end >= 0 && line[i - 1] !== "!") {
        links.push({ mark: "link", open: [i, i + 1], close: [close, end + 1] });
        skip.set(close, end + 1);
      }
      i++;
    } else {
      i++;
    }
  }
  return { spans: [...emphasis.spans, ...links], verbatim };
}

const NONE = { lead: 0, trail: 0 };
const LINK_TAIL = /\]\([^)]*\)$/;

const link: MarkSpec = {
  peel(core) {
    const tail = LINK_TAIL.exec(core);
    return core.startsWith("[") && tail && core.length >= 1 + tail[0].length
      ? { lead: 1, trail: tail[0].length }
      : NONE;
  },
  detect: enclosingLink,
  apply(core) {
    if (looksLikeUrl(core)) return { prefix: "[", suffix: `](${markdownDest(core)})` };
    return { prefix: "[", suffix: "]()", sel: [core.length + 3, core.length + 3] };
  },
};

const marks: Partial<Record<Mark, MarkSpec>> = {
  bold: runSpec("*", 2, false, /\*\*([^*\n]+?)\*\*/g),
  italic: runSpec("*", 1, true, /(?<!\*)\*([^*\n]+?)\*(?!\*)/g),
  strike: runSpec("~", 2, false, /~~([^~\n]+?)~~/g),
  code: codeSpec,
  link,
};

function classify(line: string): LineClass {
  const indent = /^[ \t]*/.exec(line)?.[0].length ?? 0;
  const rest = line.slice(indent);
  const at = (block: Block, len: number): LineClass => ({
    block,
    indent,
    prefixEnd: indent + len,
    suffix: 0,
  });
  const heading = /^(#{1,6})[ \t]+/.exec(rest);
  if (heading) return at(`h${Math.min(heading[1]?.length ?? 1, 4)}` as Block, heading[0].length);
  const task = /^[-*+][ \t]+\[[ xX]\][ \t]+/.exec(rest);
  if (task) return at("task", task[0].length);
  const ul = /^[-*+][ \t]+/.exec(rest);
  if (ul) return at("ul", ul[0].length);
  const ol = /^\d+[.)][ \t]+/.exec(rest);
  if (ol) return at("ol", ol[0].length);
  const quote = /^>[ \t]?/.exec(rest);
  if (quote) return at("quote", quote[0].length);
  return at("paragraph", 0);
}

const BLOCKS: Block[] = [
  "h1",
  "h2",
  "h3",
  "h4",
  "paragraph",
  "ul",
  "ol",
  "task",
  "quote",
  "codeblock",
];
const INSERTS: InsertItem[] = ["image", "csvTable", "footnote", "link", "table", "formula"];

export const markdown: LangSyntax = {
  marks,
  blocks: new Set(BLOCKS),
  inserts: new Set(INSERTS),
  scan,
  classify,
  prefix(block, n) {
    switch (block) {
      case "h1":
      case "h2":
      case "h3":
      case "h4":
        return `${"#".repeat(Number(block[1]))} `;
      case "ul":
        return "- ";
      case "ol":
        return `${n}. `;
      case "task":
        return "- [ ] ";
      case "quote":
        return "> ";
      default:
        return "";
    }
  },
};
