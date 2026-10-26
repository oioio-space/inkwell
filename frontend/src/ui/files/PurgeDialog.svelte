<script lang="ts">
import { t } from "../../i18n";
import type { TrashItem } from "../../project/materialize";

/** « Vider la corbeille » is final for the project's list, so it names what goes (FM-12). */
let {
  open = $bindable(false),
  items,
  onconfirm,
}: {
  open: boolean;
  items: readonly TrashItem[];
  onconfirm: () => void;
} = $props();

let dialog: HTMLDialogElement | undefined = $state();

$effect(() => {
  if (open && !dialog?.open) dialog?.showModal();
  if (!open && dialog?.open) dialog.close();
});
</script>

<dialog bind:this={dialog} onclose={() => (open = false)} aria-labelledby="purge-title">
  <h2 id="purge-title">{t("files.trash.purge.title")}</h2>
  <p>{t("files.trash.purge.body")}</p>
  <ul>
    {#each items as item (item.id)}
      <li>
        <span class="path">{item.path}</span>
        {#if item.contents > 0}<span class="dim">{t("files.trash.contents", { count: item.contents })}</span>{/if}
      </li>
    {/each}
  </ul>
  <div class="actions">
    <button type="button" class="btn" onclick={() => (open = false)}>{t("files.cancel")}</button>
    <button
      type="button"
      class="sug danger"
      onclick={() => {
        open = false;
        onconfirm();
      }}
    >
      {t("files.trash.purge.confirm")}
    </button>
  </div>
</dialog>

<style>
  dialog {
    box-sizing: border-box;
    width: min(440px, calc(100vw - 32px));
    padding: var(--space-4);
    border: 0;
    border-radius: var(--radius);
    background: var(--window-bg);
    color: var(--fg);
    box-shadow:
      0 0 0 1px var(--border),
      0 12px 32px rgb(0 0 0 / 25%);
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 30%);
  }
  h2 {
    margin: 0 0 var(--space-2);
    font: 700 14.5px/1.3 var(--font-ui);
  }
  p {
    margin: 0 0 var(--space-3);
    font-size: 14px;
  }
  ul {
    max-height: 200px;
    margin: 0 0 var(--space-3);
    padding: var(--space-2) var(--space-3);
    overflow-y: auto;
    list-style: none;
    border-radius: var(--radius-button);
    background: var(--btn-bg);
    font-size: 14px;
  }
  li {
    display: flex;
    justify-content: space-between;
    gap: var(--space-2);
    padding: 3px 0;
  }
  .path {
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .dim {
    flex: none;
    color: var(--fg-dim);
    font-size: 13px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
  }
  .actions :global(.btn),
  .actions :global(.sug) {
    min-height: var(--target-min);
  }
  .danger {
    background: var(--warning);
    color: #000;
  }
</style>
