import type { EditorState, Text } from "@codemirror/state";
import type { Edit, FormatModule, RenderContext, Rendered } from "../../../editor/format/lang";
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
  pairSpec,
  runSpec,
  type Span,
  verbatimEnd,
} from "../../../editor/format/syntax";
import type { Block, InsertArgs, InsertItem, Mark } from "../../../editor/format/types";
import { stripTypst } from "../../../editor/stats/words";

// Typst's format entry (ADR 0027, EX0c3): its syntax tables, the empty pairs the convergence
// cleanup removes, its quote call, its word stripping and what each Insert item writes.

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

const syntax: LangSyntax = {
  hints: {
    bold: "*texte*",
    italic: "_texte_",
    underline: "#underline[texte]",
    strike: "#strike[texte]",
    link: '#link("url")[texte]',
    code: "`texte`",
    codeblock: "```bash … ```",
    formula: "$ x^2 $",
    ul: "- texte",
    ol: "+ texte",
    quote: "#quote[texte]",
    table: "#table(columns: 2, …)",
    h1: "=",
    h2: "==",
    h3: "===",
    h4: "====",
  },
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

// ── Insert items ────────────────────────────────────────────────────────────

const typstContent = (s: string) => s.replace(/[\\[\]#$*_`@<~]/g, "\\$&");

/** A data loader: the language's reader of the picked file's type (`readers`), else `read`. */
function dataLoader(path: string, name: string | undefined, reader: string): string {
  const base = path.slice(path.lastIndexOf("/") + 1).replace(/\.[^.]*$/, "");
  const id = (name ?? base).replace(/[^A-Za-z0-9_-]/g, "-").replace(/^(?=[0-9-])/, "data-");
  return `#let ${id || "data"} = ${reader}("${typstString(path)}")`;
}

function tableCells(cols: number, rows: number): string {
  const row = `  ${Array<string>(cols).fill("[]").join(", ")},`;
  return `#table(\n  columns: ${cols},\n${Array<string>(rows).fill(row).join("\n")}\n)`;
}

function render(item: InsertItem, a: InsertArgs, ctx: RenderContext): Rendered | null {
  const { selected, rel } = ctx;
  const str = typstString(rel);
  const own = (text: string, cursor?: number): Rendered => ({
    text,
    block: true,
    ...(cursor === undefined ? {} : { cursor }),
  });
  const inline = (text: string, cursor?: number): Rendered => ({
    text,
    block: false,
    ...(cursor === undefined ? {} : { cursor }),
  });
  switch (item) {
    case "image":
      if (!a.path) return null;
      return own(a.text ? `#image("${str}", alt: "${typstString(a.text)}")` : `#image("${str}")`);
    case "figure": {
      if (!a.path) return null;
      const head = `#figure(image("${str}"), caption: [`;
      const caption = a.text ? typstContent(a.text) : "";
      return own(`${head}${caption}])`, a.text ? undefined : head.length);
    }
    case "include":
      return a.path ? own(`#include "${str}"`) : null;
    case "attach": {
      if (!a.path) return null;
      const head = `#pdf.attach("${str}", description: "`;
      const description = typstString(a.description ?? "");
      return own(`${head}${description}")`, a.description ? undefined : head.length);
    }
    case "csvTable":
      if (!a.path) return null;
      return own(
        a.cols
          ? `#table(columns: ${a.cols}, ..csv("${str}").flatten())`
          : `#table(..csv("${str}").flatten())`,
      );
    case "data":
      return a.path ? own(dataLoader(rel, a.name, ctx.reader)) : null;
    case "bibliography":
      return a.path ? own(`#bibliography("${str}")`) : null;
    case "cite":
    case "ref":
      return a.key ? inline(`@${a.key}`) : null;
    case "footnote": {
      const text = a.text ? typstContent(a.text) : "";
      return inline(`#footnote[${text}]`, a.text ? undefined : "#footnote[".length);
    }
    case "link": {
      const label = a.text ?? selected;
      if (a.url)
        return inline(
          label
            ? `#link("${typstString(a.url)}")[${typstContent(label)}]`
            : `#link("${typstString(a.url)}")`,
        );
      return inline(`#link("")[${typstContent(label)}]`, '#link("'.length);
    }
    case "outline":
      return own("#outline()");
    case "pagebreak":
      return own("#pagebreak()");
    case "table": {
      const text = tableCells(a.cols ?? 2, a.rowCount ?? 2);
      return own(text, text.indexOf("[]") + 1);
    }
    case "formula":
      return inline(`$${selected}$`, selected ? undefined : 1);
    default:
      return null;
  }
}

// ── Quotes ──────────────────────────────────────────────────────────────────

/** Position of the `]` that closes the bracket opened just before `from` on line `lineNo`. */
function closingBracket(doc: Text, lineNo: number, from: number): number {
  let depth = 1;
  for (let n = lineNo; n <= doc.lines; n++) {
    const line = doc.line(n);
    const start = n === lineNo ? from - line.from : 0;
    for (let i = start; i < line.text.length; i++) {
      const ch = line.text[i];
      if (ch === "\\") i++;
      else if (ch === "[") depth++;
      else if (ch === "]" && --depth === 0) return line.from + i;
    }
  }
  return -1;
}

/** Typst quotes are one `#quote[…]` around a run of consecutive lines. */
function wrapQuotes(state: EditorState, lines: readonly number[], edits: Edit[]): void {
  const runs: number[][] = [];
  for (const n of lines) {
    const run = runs.at(-1);
    if (run && run.at(-1) === n - 1) run.push(n);
    else runs.push([n]);
  }
  for (const run of runs) {
    const first = state.doc.line(run[0] ?? 0);
    const last = state.doc.line(run.at(-1) ?? 0);
    const indent = /^[ \t]*/.exec(first.text)?.[0].length ?? 0;
    edits.push({ from: first.from + indent, to: first.from + indent, insert: "#quote[" });
    edits.push({ from: last.to, to: last.to, insert: "]" });
  }
}

const typst: FormatModule = {
  syntax,
  // Empty pairs only where unambiguous (see editor/format/cleanup.ts).
  emptyPairs: [
    /#(?:underline|strike|strong|emph)\[\]/g,
    /(?<!\\)\*\*/g,
    /(?<!\\)__/g,
    /(?<!`)``(?!`)/g,
  ],
  quote: { wrap: wrapQuotes, closing: closingBracket },
  strip: stripTypst,
  render,
  precondition: (item, args) =>
    item === "cite" && !args.hasBibliography ? { offer: "bibliography" } : null,
  insertSyntax: {
    image: "#image()",
    figure: "#figure()",
    include: "#include",
    attach: "#pdf.attach()",
    csvTable: "csv()",
    data: "json()",
    bibliography: "#bibliography()",
    link: "[](…)",
    cite: "@clé",
    ref: "@étiquette",
    footnote: "#footnote[]",
    outline: "#outline()",
    pagebreak: "#pagebreak()",
  },
};

export default typst;
