<script lang="ts">
import { t } from "../i18n";
import { workspace } from "../workspace/workspace.svelte";

/**
 * What stands in place of the editor after a join restarted Inkwell, while the document of the
 * person who invited has not arrived (PC1b, rule J-2), for as long as needed: who is waited for
 * (named when the member list names that person), what happens next, and the way back to the
 * document that was open before, when there is one.
 */
</script>

{#if workspace.waitingFor !== undefined}
  <div class="waiting" tabindex="-1" data-testid="join-waiting">
    <p class="who" role="status">
      {workspace.waitingFor
        ? t("join.waitingRoot", { name: workspace.waitingFor })
        : t("join.waitingRootUnnamed")}
    </p>
    <p>{t("join.waitingNext")}</p>
    {#if workspace.canReturn}
      <button type="button" class="btn" disabled={workspace.leaving} onclick={() => workspace.returnToPrevious()}>
        {t("join.return")}
      </button>
    {/if}
  </div>
{/if}

<style>
  /* As the card of a file that has no view (App.svelte, .file-view). */
  .waiting {
    box-sizing: border-box;
    height: 100%;
    overflow: auto;
    padding: 24px;
    color: var(--fg-dim);
    background: var(--view-bg);
  }
  p {
    margin: 0 0 var(--space-3);
    max-width: 60ch;
    line-height: 1.45;
  }
  .who {
    color: var(--fg);
  }
  .waiting :global(.btn) {
    min-height: var(--target-min);
  }
</style>
