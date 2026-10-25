<script lang="ts">
import type { DocHandle } from "@automerge/automerge-repo";
import { untrack } from "svelte";
import { createEditor } from "../editor/setup";
import type { Doc } from "../workspace/workspace.svelte";

let { handle, me }: { handle: DocHandle<Doc>; me: { id: string; name: string } } = $props();
let host: HTMLDivElement;
let editor: ReturnType<typeof createEditor> | undefined;

// Re-create the editor only when the document changes; a new name is broadcast instead.
$effect(() => {
  const h = handle;
  editor = untrack(() => createEditor(host, h, me));
  return () => editor?.destroy();
});
$effect(() => editor?.rename(me.name));
</script>

<div class="page" bind:this={host}></div>

<style>
  /* The source reads like the page it becomes: one centred measure, quiet chrome. */
  .page {
    height: 100%;
    max-width: 82ch;
    margin: 0 auto;
    background: var(--view-bg);
    box-shadow: 0 0 0 1px var(--border);
  }
</style>
