<script module lang="ts">
/** Where the caret was in each document, so coming back to a file goes on where it stopped. */
const carets = new Map<string, number>();
</script>

<script lang="ts">
import type { DocHandle } from "@automerge/automerge-repo";
import type { LanguageSupport } from "@codemirror/language";
import type { StateCommand } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { untrack } from "svelte";
import { type Lang, textHandlers } from "../editor/format";
import { registry } from "../extensions";
import { surface } from "../extensions/commands";
import { createEditor } from "../editor/setup";
import { megabytesOf, t } from "../i18n";
import { MAX_GROWN_CHARS } from "../store/limits";
import { type Doc, workspace } from "../workspace/workspace.svelte";
import { session } from "./editorSession.svelte";
import { notice } from "./notice.svelte";

let {
  handle,
  me,
  lang,
  fileType,
  support,
  at,
}: {
  handle: DocHandle<Doc>;
  me: { id: string; name: string };
  /** The formatting language (its commands on this surface); undefined is plain text. */
  lang: Lang | undefined;
  /** The file's type id: the language service's type gets its writing help (K-7). */
  fileType?: string | undefined;
  /** The grammar of the file's type, once loaded (ADR 0027). */
  support?: LanguageSupport | undefined;
  /** Where to put the caret when it opens (switching from a table's grid to its text). */
  at?: number | undefined;
} = $props();
let host: HTMLDivElement;
let editor: ReturnType<typeof createEditor> | undefined;

// Re-create the editor only when the document changes; a new name or file language is
// reconfigured in place (a Compartment), so the text, caret and undo history stay.
$effect(() => {
  const h = handle;
  const l = untrack(() => fileType);
  const g = untrack(() => support);
  const start = untrack(() => at) ?? carets.get(h.url);
  let mine: EditorView | undefined;
  editor = untrack(() =>
    createEditor(
      host,
      h,
      me,
      (people) => {
        workspace.hearNames(people);
      },
      {
        lang: l,
        support: g,
        ...(start !== undefined ? { at: start } : {}),
        onView: (view) => {
          mine = view;
          session.view = view;
          if (start === undefined) return;
          const head = view.state.selection.main.head;
          view.dispatch({ effects: EditorView.scrollIntoView(head, { y: "center" }) });
        },
        onState: session.note,
        onUndoKey: () => workspace.undoWithKey(),
        onRefused: (chars) =>
          notice.show(
            t("limit.pasteRefused", {
              size: megabytesOf(chars),
              max: megabytesOf(MAX_GROWN_CHARS),
            }),
            "alert",
          ),
      },
    ),
  );
  return () => {
    if (mine) carets.set(h.url, mine.state.selection.main.head);
    editor?.destroy();
    session.view = undefined;
    session.state = undefined;
  };
});
$effect(() => editor?.rename(me.name));

/** The view of this editor, while it is mounted. */
const view = () => (host ? EditorView.findFromDOM(host.querySelector<HTMLElement>(".cm-editor") ?? host) : null) ?? undefined;
const runOn = (command: StateCommand) => {
  const v = view();
  if (!v) return false;
  const done = command(v);
  v.focus();
  return done;
};
/** The text surface (`text.editor`): formatting, undo and redo, on this editor. */
const handlers = $derived(textHandlers(lang, runOn, () => workspace.undoWithKey()));
$effect(() => editor?.setLanguage(support, fileType));
// Until the rights are known, a reader, or a person who was disconnected (or whom the member list
// does not name): the text is read, selected and copied, not typed in.
$effect(() => editor?.setReadOnly(workspace.locked));
</script>

<div class="page" bind:this={host} use:surface={{ kind: "editor", handlers, viewer: registry.essentialViewer("text"), view }}></div>

<style>
  /* The source reads like the page it becomes: quiet chrome, the text in its own view. */
  .page {
    height: 100%;
    background: var(--view-bg);
  }
</style>
