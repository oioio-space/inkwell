import { EditorState, Transaction } from "@codemirror/state";

/**
 * Keeps other people's edits out of CodeMirror's undo history, so Ctrl+Z only
 * undoes what *you* typed — by an inverse edit, never by rewriting Automerge history
 * (design doc, shadow zone 1).
 *
 * automerge-codemirror applies remote patches as transactions without a
 * `userEvent`; every local input, paste, delete, undo or redo carries one.
 */
export const localHistoryOnly = EditorState.transactionExtender.of((tr) =>
  tr.docChanged && !tr.annotation(Transaction.userEvent)
    ? { annotations: Transaction.addToHistory.of(false) }
    : null,
);
