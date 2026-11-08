import { next as A } from "@automerge/automerge";
import type { DocHandle } from "@automerge/automerge-repo";
import { EditorState, type Extension, Prec, type Transaction } from "@codemirror/state";
import { EditorView, ViewPlugin } from "@codemirror/view";
import { profileNativeStage } from "../perf/nativeDispatch";
import { applyEdits, chunkEdits, type Edit, spliceAll } from "../store/chunked";
import { growthRefused } from "../store/limits";
import { applyAmPatchesToCm, isReconcileTx, reconcileAnnotationType } from "./amPatches";

export { isReconcileTx, reconcileAnnotationType };

/** An editor's typed edits the document has not received yet (they wait for the paint). */
interface Waiting {
  readonly waiting: boolean;
  flush(): void;
  /** Runs `fn` once what waits is in the document. */
  onceWritten(fn: () => void): void;
}
const editorsOf = new WeakMap<object, Set<Waiting>>();
const editorOfView = new WeakMap<EditorView, Waiting>();

/**
 * Writes now the keys typed in the editors of `handle` that wait for the paint. Call it before
 * reading the document with the editor's offsets, or writing to it at them, outside a shortcut or
 * a click (those already flush).
 */
export function flushTyped(handle: DocHandle<unknown>): void {
  for (const editor of editorsOf.get(handle) ?? []) editor.flush();
}

/** Runs `fn` once the keys typed in every editor of `handle` are in the document: now if none wait. */
export function whenWritten(handle: DocHandle<unknown>, fn: () => void): void {
  const editor = [...(editorsOf.get(handle) ?? [])].find((e) => e.waiting);
  if (editor) editor.onceWritten(() => whenWritten(handle, fn));
  else fn();
}

/**
 * Runs `fn` once the keys typed in `view` are in its document: now if none wait, or if `view` has
 * no document. For a reader of the document reacting to a typed key (the language service).
 */
export function whenTypedWritten(view: EditorView, fn: () => void): void {
  const editor = editorOfView.get(view);
  if (editor?.waiting) editor.onceWritten(() => whenTypedWritten(view, fn));
  else fn();
}

// Keys that only type text (or prepare it): the edit they make may wait for the paint. Any other
// key may run a command that reads or writes the document, so what waits is written first.
const PREFIX_KEYS = new Set(["Shift", "CapsLock", "AltGraph", "Dead", "Process", "Unidentified"]);
const typesText = (e: KeyboardEvent): boolean =>
  e.isComposing ||
  PREFIX_KEYS.has(e.key) ||
  (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey);

const profiled = <T>(work: () => T): T =>
  import.meta.env.VITE_PERF_NATIVE === "1" ? profileNativeStage("automerge-ui", work) : work();

/** Typed edits that could not be written: the document is the source of truth, say what is lost. */
function reportLost(edits: readonly Edit[], error: unknown): void {
  const chars = edits.reduce((n, e) => n + e.put.length, 0);
  console.error(`amSync: ${chars} typed characters were not written to the document`, error);
}

