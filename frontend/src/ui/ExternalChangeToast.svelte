<script lang="ts">
import { t } from "../i18n";
import { workspace } from "../workspace/workspace.svelte";

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
  <div class="toast" role="alert">
    <span>{t("external.message", { name: workspace.external })}</span>
    <button type="button" class="suggested" disabled={busy} onclick={() => resolve(() => workspace.importExternal())}>
      {t("external.import")}
    </button>
    <button type="button" class="flat" disabled={busy} onclick={() => resolve(() => workspace.ignoreExternal())}>
      {t("external.ignore")}
    </button>
  </div>
{/if}

<style>
  .toast {
    position: fixed;
    inset-block-end: var(--space-4);
    inset-inline: 0;
    margin-inline: auto;
    width: fit-content;
    max-width: calc(100vw - 2 * var(--space-4));
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-3);
    background: var(--card-bg);
    color: var(--fg);
    border: 1px solid var(--border);
    border-radius: var(--radius-small);
    box-shadow: 0 2px 8px rgb(0 0 0 / 25%);
    font: 14px/1.3 var(--font-ui);
  }
  button {
    min-height: 34px;
    padding: 0 var(--space-4);
    border: 0;
    border-radius: var(--radius-small);
    font: 700 14px/1 var(--font-ui);
    cursor: pointer;
  }
  .suggested {
    background: var(--accent);
    color: var(--accent-fg);
  }
  .flat {
    background: transparent;
    color: var(--fg);
  }
</style>
