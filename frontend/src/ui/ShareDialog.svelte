<script lang="ts">
import { untrack } from "svelte";
import { t } from "../i18n";
import { workspace } from "../workspace/workspace.svelte";
import Icon from "./icons/Icon.svelte";
import JoinConfirmDialog from "./JoinConfirmDialog.svelte";

let { open = $bindable(false) }: { open: boolean } = $props();
let dialog: HTMLDialogElement | undefined = $state();
let address = $state("");
let copied = $state(false);
/** Whether Go has answered, for this opening, what became of the invitation shown (J-8). */
let checked = $state(false);

const hostHelp = $derived(
  {
    loading: t("share.host.loading"),
    admin: t("share.host.help"),
    nonAdmin: t("share.host.adminOnly"),
    notMember: t("share.host.notMember"),
  }[workspace.inviteState],
);
const foreign = $derived(workspace.inviteState === "notMember");

$effect(() => {
  if (open && !dialog?.open) dialog?.showModal();
  if (!open && dialog?.open) dialog.close();
});

// The invitation shown is the one Go still holds, never a used or expired one (PC1b, rule J-8):
// at each opening it shows only once Go has answered, so a spent one never flashes by.
$effect(() => {
  if (!open) return;
  checked = false;
  // Once per opening: a new invitation made here is open, and is not asked about again.
  untrack(() => void workspace.refreshInvitation().finally(() => (checked = true)));
});

// A dropped invitation is used (PC1b, rule J-2): its text leaves the field.
$effect(() => {
  if (workspace.joinCancelled) address = "";
});

async function copy() {
  await navigator.clipboard.writeText(workspace.invitation);
  copied = true;
  setTimeout(() => (copied = false), 1500);
}

async function join(e: SubmitEvent) {
  e.preventDefault();
  await workspace.join(address);
  // An invitation into another document waits for the confirmation, over this dialog.
  if (!workspace.error && !workspace.joinOffer) open = false;
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
      <span>{hostHelp}</span>
      <button type="button" class="btn" disabled={!workspace.mayInvite} onclick={() => workspace.host()}>{t("share.host")}</button>
    </div>
    {#if !checked}
      <!-- Go is asked what became of the invitation: nothing is offered meanwhile. -->
    {:else if workspace.invitation && workspace.invitationState !== "open"}
      <div class="row" role="status">
        <span>{t(workspace.invitationState === "expired" ? "share.expired" : "share.used")}</span>
        <button type="button" class="btn" disabled={!workspace.mayInvite} onclick={() => workspace.host()}>{t("share.renew")}</button>
      </div>
    {:else if workspace.invitation}
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
      <span>{foreign ? t("share.join.foreignHelp") : t("share.join.help")}</span>
      <input
        bind:value={address}
        placeholder={t("share.join.placeholder")}
        spellcheck="false"
        required
      />
    </label>
    <div class="row end">
      <button type="submit" class="sug" disabled={workspace.joining} aria-busy={workspace.joining}>
        {t("share.join")}
      </button>
    </div>
    <!-- While the person who invites answers, the card says so; before, that a join into another
         document restarts Inkwell and keeps the current one (PC1b, rule J-2; in a copy of this
         one it does not); after « Annuler », that the invitation is used; after a failed attempt,
         only the error below. -->
    {#if workspace.joining}
      <p class="note in-card" role="status">{t("join.connecting")}</p>
    {:else if workspace.joinCancelled}
      <p class="note in-card" role="status">{t("join.cancelled")}</p>
    {:else if !foreign && !workspace.error}
      <p class="note in-card">{t("join.restart.note")} {t("join.restart.kept")}</p>
    {/if}
  </form>

  {#if workspace.error}
    <p class="error" role="alert">{workspace.error}</p>
  {/if}
  <p class="note">{t("share.trust")}</p>
</dialog>

{#if workspace.joinOffer}
  <JoinConfirmDialog
    offer={workspace.joinOffer}
    busy={workspace.leaving}
    onyes={() => workspace.confirmJoin()}
    onno={() => workspace.cancelJoin()}
  />
{/if}

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
  .in-card {
    margin: 0;
    padding: 0 var(--space-3) var(--space-2);
  }
</style>
