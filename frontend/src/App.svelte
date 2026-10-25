<script lang="ts">
import { onMount } from "svelte";
import { t } from "./i18n";
import Editor from "./ui/Editor.svelte";
import HeaderBar from "./ui/HeaderBar.svelte";
import ShareDialog from "./ui/ShareDialog.svelte";
import { workspace } from "./workspace/workspace.svelte";

let sharing = $state(false);
const me = $derived({ id: workspace.myId, name: workspace.myName });

onMount(() => {
  void workspace.start();
});
</script>

<div class="window">
  <HeaderBar
    title={t("app.title")}
    status={workspace.status}
    {me}
    peers={workspace.peers}
    onShare={() => (sharing = true)}
  />
  <main>
    {#if workspace.handle}
      {#key workspace.handle.url}
        <Editor handle={workspace.handle} {me} />
      {/key}
    {/if}
  </main>
</div>
<ShareDialog bind:open={sharing} />

<style>
  .window {
    display: grid;
    grid-template-rows: auto 1fr;
    height: 100vh;
  }
  main {
    min-height: 0;
    padding: 0 var(--space-4);
  }
</style>
