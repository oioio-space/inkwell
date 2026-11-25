/**
 * The status bar's host side (EX0c1, plan §1): what every item is given, its component loaded
 * once from its extension's entry (`status[id]`), and the layout of the extensions' group.
 */
import type { EditorState } from "@codemirror/state";
import type { Component } from "svelte";
import type { Lang } from "../../editor/format";
import { commandRegistry } from "../../extensions/commands";
import { STATUS, type StatusGroupSpec } from "../../extensions/manifest";
import type { StatusItem } from "../../extensions/registry";
import { builtinStatusEntry } from "../../extensions/statusEntries";
import type { Diagnostic } from "../../preview/protocol";
import type { FileInfo } from "../../store/files";
import type { Status } from "../../workspace/workspace.svelte";

/** What the window knows that the items show: one object, the same for every item. */
export interface StatusBarProps {
  lang: Lang;
  status: Status;
  /** People connected besides this one. */
  peerCount: number;
  editorState: EditorState | undefined;
  info: FileInfo | undefined;
  /** The person chose a problem in the list: go to its place. */
  onProblem: (d: Diagnostic) => void;
  /** The counter steps through the problems (`app.problem-next`): the main window's status bar. */
  problemsNav: boolean;
  /** The text is not in the file yet (saving with Ctrl+S, or a write that failed). */
  unsaved: boolean;
  /** The open file is main.typ (the problems list's own branch, outside the main window). */
  ofMain: boolean;
}

/** An item's component receives its contribution and the bar's values. */
export interface StatusItemProps {
  item: StatusItem;
  bar: StatusBarProps;
}

const loaded: Record<string, Component<StatusItemProps>> = $state({});
const pending = new Set<string>();

const componentIn = (m: unknown, id: string): Component<StatusItemProps> | undefined =>
  (m as { status?: Record<string, Component<StatusItemProps>> } | undefined)?.status?.[id];

/**
 * An item's component. A built-in's entry is part of the start (`builtinStatusEntry`), so its item draws
 * with the first paint; any other entry is asked for once and draws when it is there (L-9).
 */
export function componentOf(item: StatusItem): Component<StatusItemProps> | undefined {
  if (item.origin === "builtin") {
    const eager = componentIn(builtinStatusEntry(item.folder, item.entry), item.id);
    if (eager) return eager;
  }
  const c = loaded[item.id];
  if (!c) start(item);
  return c;
}

function start(item: StatusItem): void {
  const r = commandRegistry();
  if (!r || pending.has(item.id)) return;
  pending.add(item.id);
  void r
    .loadStatus(item)
    .then((m) => {
      const c = componentIn(m, item.id);
      if (c) loaded[item.id] = c;
      else console.warn(`status ${item.id}: its entry has no component`);
    })
    .catch((error: unknown) => console.warn(`status ${item.id}:`, error));
}

/** How many items of the extensions' group show in the bar before ⋯ (§4.31 row 5). */
export const EXTENSION_ITEMS = 3;

/** A status group's spec, by side. */
export const groupOf = (item: StatusItem): StatusGroupSpec | undefined =>
  (STATUS[item.side].groups as readonly StatusGroupSpec[]).find((g) => g.id === item.group);

/**
 * The bar's layout: each item in the bar, or in ⋯ when its group says `more` and either the bar
 * is narrow (640 px or less) or more than {@link EXTENSION_ITEMS} of them show (L-4).
 */
export function statusLayout(
  items: readonly StatusItem[],
  narrow: boolean,
): { start: StatusItem[]; end: StatusItem[]; more: StatusItem[] } {
  const out = { start: [] as StatusItem[], end: [] as StatusItem[], more: [] as StatusItem[] };
  let extensions = 0;
  for (const x of items) {
    if (groupOf(x)?.narrow === "more" && (narrow || ++extensions > EXTENSION_ITEMS)) {
      out.more.push(x);
      continue;
    }
    out[x.side].push(x);
  }
  return out;
}
