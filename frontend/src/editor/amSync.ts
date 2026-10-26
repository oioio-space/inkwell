import { next as A } from "@automerge/automerge";
// automerge-codemirror 0.2.0 (pinned) exports only its own plugin; the two pieces that turn a
// peer's patches into editor changes are the same ones it uses. The package has no `exports` map.
import { applyAmPatchesToCm } from "@automerge/automerge-codemirror/dist/amToCodemirror.js";
import {
  isReconcileTx,
  reconcileAnnotationType,
} from "@automerge/automerge-codemirror/dist/plugin.js";
import type { DocHandle } from "@automerge/automerge-repo";
import { EditorState, type Extension, type Transaction } from "@codemirror/state";
import { type EditorView, ViewPlugin } from "@codemirror/view";
import { applyEdits, type Edit } from "../store/chunked";
import { growthRefused } from "../store/limits";

export { isReconcileTx, reconcileAnnotationType };

/**
 * The editor's side of a document, in place of `automergeSyncPlugin`: the same two-way
 * reconciliation, plus what the size gate asks (ADR 0028):
 *
 * - a local edit that would take the text past 4 Mi characters is refused before anything is
 *   written (`onRefused` tells the person); a peer's growth is applied, since refusing it would
 *   break convergence, and the text only gets read-only where it is shown;
 * - an addition of more than 64 Ki characters (a paste) goes into the document in changes of
 *   64 Ki, 7 bytes of WASM per character instead of 300.
 */
export function amSync({
  handle,
  path,
  onRefused,
}: {
  handle: DocHandle<{ text: string }>;
  path: string[];
  onRefused?: (chars: number) => void;
}): Extension {
  if (!handle.isReady()) {
    throw new Error("ensure the handle is ready before initializing the sync");
  }
  const gate = EditorState.transactionFilter.of((tr) => {
    if (!tr.docChanged || isReconcileTx(tr)) return tr;
    if (!growthRefused(tr.startState.doc.length, tr.newDoc.length)) return tr;
    onRefused?.(tr.newDoc.length);
    return [];
  });
  const plugin = ViewPlugin.fromClass(
    class {
      reconciledHeads = A.getHeads(handle.doc());
      isProcessingCmTransaction = false;
      constructor(readonly view: EditorView) {
        handle.on("change", this.onChange);
      }
      update(update: { transactions: readonly Transaction[] }) {
        // Changes made through the transaction are ignored by the listener below.
        this.isProcessingCmTransaction = true;
        try {
          const heads = applyTransactions(handle, path, update.transactions);
          if (heads) this.reconciledHeads = heads;
        } finally {
          this.isProcessingCmTransaction = false;
        }
      }
      onChange = () => {
        if (this.isProcessingCmTransaction) return;
        const current = A.getHeads(handle.doc());
        if (A.equals(current, this.reconciledHeads)) return;
        applyAmPatchesToCm(this.view, path, A.diff(handle.doc(), this.reconciledHeads, current));
        this.reconciledHeads = current;
      };
      destroy() {
        handle.off("change", this.onChange);
      }
    },
  );
  return [gate, plugin];
}

/** Writes the local transactions to the document; the heads after, or undefined if none changed it. */
function applyTransactions(
  handle: DocHandle<{ text: string }>,
  path: string[],
  transactions: readonly Transaction[],
): A.Heads | undefined {
  const edits: Edit[] = [];
  for (const tr of transactions) {
    if (isReconcileTx(tr) || tr.changes.empty) continue;
    tr.changes.iterChanges((fromA, toA, fromB, _toB, inserted) => {
      edits.push({ at: fromB, remove: toA - fromA, put: inserted.toString() });
    });
  }
  if (edits.length === 0) return undefined;
  applyEdits(handle, edits, path);
  return A.getHeads(handle.doc());
}
