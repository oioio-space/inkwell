import { EditorSelection, type EditorState, type StateCommand, type Text } from "@codemirror/state";
import type { Lang } from "./lang";
import { markdownDest, markdownLabel } from "./markdown";
import { syntaxOf } from "./support";
import type { InsertArgs, InsertItem } from "./types";
import { typstString } from "./typst";

// The Insert menu. Each item renders to text for the file's language; block items sit on
// their own line, inline items replace the selection.

/** `target` written relative to the directory of `currentFile` (both project paths). */
export function relativePath(currentFile: string | undefined, target: string): string {
  if (!currentFile) return target;
  const from = currentFile.split("/").slice(0, -1);
  const to = target.split("/");
  let common = 0;
  while (common < from.length && common < to.length - 1 && from[common] === to[common]) common++;
  return [...Array<string>(from.length - common).fill(".."), ...to.slice(common)].join("/");
}

interface Rendered {
  text: string;
  /** Block items get a line of their own. */
  block: boolean;
  /** Where the caret lands, in the rendered text (default: the end). */
  cursor?: number;
  /** Text appended at the end of the document (Markdown footnote definitions). */
  appendix?: string;
  /** The caret goes after the appended text rather than after the inserted one. */
  caretInAppendix?: boolean;
}

/** What must exist before an item makes sense, and the item that provides it. */
export function insertPrecondition(
  lang: Lang,
  item: InsertItem,
  args: InsertArgs,
): { offer: InsertItem } | null {
  if (item === "cite" && syntaxOf(lang)?.inserts.has("cite") && !args.hasBibliography) {
    return { offer: "bibliography" };
  }
  return null;
}

const typstContent = (s: string) => s.replace(/[\\[\]#$*_`@<~]/g, "\\$&");

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

function dataLoader(path: string, name: string | undefined): string {
  const ext = path.slice(path.lastIndexOf(".") + 1).toLowerCase();
  const reader =
    { csv: "csv", json: "json", yaml: "yaml", yml: "yaml", toml: "toml", xml: "xml" }[ext] ??
    "read";
  const base = path.slice(path.lastIndexOf("/") + 1).replace(/\.[^.]*$/, "");
  const id = (name ?? base).replace(/[^A-Za-z0-9_-]/g, "-").replace(/^(?=[0-9-])/, "data-");
  return `#let ${id || "data"} = ${reader}("${typstString(path)}")`;
}

function tableCells(cols: number, rows: number): string {
  const row = `  ${Array<string>(cols).fill("[]").join(", ")},`;
  return `#table(\n  columns: ${cols},\n${Array<string>(rows).fill(row).join("\n")}\n)`;
}

function renderTypst(
  item: InsertItem,
  a: InsertArgs,
  selected: string,
  rel: string,
): Rendered | null {
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
      return a.path ? own(dataLoader(rel, a.name)) : null;
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

function renderMarkdown(
  item: InsertItem,
  a: InsertArgs,
  selected: string,
  rel: string,
  doc: Text,
): Rendered | null {
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

/** Inserts an item of the Insert menu; false when the format lacks it or an input is missing. */
export function insert(lang: Lang, item: InsertItem, args: InsertArgs = {}): StateCommand {
  return ({ state, dispatch }) => {
    const syntax = syntaxOf(lang);
    if (!syntax?.inserts.has(item) || state.readOnly) return false;
    if (insertPrecondition(lang, item, args)) return false;
    const range = state.selection.main;
    const selected = state.doc.sliceString(range.from, range.to);
    const rel = args.path ? relativePath(args.currentFile, args.path) : "";
    const rendered =
      lang === "typst"
        ? renderTypst(item, args, selected, rel)
        : renderMarkdown(item, args, selected, rel, state.doc);
    if (!rendered) return false;
    dispatch(place(state, rendered, item));
    return true;
  };
}

function place(state: EditorState, r: Rendered, item: InsertItem) {
  const { to } = state.selection.main;
  // A footnote note goes after the selection, which it does not replace.
  const from = item === "footnote" ? to : state.selection.main.from;
  const first = state.doc.lineAt(from);
  const last = state.doc.lineAt(to);
  const lead = r.block && first.text.slice(0, from - first.from).trim() !== "" ? "\n" : "";
  const trail = r.block && last.text.slice(to - last.from).trim() !== "" ? "\n" : "";
  const text = lead + r.text + trail;
  const changes: { from: number; to: number; insert: string }[] = [{ from, to, insert: text }];
  let cursor = from + lead.length + (r.cursor ?? r.text.length);
  if (r.appendix !== undefined) {
    changes.push({ from: state.doc.length, to: state.doc.length, insert: r.appendix });
    if (r.caretInAppendix)
      cursor = state.doc.length + text.length - (to - from) + r.appendix.length;
  }
  return state.update({
    changes,
    selection: EditorSelection.cursor(cursor),
    userEvent: `format.insert.${item}`,
    scrollIntoView: true,
  });
}
