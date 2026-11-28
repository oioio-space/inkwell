import { EditorSelection, type EditorState, type StateCommand } from "@codemirror/state";
import { registry } from "../../extensions";
import { shell } from "../../extensions/commands";
import type { Lang, Rendered } from "./lang";
import type { InsertArgs, InsertItem } from "./types";

// The Insert menu. Each item renders to text through the file's language (its format entry's
// `render`); block items sit on their own line, inline items replace the selection.

/** `target` written relative to the directory of `currentFile` (both project paths). */
export function relativePath(currentFile: string | undefined, target: string): string {
  if (!currentFile) return target;
  const from = currentFile.split("/").slice(0, -1);
  const to = target.split("/");
  let common = 0;
  while (common < from.length && common < to.length - 1 && from[common] === to[common]) common++;
  return [...Array<string>(from.length - common).fill(".."), ...to.slice(common)].join("/");
}

/** What must exist before an item makes sense, and the item that provides it. */
export function insertPrecondition(
  lang: Lang | undefined,
  item: InsertItem,
  args: InsertArgs,
): { offer: InsertItem } | null {
  if (!lang?.syntax.inserts.has(item)) return null;
  return lang.precondition?.(item, args) ?? null;
}

/** What an Insert item asks for before it writes anything (the same in every language). */
export const INSERT_INPUT: Readonly<Record<InsertItem, "nothing" | "file" | "key">> = {
  image: "file",
  figure: "file",
  include: "file",
  attach: "file",
  csvTable: "file",
  data: "file",
  bibliography: "file",
  link: "file",
  cite: "key",
  ref: "key",
  footnote: "nothing",
  outline: "nothing",
  pagebreak: "nothing",
  table: "nothing",
  formula: "nothing",
};

/**
 * The Insert menu's commands (ext/format, EX0c3), by their name in `format.insert-<name>`: the
 * item each one inserts. A file link is the `link` item, offered where `attach` is not.
 */
export const INSERT_COMMANDS: Readonly<Record<string, InsertItem>> = {
  image: "image",
  figure: "figure",
  include: "include",
  attach: "attach",
  "csv-table": "csvTable",
  data: "data",
  bibliography: "bibliography",
  "file-link": "link",
  cite: "cite",
  ref: "ref",
  footnote: "footnote",
  outline: "outline",
  pagebreak: "pagebreak",
};

/** The Insert item a command id stands for, if it is one of the Insert menu's. */
export const insertItemOf = (id: string): InsertItem | undefined => {
  const name = id.startsWith("format.insert-") ? id.slice("format.insert-".length) : "";
  return Object.hasOwn(INSERT_COMMANDS, name) ? INSERT_COMMANDS[name] : undefined;
};

/** The items of the Insert menu a language offers: the `inserts` of the `when` context. */
export const insertsOf = (lang: Lang | undefined): Set<string> =>
  new Set(Object.values(INSERT_COMMANDS).filter((item) => lang?.syntax.inserts.has(item)));

/** The language's reader of the picked file's type (its manifest's `readers`), else `read`. */
function readerOf(lang: Lang, path: string): string {
  const type = registry.typeOf(path)?.id ?? "";
  return Object.hasOwn(lang.readers, type) ? (lang.readers[type] as string) : "read";
}

// A step (a file to pick, a key to type) is the Insert menu's: a command run from the palette asks
// the shell (`step`), which shows the bar and asks the menu to open there; the menu, once
// mounted, takes the request.
let pendingStep: InsertItem | undefined;
const stepListeners = new Set<(item: InsertItem) => void>();

/** Asks the Insert menu to open at an item's step, now or as soon as it is mounted. */
export function openInsertStep(item: InsertItem): void {
  pendingStep = item;
  for (const f of stepListeners) f(item);
}

/** The Insert menu listens while it is mounted; a request made just before it was there is given. */
export function onInsertStep(f: (item: InsertItem) => void): () => void {
  stepListeners.add(f);
  if (pendingStep !== undefined) f(pendingStep);
  return () => stepListeners.delete(f);
}

let host = { file: "", bar: false };

/**
 * Where the Insert menu would show (App.svelte): the open file and whether its bar shows. A step
 * asked for is dropped when the file changes or the bar hides before a menu took it, so no menu
 * opens later at a step asked for elsewhere; the bar the shell shows for the step keeps it.
 */
export function noteInsertHost(next: { file: string; bar: boolean }): void {
  if (next.file !== host.file || (host.bar && !next.bar)) pendingStep = undefined;
  host = next;
}

/** The menu took the request (or closed): nothing pending any more. */
export function insertStepTaken(): void {
  pendingStep = undefined;
}

/**
 * The text surface's handlers of the Insert menu's commands: an item that needs nothing inserts at
 * once (one undo step); any other asks the shell to open the menu at its step (`step`), the bar
 * shown first if the person hid it.
 */
export function insertHandlers(
  lang: Lang | undefined,
  run: (command: StateCommand) => boolean,
): Record<string, () => boolean> {
  const step = (item: InsertItem) => {
    shell().step(item);
    return true;
  };
  return Object.fromEntries(
    Object.entries(INSERT_COMMANDS)
      .filter(([, item]) => lang?.syntax.inserts.has(item))
      .map(([name, item]) => [
        `format.insert-${name}`,
        INSERT_INPUT[item] === "nothing" ? () => run(insert(lang, item)) : () => step(item),
      ]),
  );
}

/** Inserts an item of the Insert menu; false when the format lacks it or an input is missing. */
export function insert(
  lang: Lang | undefined,
  item: InsertItem,
  args: InsertArgs = {},
): StateCommand {
  return ({ state, dispatch }) => {
    if (!lang?.syntax.inserts.has(item) || state.readOnly) return false;
    if (insertPrecondition(lang, item, args)) return false;
    const range = state.selection.main;
    const selected = state.doc.sliceString(range.from, range.to);
    const rel = args.path ? relativePath(args.currentFile, args.path) : "";
    const reader = args.path ? readerOf(lang, args.path) : "read";
    const rendered = lang.render(item, args, { selected, rel, doc: state.doc, reader });
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
