<script lang="ts">
import { t } from "../i18n";
import { workspace } from "../workspace/workspace.svelte";
import Banner from "./Banner.svelte";

let busy = $state(false);
const notice = $derived(workspace.notice);

async function run(action: () => Promise<void>) {
  busy = true;
  try {
    await action();
  } finally {
    busy = false;
  }
}
</script>

<!-- A file that is gone or no longer text: Inkwell keeps its text and waits for the person. -->
{#if notice?.kind === "deleted"}
  <Banner tone="warning" role="alert">
    {t("external.deleted", { name: notice.name })}
    {#snippet action()}
      <button type="button" class="btn" disabled={busy} onclick={() => run(() => workspace.recreate())}>
        {t("external.recreate")}
      </button>
      <button type="button" class="btn" onclick={() => workspace.dismissNotice()}>
        {t("external.close")}
      </button>
    {/snippet}
  </Banner>
{:else if notice?.kind === "binary" || notice?.kind === "invalid"}
  <Banner tone="warning" role="alert">
    {#if notice.kind === "binary"}
      {t("external.binary", { name: notice.name })}
    {:else if notice.line !== undefined}
      {t("external.invalid", { name: notice.name, line: notice.line })}
    {:else}
      {t("external.invalid.noline", { name: notice.name })}
    {/if}
    {#snippet action()}
      <button type="button" class="btn" disabled={busy} onclick={() => run(() => workspace.keepOurs())}>
        {t("external.keep")}
      </button>
    {/snippet}
  </Banner>
{/if}
