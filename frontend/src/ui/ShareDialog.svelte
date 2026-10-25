<script lang="ts">
import { t } from "../i18n";
import { workspace } from "../workspace/workspace.svelte";
import Icon from "./icons/Icon.svelte";

let { open = $bindable(false) }: { open: boolean } = $props();
let dialog: HTMLDialogElement | undefined = $state();
let address = $state("");
let copied = $state(false);

$effect(() => {
  if (open && !dialog?.open) dialog?.showModal();
  if (!open && dialog?.open) dialog.close();
});

async function copy() {
  await navigator.clipboard.writeText(workspace.invitation);
  copied = true;
  setTimeout(() => (copied = false), 1500);
}

async function join(e: SubmitEvent) {
  e.preventDefault();
  await workspace.join(address);
  if (!workspace.error) open = false;
}
</script>

<dialog bind:this={dialog} onclose={() => (open = false)} aria-labelledby="share-title">
  <header>
    <h2 id="share-title">{t("share.title")}</h2>
    <button type="button" class="close" aria-label={t("share.close")} onclick={() => (open = false)}>
      <span class="disc"><Icon name="close" size={12} /></span>
    </button>
  </header>

  <label class="boxed card name">
    <span class="row">
      <span>{t("share.name")}</span>
      <input
        value={workspace.myName}
        maxlength="40"
        autocomplete="nickname"
        onchange={(e) => workspace.setName(e.currentTarget.value)}
      />
    </span>
  </label>

  <section class="boxed card">
    <div class="row">
      <span>{t("share.host.help")}</span>
      <button type="button" class="btn" onclick={() => workspace.host()}>{t("share.host")}</button>
    </div>
    {#if workspace.invitation}
      <div class="row">
        <input
          class="invitation"
          readonly
          aria-label={t("share.invitation")}
          value={workspace.invitation}
          onfocus={(e) => e.currentTarget.select()}
        />
        <button type="button" class="btn flat" onclick={copy}>
          {copied ? t("share.copied") : t("share.copy")}
        </button>
      </div>
    {/if}
  </section>

  <form class="boxed card" onsubmit={join}>
    <label class="row">
      <span>{t("share.join.help")}</span>
      <input
        bind:value={address}
        placeholder={t("share.join.placeholder")}
        spellcheck="false"
        required
      />
    </label>
    <div class="row end">
      <button type="submit" class="sug" disabled={workspace.joining}>
        {t("share.join")}
      </button>
    </div>
  </form>

  {#if workspace.error}
    <p class="error" role="alert">{workspace.error}</p>
  {/if}
  <p class="note">{t("share.trust")}</p>
</dialog>

<style>
  dialog {
    box-sizing: border-box;
    width: min(480px, calc(100vw - 32px));
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
  header {
    position: relative;
    display: grid;
    place-items: center;
    min-height: 34px;
    margin-bottom: var(--space-3);
  }
  h2 {
    margin: 0;
    font: 700 14.5px/1.3 var(--font-ui);
  }
  /* A 44 px target around a 26 px round button. */
  .close {
    position: absolute;
    inset-inline-end: -10px;
    display: grid;
    place-items: center;
    width: var(--target-min);
    height: var(--target-min);
    padding: 0;
    border: 0;
    background: transparent;
    color: var(--fg);
  }
  .disc {
    display: grid;
    place-items: center;
    width: 26px;
    height: 26px;
    border-radius: 50%;
    background: var(--btn-bg);
  }
  .boxed {
    display: block;
    margin: var(--space-3) 0;
  }
  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    min-height: var(--target-min);
    padding: var(--space-2) var(--space-3);
  }
  .row > span {
    font-size: 14px;
  }
  .end {
    justify-content: flex-end;
  }
  input {
    box-sizing: border-box;
    flex: 0 1 220px;
    min-height: var(--target-min);
    padding: 0 var(--space-2);
    border: 0;
    border-radius: var(--radius-button);
    background: var(--btn-bg);
    color: inherit;
    font: 14px var(--font-ui);
  }
  .invitation {
    flex: 1 1 auto;
    min-width: 0;
    font: 13px var(--font-mono);
    text-overflow: ellipsis;
  }
  dialog :global(.btn),
  dialog :global(.sug) {
    min-height: var(--target-min);
  }
  .error {
    color: var(--warning);
    font-size: 14px;
  }
  .note {
    margin: var(--space-2) 0 0;
    color: var(--fg-dim);
    font-size: 13px;
  }
</style>
