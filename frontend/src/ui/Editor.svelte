<script module lang="ts">
/** Where the caret was in each document, so coming back to a file goes on where it stopped. */
const carets = new Map<string, number>();
</script>

<script lang="ts">
import type { DocHandle } from "@automerge/automerge-repo";
import type { LanguageSupport } from "@codemirror/language";
import { EditorView } from "@codemirror/view";
import { untrack } from "svelte";
import type { Lang } from "../editor/format";
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
  support,
  at,
}: {
  handle: DocHandle<Doc>;
  me: { id: string; name: string };
  lang: Lang;
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
  const l = untrack(() => lang);
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
$effect(() => editor?.setLanguage(lang, support));
// Until the rights are known, a reader, or a person who was disconnected (or whom the member list
// does not name): the text is read, selected and copied, not typed in.
$effect(() => editor?.setReadOnly(workspace.locked));
</script>

<div class="page" bind:this={host}></div>

<style>
  /* The source reads like the page it becomes: quiet chrome, the text in its own view. */
  .page {
    height: 100%;
    background: var(--view-bg);
  }
</style>
