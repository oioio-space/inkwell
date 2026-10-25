import { automergeSyncPlugin } from "@automerge/automerge-codemirror";
import type { DocHandle } from "@automerge/automerge-repo";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { Compartment, type EditorState, type Extension, Prec } from "@codemirror/state";
import {
  drawSelection,
  EditorView,
  highlightActiveLine,
  keymap,
  lineNumbers,
  placeholder,
} from "@codemirror/view";
import { t } from "../i18n";
import { formatKeymap, type Lang } from "./format";
import { languageExtension } from "./highlight";
import { localHistoryOnly } from "./localHistory";
import { type Person, presenceSync } from "./presence";
import { remoteCursors } from "./remoteCursors";

type Doc = { text: string };

// The source as in the mockup: 14 px mono, airy lines, a 56 px gutter, a pale active line.
const theme = EditorView.theme({
  "&": {
    height: "100%",
    backgroundColor: "var(--view-bg)",
    color: "var(--fg)",
    fontSize: "14px",
  },
  ".cm-scroller": { fontFamily: "var(--font-mono)", lineHeight: "1.75" },
  ".cm-content": { padding: "20px 28px 20px 0", caretColor: "var(--fg)" },
  "&.cm-editor .cm-line": { padding: "0 0 0 16px" },
  ".cm-gutters": {
    backgroundColor: "var(--view-bg)",
    color: "color-mix(in srgb, var(--fg) 45%, transparent)",
    border: "none",
  },
  "&.cm-editor .cm-lineNumbers .cm-gutterElement": { minWidth: "56px", padding: "0 0 0 0" },
  ".cm-activeLine": { backgroundColor: "color-mix(in srgb, var(--accent-bg) 7%, transparent)" },
  ".cm-activeLineGutter": { backgroundColor: "transparent", color: "var(--fg)" },
  ".cm-cursor, .cm-dropCursor": { borderLeftColor: "var(--fg)", borderLeftWidth: "2px" },
  "&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground, .cm-selectionBackground":
    { backgroundColor: "color-mix(in srgb, var(--accent-bg) 22%, transparent)" },
  "&.cm-focused": { outline: "none" },
});

/**
 * What depends on the file's language: its parser and colours, and the formatting keys (ahead
 * of the default keymap, which binds Mod-i to "select parent syntax").
 */
export const forLang = (lang: Lang): Extension => [
  languageExtension(lang),
  Prec.high(keymap.of(formatKeymap(lang))),
];

/** The per-launch CSP nonce from index.html, so CodeMirror's <style> elements are allowed. */
function cspNonce() {
  const nonce = document.querySelector<HTMLMetaElement>('meta[name="csp-nonce"]')?.content;
  return nonce && nonce !== "__CSP_NONCE__" ? EditorView.cspNonce.of(nonce) : [];
}

/** What the shell needs from the editor: its language and a feed of its state. */
export interface EditorOptions {
  lang?: Lang;
  /** Called with the new state at creation and after each change of text or selection. */
  onState?: (state: EditorState) => void;
  /** Called once with the view, so the shell can run commands on it and give it back focus. */
  onView?: (view: EditorView) => void;
}

/** Creates the collaborative source editor for a project document. */
export function createEditor(
  parent: HTMLElement,
  handle: DocHandle<Doc>,
  me: { id: string; name: string },
  onPeople?: (people: Person[]) => void,
  { lang = "plain", onState, onView }: EditorOptions = {},
): { rename: (name: string) => void; setLang: (lang: Lang) => void; destroy: () => void } {
  let view: EditorView | undefined;
  const language = new Compartment();
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
      placeholder(t("editor.placeholder")),
      language.of(forLang(lang)),
      keymap.of([...defaultKeymap, ...historyKeymap]),
      EditorView.contentAttributes.of({ "aria-label": t("editor.label") }),
      EditorView.updateListener.of((u) => {
        if (u.docChanged || u.selectionSet) onState?.(u.state);
      }),
      automergeSyncPlugin({ handle, path: ["text"] }),
      remoteCursors(),
      presence.extension,
      theme,
    ],
  });
  view.focus();
  onView?.(view);
  onState?.(view.state);
  return {
    rename: presence.rename,
    setLang: (next) => view?.dispatch({ effects: language.reconfigure(forLang(next)) }),
    destroy: () => {
      presence.stop();
      view?.destroy();
    },
  };
}
