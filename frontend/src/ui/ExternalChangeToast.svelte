<script lang="ts">
import { t } from "../i18n";
import { workspace } from "../workspace/workspace.svelte";
import Toast from "./Toast.svelte";

let busy = $state(false);

async function resolve(action: () => Promise<void>) {
  busy = true;
  try {
    await action();
  } finally {
    busy = false;
  }
}
</script>

{#if workspace.external}
  <Toast role="alert">
    {t("external.message", { name: workspace.external })}
    {#snippet actions()}
      <button type="button" class="fb lbl" disabled={busy} onclick={() => resolve(() => workspace.importExternal())}>
        {t("external.import")}
      </button>
      <button type="button" class="fb lbl" disabled={busy} onclick={() => resolve(() => workspace.ignoreExternal())}>
        {t("external.ignore")}
      </button>
    {/snippet}
  </Toast>
{/if}
