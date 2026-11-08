import {
  autocompletion,
  type CompletionContext,
  type CompletionResult,
} from "@codemirror/autocomplete";
import { StateEffect, StateField } from "@codemirror/state";
import { EditorView, hoverTooltip, keymap, showTooltip, type Tooltip } from "@codemirror/view";
import { whenTypedWritten } from "../editor/amSync";
import { calleeOffset } from "./calls";
import { documentation, readable } from "./documentation";
import { docPosition } from "./positions";
import { currentFile, language } from "./service";

interface Item {
  label: string;
  detail?: string;
  documentation?: unknown;
  insertText?: string;
  textEdit?: { newText: string };
  insertTextFormat?: number;
}
function record(value: unknown): Record<string, unknown> | undefined {
  return value !== null && typeof value === "object"
    ? (value as Record<string, unknown>)
    : undefined;
}

function card(text: string): HTMLElement {
  const node = document.createElement("div");
  node.className = "writing-help";
  const p = document.createElement("p");
  p.textContent = readable(text);
  node.append(p);
  return node;
}
async function complete(context: CompletionContext): Promise<CompletionResult | null> {
  const word = context.matchBefore(/[\p{L}\p{N}_-]*/u);
  const before = context.state.doc.sliceString(0, context.pos);
  if (!context.explicit && !/[#.@(,:][^\n]*$/u.test(before)) return null;
  const result = await language.request(
    "textDocument/completion",
    currentFile(),
    docPosition(context.state.doc, context.pos),
  );
  const list = Array.isArray(result) ? result : record(result)?.items;
  if (!Array.isArray(list)) return null;
  // The project's own folder (.inkwell/) is not something to offer to a person writing.
  const items = list.filter(
    (item): item is Item => typeof item?.label === "string" && !/^\.[^/]*\/$/.test(item.label),
  );
  return {
    from: word?.from ?? context.pos,
    options: items.slice(0, 50).map((item) => ({
      label: item.label,
      ...(item.detail ? { detail: item.detail } : {}),
      // No documentation, no card: an empty box beside the list looks like a glitch.
      info: () => {
        const text = readable(documentation(item.documentation));
        return text ? card(text) : null;
      },
      apply:
        item.insertTextFormat === 2
          ? item.label
          : (item.textEdit?.newText ?? item.insertText ?? item.label),
    })),
  };
}
const helpEffect = StateEffect.define<Tooltip | null>();
const helpState = StateField.define<Tooltip | null>({
  create: () => null,
  update(value, tr) {
    if (tr.docChanged) return null;
    for (const effect of tr.effects) if (effect.is(helpEffect)) value = effect.value;
    return value;
  },
  provide: (field) => showTooltip.from(field),
});
/** Decorations may replace EditorState while the document and requested location stay current. */
function helpContext(view: EditorView) {
  const state = view.state;
  const file = currentFile();
  return {
    state,
    file,
    current: () =>
      view.state.doc === state.doc &&
      view.state.selection.eq(state.selection) &&
      currentFile() === file,
  };
}
export async function requestSignature(view: EditorView, keyboard = false) {
  const { state, file, current } = helpContext(view);
  const pos = state.selection.main.head;
  const result = record(
    await language.request("textDocument/signatureHelp", file, docPosition(state.doc, pos)),
  );
  if (!current()) return;
  let text = "";
  const signatures = result?.signatures;
  if (Array.isArray(signatures)) {
    const active = record(
      signatures[typeof result?.activeSignature === "number" ? result.activeSignature : 0],
    );
    text = `${typeof active?.label === "string" ? active.label : ""}\n${documentation(active?.documentation)}`;
  }
  const callee = calleeOffset(state.doc.sliceString(0, pos), pos);
  if (!text.trim() && (keyboard || callee !== undefined)) {
    const result = record(
      await language.request("textDocument/hover", file, docPosition(state.doc, callee ?? pos)),
    );
    text = documentation(result?.contents);
  }
  if (!current() || !text.trim()) return;
  view.dispatch({
    effects: helpEffect.of({ pos, above: true, create: () => ({ dom: card(text) }) }),
  });
}
export async function requestHover(view: EditorView, pos: number): Promise<Tooltip | null> {
  const { state, file, current } = helpContext(view);
  const result = record(
    await language.request("textDocument/hover", file, docPosition(state.doc, pos)),
  );
  const text = documentation(result?.contents);
  return text && current() ? { pos, above: true, create: () => ({ dom: card(text) }) } : null;
}
export const writingHelp = [
  autocompletion({ override: [complete], maxRenderedOptions: 50 }),
  helpState,
  hoverTooltip(requestHover, { hoverTime: 500 }),
  keymap.of([
    {
      key: "Mod-Shift-Space",
      run: (view) => {
        void requestSignature(view, true);
        return true;
      },
    },
    {
      key: "Escape",
      run: (view) => {
        if (!view.state.field(helpState)) return false;
        view.dispatch({ effects: helpEffect.of(null) });
        return true;
      },
    },
  ]),
  EditorView.updateListener.of((update) => {
    if (
      update.docChanged &&
      /[(,]$/.test(
        update.state.doc.sliceString(
          Math.max(0, update.state.selection.main.head - 1),
          update.state.selection.main.head,
        ),
      )
    )
      // The language service reads the document, which gets a typed key after the paint.
      whenTypedWritten(update.view, () => void requestSignature(update.view));
  }),
  EditorView.theme({
    ".cm-tooltip": {
      backgroundColor: "var(--popover-bg)",
      color: "var(--fg)",
      border: "1px solid var(--shade)",
      borderRadius: "var(--radius-small)",
      boxShadow: "var(--popover-shadow)",
    },
    ".writing-help": {
      maxWidth: "420px",
      padding: "8px 12px",
      whiteSpace: "pre-wrap",
      overflowWrap: "anywhere",
      // Never taller than the window: a long documentation scrolls inside its card.
      maxHeight: "40vh",
      overflowY: "auto",
    },
  }),
];
