<script lang="ts">
import { t } from "../i18n";

/**
 * Asks before an accepted invitation restarts Inkwell (PC1b, rule J-2), as the take-over does
 * (`TakeOverDialog`): which document, the folder it gets, and that the text open now stays in
 * its own folder (not said on a first launch, which has no text of its own). It stays until the
 * person chooses.
 */
let {
  offer,
  busy = false,
  onyes,
  onno,
}: {
  offer: { title: string; folder: string; current: string };
  busy?: boolean;
  onyes: () => void;
  onno: () => void;
} = $props();

let dialog: HTMLDialogElement | undefined = $state();

$effect(() => {
  if (dialog && !dialog.open) dialog.showModal();
});
</script>

<dialog
  bind:this={dialog}
  data-testid="join-confirm"
  aria-labelledby="join-confirm-title"
  aria-describedby="join-confirm-body"
  oncancel={(e) => {
    // Once « Redémarrer et ouvrir » is pressed the page is leaving: Escape no longer cancels.
    if (busy) e.preventDefault();
  }}
  onclose={onno}
>
  <h2 id="join-confirm-title">{t("join.confirm.title", { title: offer.title })}</h2>
  <p id="join-confirm-body">
    {t("join.restart.folder", { folder: offer.folder })}
    {#if offer.current}{t("join.confirm.current", { current: offer.current })}{/if}
  </p>
  <div class="foot">
    <button type="button" class="btn" disabled={busy} onclick={onno}>{t("people.cancel")}</button>
    <button type="button" class="sug" disabled={busy} aria-busy={busy} onclick={onyes}>{t("join.confirm.yes")}</button>
  </div>
</dialog>

<style>
  /* As the question of a take-over (people/TakeOverDialog.svelte). */
  dialog {
    border: 0;
    border-radius: var(--radius);
    padding: 20px;
    background: var(--popover-bg);
    color: var(--fg);
    box-shadow: var(--shadow-popover);
    max-width: min(480px, calc(100vw - 32px));
  }
  /* Forced colours drop the shadow and the background: the dialog keeps an edge. */
  @media (forced-colors: active) {
    dialog {
      border: 1px solid CanvasText;
    }
  }
  dialog::backdrop {
    background: rgb(0 0 6 / 35%);
  }
  h2 {
    margin: 0 0 8px;
    font-size: 17px;
    overflow-wrap: anywhere;
  }
  p {
    margin: 0;
    line-height: 1.45;
    overflow-wrap: anywhere;
  }
  .foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 16px;
    min-height: var(--target-min);
    align-items: center;
  }
  .foot :global(.btn),
  .foot :global(.sug) {
    min-height: var(--target-min);
  }
</style>
