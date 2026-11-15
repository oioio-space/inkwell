<script lang="ts">
import { t } from "../../i18n";

/**
 * Asks before « Reprendre ce document ici » (ADR 0043, part 2): it says that Inkwell restarts and
 * what the people of the old document lose. One question, answered by yes or no.
 */
let { onyes, onno }: { onyes: () => void; onno: () => void } = $props();

let dialog: HTMLDialogElement | undefined = $state();

$effect(() => {
  if (dialog && !dialog.open) dialog.showModal();
});
</script>

<dialog bind:this={dialog} aria-labelledby="take-over-title" aria-describedby="take-over-body" onclose={onno}>
  <h2 id="take-over-title">{t("foreign.takeOver.confirm.title")}</h2>
  <p id="take-over-body">{t("foreign.takeOver.confirm.body")}</p>
  <div class="foot">
    <button type="button" class="btn" onclick={onno}>{t("people.cancel")}</button>
    <button type="button" class="sug" data-testid="take-over-confirm" onclick={onyes}>{t("foreign.takeOver")}</button>
  </div>
</dialog>

<style>
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
  }
  p {
    margin: 0;
    line-height: 1.45;
  }
  .foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 16px;
    min-height: var(--target-min);
    align-items: center;
  }
</style>
