import { automergeSyncPlugin } from "@automerge/automerge-codemirror";
import type { DocHandle } from "@automerge/automerge-repo";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import {
  drawSelection,
  EditorView,
  highlightActiveLine,
  keymap,
  lineNumbers,
  placeholder,
} from "@codemirror/view";
import { localHistoryOnly } from "./localHistory";
import { type Person, presenceSync } from "./presence";
import { remoteCursors } from "./remoteCursors";

type Doc = { text: string };

const theme = EditorView.theme({
  "&": { height: "100%", backgroundColor: "var(--view-bg)", color: "var(--fg)" },
  ".cm-scroller": { fontFamily: "var(--font-mono)", lineHeight: "1.6" },
  ".cm-content": { padding: "var(--space-4) 0", caretColor: "var(--accent)" },
  ".cm-gutters": { backgroundColor: "var(--view-bg)", color: "var(--fg-dim)", border: "none" },
  ".cm-activeLine": { backgroundColor: "color-mix(in srgb, var(--accent) 6%, transparent)" },
  "&.cm-focused": { outline: "none" },
});

/** The per-launch CSP nonce from index.html, so CodeMirror's <style> elements are allowed. */
function cspNonce() {
  const nonce = document.querySelector<HTMLMetaElement>('meta[name="csp-nonce"]')?.content;
  return nonce && nonce !== "__CSP_NONCE__" ? EditorView.cspNonce.of(nonce) : [];
}

/** Creates the collaborative source editor for a project document. */
export function createEditor(
  parent: HTMLElement,
  handle: DocHandle<Doc>,
  me: { id: string; name: string },
  onPeople?: (people: Person[]) => void,
): { rename: (name: string) => void; destroy: () => void } {
  let view: EditorView | undefined;
  const presence = presenceSync(handle, me, () => view, onPeople);
  view = new EditorView({
    parent,
    doc: handle.doc()?.text ?? "",
    extensions: [
      cspNonce(),
      lineNumbers(),
      history(),
      localHistoryOnly,
      drawSelection(),
      highlightActiveLine(),
      EditorView.lineWrapping,
      placeholder("Commencez à écrire…"),
      keymap.of([...defaultKeymap, ...historyKeymap]),
      automergeSyncPlugin({ handle, path: ["text"] }),
      remoteCursors(),
      presence.extension,
      theme,
    ],
  });
  view.focus();
  return {
    rename: presence.rename,
    destroy: () => {
      presence.stop();
      view?.destroy();
    },
  };
}
