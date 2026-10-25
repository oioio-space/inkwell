import { history } from "@codemirror/commands";
import {
  EditorSelection,
  EditorState,
  type Extension,
  type StateCommand,
  type Transaction,
} from "@codemirror/state";

// Test helpers: documents with the selection written in them, so one string says both the
// text and where the caret/selection is. ⟨…⟩ is a selected range, ‸ a caret.

const OPEN = "⟨";
const CLOSE = "⟩";
const CARET = "‸";

export function parse(marked: string, extensions: Extension = []): EditorState {
  let doc = "";
  const ranges: ReturnType<typeof EditorSelection.range>[] = [];
  let start = -1;
  for (const ch of marked) {
    if (ch === OPEN) start = doc.length;
    else if (ch === CLOSE) {
      ranges.push(EditorSelection.range(start, doc.length));
      start = -1;
    } else if (ch === CARET) ranges.push(EditorSelection.cursor(doc.length));
    else doc += ch;
  }
  if (ranges.length === 0) ranges.push(EditorSelection.cursor(0));
  return EditorState.create({
    doc,
    selection: EditorSelection.create(ranges),
    extensions: [EditorState.allowMultipleSelections.of(true), history(), extensions],
  });
}

/** The inverse of `parse`. */
export function show(state: EditorState): string {
  const text = state.doc.toString();
  const marks: [number, string][] = [];
  for (const r of state.selection.ranges) {
    if (r.empty) marks.push([r.from, CARET]);
    else marks.push([r.from, OPEN], [r.to, CLOSE]);
  }
  marks.sort((a, b) => a[0] - b[0]);
  let out = "";
  let at = 0;
  for (const [pos, mark] of marks) {
    out += text.slice(at, pos) + mark;
    at = pos;
  }
  return out + text.slice(at);
}

export interface Run {
  ran: boolean;
  /** Number of transactions the command dispatched. */
  dispatched: number;
  state: EditorState;
  transactions: Transaction[];
}

export function run(state: EditorState, command: StateCommand): Run {
  const transactions: Transaction[] = [];
  const ran = command({
    state,
    dispatch: (tr) => {
      transactions.push(tr);
    },
  });
  const last = transactions.at(-1);
  return { ran, dispatched: transactions.length, state: last ? last.state : state, transactions };
}

/** Runs the command on the marked document and shows the result. */
export function apply(marked: string, command: StateCommand): string {
  return show(run(parse(marked), command).state);
}
