<script lang="ts">
import type { DocHandle } from "@automerge/automerge-repo";
import { untrack } from "svelte";
import type { Lang } from "../editor/format";
import { createEditor } from "../editor/setup";
import { type Doc, workspace } from "../workspace/workspace.svelte";
import { session } from "./editorSession.svelte";

let { handle, me, lang }: { handle: DocHandle<Doc>; me: { id: string; name: string }; lang: Lang } =
  $props();
let host: HTMLDivElement;
let editor: ReturnType<typeof createEditor> | undefined;

// Re-create the editor only when the document changes; a new name or file language is
// reconfigured in place (a Compartment), so the text, caret and undo history stay.
$effect(() => {
  const h = handle;
  const l = untrack(() => lang);
  editor = untrack(() =>
    createEditor(
      host,
      h,
      me,
      (people) => {
        workspace.names = Object.fromEntries(people.map((p) => [p.peerId, p.name]));
      },
      {
        lang: l,
        onView: (view) => (session.view = view),
        onState: (state) => (session.state = state),
        onUndoKey: () => workspace.undoWithKey(),
      },
    ),
  );
  return () => {
    editor?.destroy();
    session.view = undefined;
    session.state = undefined;
  };
});
$effect(() => editor?.rename(me.name));
$effect(() => editor?.setLang(lang));
</script>

<div class="page" bind:this={host}></div>

<style>
  /* The source reads like the page it becomes: quiet chrome, the text in its own view. */
  .page {
    height: 100%;
    background: var(--view-bg);
  }
</style>
