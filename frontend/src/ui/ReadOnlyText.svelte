<script lang="ts">
import type { LanguageSupport } from "@codemirror/language";
import { EditorView } from "@codemirror/view";
import { untrack } from "svelte";
import { createReadOnlyEditor } from "../editor/setup";
import { megabytesOf, t } from "../i18n";
import Banner from "./Banner.svelte";
import { session } from "./editorSession.svelte";

/** A text too big for a document (ADR 0028): shown whole, read-only, in no Automerge document. */
let {
  text,
  message,
  support,
  at,
}: {
  text: string;
  message?: string | undefined;
  support?: LanguageSupport | undefined;
  /** Where the caret opens (switching from the table of the same file). */
  at?: number | undefined;
} = $props();
let host: HTMLDivElement;
let editor: ReturnType<typeof createReadOnlyEditor> | undefined;

$effect(() => {
  const shown = text;
  const made = untrack(() =>
    createReadOnlyEditor(host, shown, {
      support,
      ...(at !== undefined ? { at } : {}),
      onView: (view) => {
        session.view = view;
        if (at === undefined) return;
        view.dispatch({
          effects: EditorView.scrollIntoView(view.state.selection.main.head, { y: "center" }),
        });
      },
      onState: session.note,
    }),
  );
  editor = made;
  return () => {
    made.destroy();
    editor = undefined;
    session.view = undefined;
    session.state = undefined;
  };
});
// A grammar that loads after the text is shown colours it in place.
$effect(() => editor?.setSupport(support));
</script>

<div class="wrap">
  <Banner role="status">
    <span data-testid="readonly">{message ?? t("limit.tooBig", { size: megabytesOf(text.length) })}</span>
  </Banner>
  <div class="page" bind:this={host}></div>
</div>

<style>
  .wrap {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  .page {
    flex: 1;
    min-height: 0;
    background: var(--view-bg);
  }
</style>
