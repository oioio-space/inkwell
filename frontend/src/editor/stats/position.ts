import type { EditorState } from "@codemirror/state";
import { graphemeCount } from "../format/text";

export interface CursorPosition {
  /** 1-based line number. */
  line: number;
  /** 1-based column, counted in grapheme clusters. */
  column: number;
}

/** Position of the main selection head: line and grapheme column, both 1-based. */
export function cursorPosition(state: EditorState): CursorPosition {
  const head = state.selection.main.head;
  const line = state.doc.lineAt(head);
  return { line: line.number, column: graphemeCount(line.text.slice(0, head - line.from)) + 1 };
}
