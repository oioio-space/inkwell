import { type Extension, Prec } from "@codemirror/state";
import { EditorView } from "@codemirror/view";

/**
 * PROTOTYPE, perf builds only (`VITE_PERF_CM_TYPING=1`), never in the product: CodeMirror inserts
 * a typed character itself instead of letting WebKit's contenteditable typing command do it, so
 * the keystroke no longer runs that command and its synchronous layout. A plain `insertText`
 * outside a composition is prevented and dispatched like CodeMirror's own default insert (the
 * input handlers first, then the selection replaced, `input.type`); everything else (composition,
 * dead keys, deletion, paste, spell-check replacement) keeps the browser's path. Enabling it is a
 * user decision: see docs/perf/2026-11-07-mutter-instrumentation.md.
 */
export function ownTyping(): Extension {
  return Prec.highest(
    EditorView.domEventHandlers({
      beforeinput(event, view) {
        const text = event.data;
        if (
          event.inputType !== "insertText" ||
          !text ||
          event.isComposing ||
          view.composing ||
          view.compositionStarted ||
          view.state.readOnly ||
          !view.state.facet(EditorView.editable)
        )
          return false;
        const { from, to } = view.state.selection.main;
        const insert = () =>
          view.state.update(view.state.replaceSelection(text), {
            userEvent: "input.type",
            scrollIntoView: true,
          });
        event.preventDefault();
        if (!view.state.facet(EditorView.inputHandler).some((h) => h(view, from, to, text, insert)))
          view.dispatch(insert());
        return true;
      },
    }),
  );
}