/**
 * The editor's side of a document, in place of `automergeSyncPlugin`: the same two-way
 * reconciliation, plus what the size gate asks (ADR 0028):
 *
 * - a local edit that would take the text past 4 Mi characters is refused before anything is
 *   written (`onRefused` tells the person); a peer's growth is applied, since refusing it would
 *   break convergence, and the text only gets read-only where it is shown;
 * - an addition of more than 64 Ki characters (a paste) goes into the document in changes of
 *   64 Ki, 7 bytes of WASM per character instead of 300, on every write path.
 *
 * A typed key is written after the next paint (PERF-TYPE-native), so its handler only updates the
 * editor. Until then the editor is ahead of the document: a shortcut, a click, leaving the editor
 * or the page write it first; `flushTyped` and `whenWritten` are for other readers.
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
  // Set by CodeMirror's DOM input path (a typed character) just before it dispatches, which it
  // does synchronously: a handler that swallows the key leaves no flag for a later transaction.
  let typed = false;
  const typing = Prec.highest(
    EditorView.inputHandler.of(() => {
      typed = true;
      queueMicrotask(() => {
        typed = false;
      });
      return false;
    }),
  );
  const plugin = ViewPlugin.fromClass(
    class implements Waiting {
      reconciledHeads = A.getHeads(handle.doc());
      isProcessingCmTransaction = false;
      // Local edits not yet in the document, in order, in the coordinates of reconciledHeads.
      pending: Edit[] = [];
      scheduled = false;
      // The waiting edits are being written concurrently with a change that came in; `local` is
      // set, to the heads of the last change written, when the handle runs that write.
      rebase: { local?: A.Heads } | undefined;
      afterWrite: Array<() => void> = [];
      constructor(readonly view: EditorView) {
        handle.on("change", this.onChange);
        view.contentDOM.addEventListener("blur", this.flush);
        document.addEventListener("visibilitychange", this.flush);
        window.addEventListener("pagehide", this.flush);
        // Captured, so it runs before the command the key or the click starts.
        window.addEventListener("keydown", this.onKey, true);
        window.addEventListener("pointerdown", this.flush, true);
        const editors = editorsOf.get(handle) ?? new Set<Waiting>();
        editorsOf.set(handle, editors.add(this));
        editorOfView.set(view, this);
      }
      get waiting() {
        return this.pending.length > 0 || this.rebase !== undefined;
      }
      onceWritten(fn: () => void) {
        this.afterWrite.push(fn);
      }
      update(update: { transactions: readonly Transaction[] }) {
        // Only the typed insert itself waits; anything else in the same update (a paste, an undo,
        // a command) is written now, after what waits.
        const deferred =
          typed &&
          update.transactions.every(
            (tr) => !tr.docChanged || isReconcileTx(tr) || tr.isUserEvent("input.type"),
          );
        typed = false;
        collectEdits(update.transactions, this.pending);
        if (this.pending.length === 0) return;
        if (deferred && document.visibilityState === "visible") this.schedule();
        else this.flush();
      }
      onKey = (e: KeyboardEvent) => {
        if (!typesText(e)) this.flush();
      };
      schedule() {
        if (this.scheduled) return;
        this.scheduled = true;
        requestAnimationFrame(() => setTimeout(this.flush, 0));
      }
      /** Writes the waiting edits as one change (several for a big paste, ADR 0028). */
      flush = () => {
        this.scheduled = false;
        if (this.pending.length === 0) return;
        const edits = this.pending;
        this.pending = [];
        // Changes made here are ignored by the listener below.
        this.isProcessingCmTransaction = true;
        try {
          profiled(() => applyEdits(handle, edits, path));
          this.reconciledHeads = A.getHeads(handle.doc());
        } catch (error) {
          reportLost(edits, error);
        } finally {
          this.isProcessingCmTransaction = false;
        }
        this.written();
      };
      written() {
        const fns = this.afterWrite;
        this.afterWrite = [];
        for (const fn of fns) fn();
      }
      onChange = () => {
        if (this.isProcessingCmTransaction || !handle.isReady()) return;
        const current = A.getHeads(handle.doc());
        if (this.rebase) {
          // Until the handle has run the concurrent write below, the editor already shows it.
          const local = this.rebase.local;
          if (!local) return;
          this.rebase = undefined;
          this.reconcile(local, current);
          this.written();
          return;
        }
        if (A.equals(current, this.reconciledHeads)) return;
        if (this.pending.length > 0) this.rebaseWaiting();
        else this.reconcile(this.reconciledHeads, current);
      };
      /**
       * Another change came in while typed edits wait. They are written as of the text the editor
       * shows (reconciledHeads), concurrently with that change, cut as every write (ADR 0028); once
       * they land, the editor receives only what the other change adds to them. Inside a "change"
       * listener the handle may run the write now or right after the listeners: either way, the
       * heads of the written changes are known exactly, not guessed from the document's history.
       */
      rebaseWaiting() {
        const edits = this.pending;
        const base = this.reconciledHeads;
        this.pending = [];
        const rebase: { local?: A.Heads } = {};
        this.rebase = rebase;
        try {
          handle.update((doc) => {
            if (!A.hasHeads(doc, base)) {
              this.recover(doc, edits, new Error("the document no longer has the edits' base"));
              return doc;
            }
            let next = doc;
            try {
              return profiled(() => {
                let at = base;
                for (const batch of chunkEdits(edits)) {
                  const made = A.changeAt(next, at, (d) => spliceAll(d, path, batch));
                  next = made.newDoc;
                  at = made.newHeads ?? at;
                }
                rebase.local = at;
                return next;
              });
            } catch (error) {
              // A changeAt that throws leaves its document isolated at those heads (Automerge
              // 3.5): later changes would all fail. A clone is the same document, sound.
              const sound = A.clone(next);
              this.recover(sound, edits, error);
              return sound;
            }
          });
        } catch (error) {
          this.recover(handle.doc(), edits, error);
        }
      }
      /**
       * The waiting edits cannot be written where they were typed (the document was replaced by
       * one without that history, or refuses changes): the editor takes the document's text, the
       * single source of truth, and the lost keys are reported.
       */
      recover(doc: A.Doc<{ text: string }>, edits: readonly Edit[], error: unknown) {
        this.rebase = undefined;
        reportLost(edits, error);
        const text = path.reduce<unknown>((o, k) => (o as Record<string, unknown>)?.[k], doc);
        this.view.dispatch({
          changes: { from: 0, to: this.view.state.doc.length, insert: String(text ?? "") },
          annotations: reconcileAnnotationType.of({}),
        });
        this.reconciledHeads = A.getHeads(doc);
        this.written();
      }
      reconcile(from: A.Heads, to: A.Heads) {
        applyAmPatchesToCm(this.view, path, A.diff(handle.doc(), from, to));
        this.reconciledHeads = to;
      }
      destroy() {
        try {
          this.flush();
        } finally {
          handle.off("change", this.onChange);
          this.view.contentDOM.removeEventListener("blur", this.flush);
          document.removeEventListener("visibilitychange", this.flush);
          window.removeEventListener("pagehide", this.flush);
          window.removeEventListener("keydown", this.onKey, true);
          window.removeEventListener("pointerdown", this.flush, true);
          editorsOf.get(handle)?.delete(this);
          this.written();
        }
      }
    },
  );
  return [gate, typing, plugin];
}

/** Appends the local transactions' edits, each in the coordinates the previous ones left. */
function collectEdits(transactions: readonly Transaction[], edits: Edit[]): void {
  for (const tr of transactions) {
    if (isReconcileTx(tr) || tr.changes.empty) continue;
    tr.changes.iterChanges((fromA, toA, fromB, _toB, inserted) => {
      edits.push({ at: fromB, remove: toA - fromA, put: inserted.toString() });
    });
  }
}
