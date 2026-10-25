<script lang="ts">
import { t } from "../i18n";
import { workspace } from "../workspace/workspace.svelte";

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
    <button type="button" class="flat" onclick={() => (open = false)}>{t("share.close")}</button>
  </header>

  <label class="row">
    <span>{t("share.name")}</span>
    <input
      value={workspace.myName}
      maxlength="40"
      autocomplete="nickname"
      onchange={(e) => workspace.setName(e.currentTarget.value)}
    />
  </label>

  <section class="boxed">
    <div class="row">
      <span>{t("share.host.help")}</span>
      <button type="button" onclick={() => workspace.host()}>{t("share.host")}</button>
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
        <button type="button" class="flat" onclick={copy}>
          {copied ? t("share.copied") : t("share.copy")}
        </button>
      </div>
    {/if}
  </section>

  <form class="boxed" onsubmit={join}>
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
      <button type="submit" class="suggested" disabled={workspace.status === "connecting"}>
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
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: var(--space-3);
  }
  h2 {
    margin: 0;
    font: 700 18px/1.3 var(--font-ui);
  }
  .boxed {
    margin: var(--space-3) 0;
    border-radius: var(--radius);
    background: var(--card-bg);
    box-shadow: 0 0 0 1px var(--border);
  }
  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    min-height: var(--target-min);
    padding: var(--space-2) var(--space-3);
  }
  .boxed > * + * {
    border-top: 1px solid var(--border);
  }
  .row > span {
    font-size: 14px;
  }
  .end {
    justify-content: flex-end;
  }
  input {
    flex: 0 1 220px;
    min-height: 34px;
    padding: 0 var(--space-2);
    border: 0;
    border-radius: var(--radius-small);
    background: color-mix(in srgb, var(--fg) 8%, transparent);
    color: inherit;
    font: 14px var(--font-ui);
  }
  .invitation {
    flex: 1 1 auto;
    min-width: 0;
    font: 13px var(--font-mono);
    text-overflow: ellipsis;
  }
  button {
    min-height: 34px;
    padding: 0 var(--space-3);
    border: 0;
    border-radius: var(--radius-small);
    background: color-mix(in srgb, var(--fg) 10%, transparent);
    color: inherit;
    font: 600 14px var(--font-ui);
    cursor: pointer;
  }
  .flat {
    background: transparent;
  }
  .suggested {
    background: var(--accent);
    color: var(--accent-fg);
  }
  button:disabled {
    opacity: 0.5;
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
