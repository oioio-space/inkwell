import { type EditorState, type StateCommand, Transaction } from "@codemirror/state";
import type { EditorView } from "@codemirror/view";

/**
 * The bridge between the editor and the shell around it: the shell reads the editor's state
 * (caret, selection, text) and runs commands on it, and every menu hands focus back to it.
 */
class EditorSession {
  view = $state.raw<EditorView | undefined>();
  state = $state.raw<EditorState | undefined>();
  /** Set by a view that is not the editor (the grid) while it shows: puts its caret at a text offset. */
  placeAt = $state.raw<((offset: number) => void) | undefined>();
  /** How many times the person has acted on the editor (typed, selected, ran a command); others' edits do not count. */
  acts = 0;

  /** Takes the editor's new state and the transactions that made it. */
  note = (state: EditorState, transactions: readonly Transaction[] = []): void => {
    this.state = state;
    if (transactions.some((tr) => tr.annotation(Transaction.userEvent) !== undefined)) this.acts++;
  };

  /** Runs a command on the editor. Returns false when there is no editor or it declined. */
  run = (command: StateCommand): boolean => {
    const view = this.view;
    if (!view) return false;
    const done = command(view);
    view.focus();
    return done;
  };

  /** Gives focus back to the editor (after a menu closes). */
  focus = (): void => {
    this.view?.focus();
  };
}

export const session = new EditorSession();
