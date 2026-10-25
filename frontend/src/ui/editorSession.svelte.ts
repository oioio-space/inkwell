import type { EditorState, StateCommand } from "@codemirror/state";
import type { EditorView } from "@codemirror/view";

/**
 * The bridge between the editor and the shell around it: the shell reads the editor's state
 * (caret, selection, text) and runs commands on it, and every menu hands focus back to it.
 */
class EditorSession {
  view = $state.raw<EditorView | undefined>();
  state = $state.raw<EditorState | undefined>();

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
