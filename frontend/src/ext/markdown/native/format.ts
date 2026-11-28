import type { Text } from "@codemirror/state";
import type { FormatModule, RenderContext, Rendered } from "../../../editor/format/lang";
import {
  codeSpec,
  Emphasis,
  enclosingLink,
  type LangSyntax,
  type LineClass,
  type LineScan,
  looksLikeUrl,
  type MarkSpec,
  NONE,
  runSpec,
  type Span,
  verbatimEnd,
} from "../../../editor/format/syntax";
import { leadRun } from "../../../editor/format/text";
import type { Block, InsertArgs, InsertItem, Mark } from "../../../editor/format/types";
import { stripMarkdown } from "../../../editor/stats/words";

// Markdown's format entry (ADR 0027, EX0c3): its syntax tables, the empty pairs the convergence
// cleanup removes, its word stripping, what each Insert item writes and how the menu words it.

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

const syntax: LangSyntax = {
  hints: {
    bold: "**texte**",
    italic: "*texte*",
    strike: "~~texte~~",
    link: "[texte](url)",
    code: "`texte`",
    codeblock: "```bash … ```",
    formula: "$x^2$",
    ul: "- texte",
    ol: "1. texte",
    task: "- [ ] texte",
    quote: "> texte",
    table: "| a | b |",
    h1: "#",
    h2: "##",
    h3: "###",
    h4: "####",
  },
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

// ── Insert items ────────────────────────────────────────────────────────────

function csvToMarkdown(rows: readonly (readonly string[])[], source: string): string {
  const width = Math.max(...rows.map((r) => r.length));
  const cell = (s: string | undefined) => (s ?? "").replace(/\|/g, "\\|").replace(/\s*\n\s*/g, " ");
  const row = (r: readonly string[]) =>
    `| ${Array.from({ length: width }, (_, i) => cell(r[i])).join(" | ")} |`;
  const [header = [], ...rest] = rows;
  return [
    `<!-- copie figée de ${source.replace(/--/g, "- -")} : à réinsérer si le fichier change -->`,
    row(header),
    `| ${Array<string>(width).fill("---").join(" | ")} |`,
    ...rest.map(row),
  ].join("\n");
}

/** What separates a footnote definition from the end of the document: one blank line. */
function footnoteGap(doc: Text): string {
  const tail = doc.sliceString(Math.max(0, doc.length - 2));
  if (tail.endsWith("\n\n")) return "";
  if (tail.endsWith("\n")) return "\n";
  return "\n\n";
}

function markdownTable(cols: number, rows: number): string {
  const row = `|${Array<string>(cols).fill("   ").join("|")}|`;
  return [
    row,
    `|${Array<string>(cols).fill(" --- ").join("|")}|`,
    ...Array<string>(Math.max(0, rows - 1)).fill(row),
  ].join("\n");
}

function render(item: InsertItem, a: InsertArgs, ctx: RenderContext): Rendered | null {
  const { selected, rel, doc } = ctx;
  switch (item) {
    case "image": {
      if (!a.path) return null;
      return { text: `![${markdownLabel(a.text ?? selected)}](${markdownDest(rel)})`, block: true };
    }
    case "csvTable":
      if (!a.path || !a.rows || a.rows.length === 0) return null;
      return { text: csvToMarkdown(a.rows, rel), block: true };
    case "footnote": {
      const used = [...doc.toString().matchAll(/\[\^(\d+)\]/g)].map((m) => Number(m[1]));
      const n = Math.max(0, ...used) + 1;
      const ref = `[^${n}]`;
      return {
        text: ref,
        block: false,
        appendix: `${footnoteGap(doc)}${ref}: ${a.text ?? ""}`,
        caretInAppendix: !a.text,
      };
    }
    case "link": {
      const label = a.text ?? selected;
      if (a.url) return { text: `[${markdownLabel(label)}](${markdownDest(a.url)})`, block: false };
      const open = `[${markdownLabel(label)}](`;
      return { text: `${open})`, block: false, cursor: open.length };
    }
    case "table":
      return {
        text: markdownTable(a.cols ?? 2, a.rowCount ?? 2),
        block: true,
        cursor: 2,
      };
    case "formula":
      return { text: `$${selected}$`, block: false, ...(selected ? {} : { cursor: 1 }) };
    default:
      return null;
  }
}

const markdown: FormatModule = {
  syntax,
  // Markdown's empty italic `**` cannot be told from a stray literal: left alone.
  emptyPairs: [/(?<!\\)\*\*\*\*/g, /(?<!\\)~~~~/g, /(?<!`)``(?!`)/g],
  strip: stripMarkdown,
  render,
  insertSyntax: { image: "![]()", csvTable: "| a | b |", footnote: "[^1]", link: "[](…)" },
  describe: { csvTable: "insert.csvTable.markdown.desc" },
  note: "insert.markdown.note",
  // A frozen copy of a CSV is written from its rows, read when it is picked.
  needsRows: new Set(["csvTable"]),
};

export default markdown;
