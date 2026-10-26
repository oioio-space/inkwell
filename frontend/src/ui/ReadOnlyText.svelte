<script lang="ts">
import { untrack } from "svelte";
import { createReadOnlyEditor } from "../editor/setup";
import { megabytesOf, t } from "../i18n";
import Banner from "./Banner.svelte";
import { session } from "./editorSession.svelte";

/** A text too big for a document (ADR 0028): shown whole, read-only, in no Automerge document. */
let { text }: { text: string } = $props();
let host: HTMLDivElement;

$effect(() => {
  const shown = text;
  const editor = untrack(() =>
    createReadOnlyEditor(host, shown, {
      onView: (view) => (session.view = view),
      onState: (state) => (session.state = state),
    }),
  );
  return () => {
    editor.destroy();
    session.view = undefined;
    session.state = undefined;
  };
});
</script>

<div class="wrap">
  <Banner role="status">
    <span data-testid="readonly">{t("limit.tooBig", { size: megabytesOf(text.length) })}</span>
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
