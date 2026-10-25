<script lang="ts">
import { t } from "../i18n";
import { workspace } from "../workspace/workspace.svelte";
</script>

{#if workspace.sealed}
  <div class="banner" role="alert">{t("sealed.message")}</div>
{/if}

<style>
  .banner {
    position: fixed;
    inset-block-start: var(--space-4);
    inset-inline: 0;
    margin-inline: auto;
    width: fit-content;
    max-width: calc(100vw - 2 * var(--space-4));
    padding: var(--space-2) var(--space-3);
    background: var(--card-bg);
    color: var(--fg);
    border: 1px solid var(--border);
    border-radius: var(--radius-small);
    box-shadow: 0 2px 8px rgb(0 0 0 / 25%);
    font: 14px/1.3 var(--font-ui);
  }
</style>
