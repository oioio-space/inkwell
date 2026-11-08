import { next as A } from "@automerge/automerge";
import { Repo } from "@automerge/automerge-repo";
import type { Extension } from "@codemirror/state";
import { EditorView } from "@codemirror/view";

let warmed = false;

/** CodeMirror's DOM observer: internal, so reached defensively (absent → that step is skipped). */
type WithObserver = { observer?: { flush?: (readSelection?: boolean) => boolean } };

/**
 * The first key typed in a fresh window cost 20 to 40 ms more than the next ones (PERF-TYPE-native
 * cold, docs/perf/2026-11-07-mutter-instrumentation.md): JavaScript and WebAssembly met for the
 * first time, CodeMirror's input path never run. Once per window, a hidden editor with the same
 * extensions takes a few characters through CodeMirror's DOM-change path and a few typing
 * transactions, and a throwaway in-memory document takes a few changes, so the person's first key
 * finds that code ready. Nothing reaches the real document: the hidden editor has its own text,
 * the throwaway repo has no network and no storage.
 *
 * Nothing outlives the call and focus never moves: the hidden editor is inert and aria-hidden,
 * is never focused (assistive technology has nothing to announce), and is destroyed before this
 * returns, so the page never holds a second editor.
 */
export function prewarmTyping(extensions: Extension): void {
  if (warmed) return;
  warmed = true;
  const box = document.createElement("div");
  box.setAttribute("aria-hidden", "true");
  box.inert = true;
  // In the viewport (CodeMirror measures only what is in view), invisible, inert to the pointer.
  box.style.cssText =
    "position:fixed;inset:0;opacity:0;pointer-events:none;overflow:hidden;contain:strict;z-index:-1";
  document.body.append(box);
  const view = new EditorView({ parent: box, doc: '#set text(lang: "fr")\n= Titre\n', extensions });
  const repo = new Repo();
  try {
    view.dispatch({ selection: { anchor: view.state.doc.length } });
    typeThroughDOM(view, "warm");
    for (let i = 0; i < 8; i++) {
      const at = view.state.doc.length;
      view.dispatch({
        changes: { from: at, insert: "x" },
        selection: { anchor: at + 1 },
        userEvent: "input.type",
      });
      view.coordsAtPos(at + 1);
    }
    const handle = repo.create<{ text: string }>({ text: "warm" });
    for (let i = 0; i < 8; i++) handle.change((d) => A.splice(d, ["text"], 4 + i, 0, "x"));
    const doc = handle.doc();
    A.diff(doc, [], A.getHeads(doc));
    A.saveIncremental(doc);
    A.generateSyncMessage(doc, A.initSyncState());
  } catch {
    // A warm-up that fails costs only the warm-up.
  } finally {
    view.destroy();
    box.remove();
    void repo.shutdown();
  }
}

/**
 * What a typed character does once the browser has edited the DOM: CodeMirror reads the DOM
 * change, runs the input handlers and dispatches an `input.type` transaction. The edit is made
 * on the text node directly (no focus, no selection), then the observer is flushed at once.
 */
function typeThroughDOM(view: EditorView, chars: string): void {
  const observer = (view as unknown as WithObserver).observer;
  if (typeof observer?.flush !== "function") return;
  for (const ch of chars) {
    const walker = document.createTreeWalker(view.contentDOM, NodeFilter.SHOW_TEXT);
    let last: Text | null = null;
    for (let n = walker.nextNode(); n; n = walker.nextNode()) last = n as Text;
    if (!last) return;
    last.appendData(ch);
    observer.flush(false);
  }
}
