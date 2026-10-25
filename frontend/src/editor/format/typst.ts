import {
  codeSpec,
  Emphasis,
  enclosingLink,
  type LangSyntax,
  type LineClass,
  type LineScan,
  looksLikeUrl,
  type MarkSpec,
  pairSpec,
  runSpec,
  type Span,
  verbatimEnd,
} from "./syntax";
import type { Block, InsertItem, Mark } from "./types";

/** A Typst string literal body: quotes, backslashes and line breaks escaped. */
export const typstString = (s: string): string =>
  s.replace(
    /[\\"\n\r\t]/g,
    (c) => ({ "\\": "\\\\", '"': '\\"', "\n": "\\n", "\r": "\\r", "\t": "\\t" })[c] ?? c,
  );

const LINK_OPEN = /#link\("(?:[^"\\]|\\.)*"\)\[/y;
const BRACKET_CALLS: [RegExp, Mark][] = [
  [/#underline\[/y, "underline"],
  [/#strike\[/y, "strike"],
  [/#strong\[/y, "bold"],
  [/#emph\[/y, "italic"],
  [LINK_OPEN, "link"],
];

function scan(line: string): LineScan {
  const emphasis = new Emphasis(line, { "*": { m1: "bold" }, _: { m1: "italic", wordSafe: true } });
  const verbatim: [number, number][] = [];
  const calls: Span[] = [];
  const open: { mark?: Mark; at: [number, number] }[] = [];
  for (let i = 0; i < line.length; ) {
    const ch = line[i];
    if (ch === "\\") {
      i += 2;
    } else if (ch === "`" || ch === "$") {
      const end = verbatimEnd(line, i);
      if (end < 0) {
        i++;
        continue;
      }
      verbatim.push([i, end]);
      i = end;
    } else if (ch === "*" || ch === "_") {
      emphasis.run(ch, i, 1);
      i++;
    } else if (ch === "#") {
      let matched = false;
      for (const [re, mark] of BRACKET_CALLS) {
        re.lastIndex = i;
        const m = re.exec(line);
        if (!m) continue;
        open.push({ mark, at: [i, i + m[0].length] });
        i += m[0].length;
        matched = true;
        break;
      }
      if (!matched) i++;
    } else if (ch === "[") {
      open.push({ at: [i, i + 1] });
      i++;
    } else if (ch === "]") {
      const top = open.pop();
      if (top?.mark) calls.push({ mark: top.mark, open: top.at, close: [i, i + 1] });
      i++;
    } else {
      i++;
    }
  }
  return { spans: [...emphasis.spans, ...calls], verbatim };
}

const NONE = { lead: 0, trail: 0 };
const LINK_PREFIX = /^#link\("(?:[^"\\]|\\.)*"\)\[/;

const link: MarkSpec = {
  peel(core) {
    const m = LINK_PREFIX.exec(core);
    return m && core.length > m[0].length && core.endsWith("]")
      ? { lead: m[0].length, trail: 1 }
      : NONE;
  },
  detect(ctx) {
    const enclosing = enclosingLink(ctx);
    if (enclosing) return enclosing;
    const { line, from, to } = ctx;
    // The bare form `#link("url")`, with the URL as the marked text.
    if (!line.slice(0, from).endsWith('#link("')) return null;
    const after = line.slice(to);
    if (!after.startsWith('")') || after[2] === "[") return null;
    return {
      del: [
        [-7, 0],
        [to - from, to - from + 2],
      ],
    };
  },
  apply(core) {
    if (looksLikeUrl(core)) {
      const escaped = typstString(core);
      return { prefix: '#link("', suffix: '")', ...(escaped === core ? {} : { core: escaped }) };
    }
    return { prefix: '#link("")[', suffix: "]", sel: [7, 7] };
  },
};

const marks: Partial<Record<Mark, MarkSpec>> = {
  bold: runSpec("*", 1, true, /\*([^*\n]+?)\*/g),
  italic: runSpec("_", 1, true, /_([^_\n]+?)_/g),
  underline: pairSpec("#underline[", "]", /#underline\[([^[\]]*)\]/g),
  strike: pairSpec("#strike[", "]", /#strike\[([^[\]]*)\]/g),
  code: codeSpec,
  link,
};

function classify(line: string): LineClass {
  const indent = /^[ \t]*/.exec(line)?.[0].length ?? 0;
  const rest = line.slice(indent);
  const at = (block: Block, len: number, suffix = 0): LineClass => ({
    block,
    indent,
    prefixEnd: indent + len,
    suffix,
  });
  const heading = /^(=+)[ \t]+/.exec(rest);
  if (heading) return at(`h${Math.min(heading[1]?.length ?? 1, 4)}` as Block, heading[0].length);
  const call = /^#heading\(\s*level:\s*(\d+)\s*\)\[/.exec(rest);
  if (call && rest.endsWith("]")) {
    return at(`h${Math.min(Number(call[1]), 4)}` as Block, call[0].length, 1);
  }
  const quote = /^#quote\[/.exec(rest);
  if (quote) return at("quote", quote[0].length);
  const ul = /^-[ \t]+/.exec(rest);
  if (ul) return at("ul", ul[0].length);
  const ol = /^(?:\+|\d+\.)[ \t]+/.exec(rest);
  if (ol) return at("ol", ol[0].length);
  return at("paragraph", 0);
}

const BLOCKS: Block[] = ["h1", "h2", "h3", "h4", "paragraph", "ul", "ol", "quote", "codeblock"];
const INSERTS: InsertItem[] = [
  "image",
  "figure",
  "include",
  "attach",
  "csvTable",
  "data",
  "bibliography",
  "cite",
  "ref",
  "footnote",
  "link",
  "outline",
  "pagebreak",
  "table",
  "formula",
];

export const typst: LangSyntax = {
  marks,
  blocks: new Set(BLOCKS),
  inserts: new Set(INSERTS),
  scan,
  classify,
  prefix(block) {
    switch (block) {
      case "h1":
      case "h2":
      case "h3":
      case "h4":
        return `${"=".repeat(Number(block[1]))} `;
      case "ul":
        return "- ";
      case "ol":
        return "+ ";
      default:
        return "";
    }
  },
};
