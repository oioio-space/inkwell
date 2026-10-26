import type { EditorView } from "@codemirror/view";
import { createEditor } from "../../editor/setup";
import type { EditorModule, FileType } from "../types";

/**
 * The text editor: CodeMirror over the file's Automerge document. The language of the type, if it
 * has one, is loaded after the editor appears, so a JSON file shows its text at once and its
 * colours a moment later; the formatting profile (Typst, Markdown) is applied at once.
 */
const text: EditorModule = {
  mount(host, ctx) {
    const handle = ctx.handle;
    if (!handle) throw new Error("the text editor needs the document of the file");
    let type: FileType = ctx.type;
    let view: EditorView | undefined;
    let gone = false;
    const editor = createEditor(host, handle, ctx.me, ctx.onPeople, {
      lang: type.formatProfile ?? "plain",
      focus: ctx.focus,
      ...(ctx.at !== undefined ? { at: ctx.at } : {}),
      ...(ctx.onUndoKey ? { onUndoKey: ctx.onUndoKey } : {}),
      onView: (v) => {
        view = v;
        ctx.onView?.(v);
      },
      ...(ctx.onState ? { onState: ctx.onState } : {}),
    });
    const load = (t: FileType) => {
      void t.language?.().then((ext) => {
        if (!gone && type === t) editor.setExtra(ext);
      });
    };
    load(type);
    return {
      destroy() {
        gone = true;
        editor.destroy();
      },
      focus: () => view?.focus(),
      selection() {
        const r = view?.state.selection.main;
        return r ? { from: r.from, to: r.to } : undefined;
      },
      rename: editor.rename,
      setType(next) {
        type = next;
        editor.setLang(next.formatProfile ?? "plain");
        editor.setExtra([]);
        load(next);
      },
    };
  },
};

export default text;
