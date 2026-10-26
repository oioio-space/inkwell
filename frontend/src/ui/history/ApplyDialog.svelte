<script lang="ts">
import { compareTexts } from "../../history/diffview";
import { moveSections } from "../../history/stamp";
import { summarize } from "../../history/summary";
import { t } from "../../i18n";
import DiffInline from "./DiffInline.svelte";
import { type ApplyRequest, history } from "./history.svelte";

/** What going back, or undoing one change, would do: shown first, applied on a yes. */
let { req }: { req: ApplyRequest } = $props();
let dialog: HTMLDialogElement | undefined = $state();

const diff = $derived(compareTexts(req.before, req.after, undefined));
const summary = $derived(summarize(req.before, req.after, diff.ops));
const moves = $derived(moveSections(req.before, req.after, diff.ops));
const same = $derived(req.before === req.after);

$effect(() => {
  dialog?.showModal();
  return () => dialog?.close();
});

const cancel = () => {
  history.apply = undefined;
};
</script>

<dialog bind:this={dialog} onclose={cancel} aria-labelledby="apply-title" data-testid="apply-dialog">
  <header>
    <h2 id="apply-title">{req.title}</h2>
    <p class="hint">{t(req.kind === "revert" ? "history.revert.hint" : "history.undo.hint")}</p>
    <p class="summary" data-testid="apply-summary">{same ? t("history.apply.same") : summary.text}</p>
  </header>
  <div class="body">
    <DiffInline frags={diff.frags} hunks={diff.hunks} author={history.author} {moves} label={t("history.apply.preview")} />
  </div>
  <footer>
    <button type="button" class="btn" onclick={cancel}>{t("history.apply.cancel")}</button>
    <button type="button" class="sug" disabled={same} onclick={req.confirm}>
      {t(req.kind === "revert" ? "history.revert.button" : "history.undo.button")}
    </button>
  </footer>
</dialog>

<style>
  dialog {
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    width: min(760px, calc(100vw - 32px));
    max-height: calc(100vh - 48px);
    padding: 0;
    border: 0;
    border-radius: var(--radius);
    background: var(--window-bg);
    color: var(--fg);
    box-shadow: 0 0 0 1px var(--border), 0 12px 32px rgb(0 0 0 / 25%);
  }
  dialog:not([open]) {
    display: none;
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 30%);
  }
  header {
    padding: 18px 20px 8px;
  }
  h2 {
    margin: 0;
    font: 700 15px/1.4 var(--font-ui);
  }
  .hint {
    margin: 4px 0 0;
    color: var(--fg-dim);
    font: 13px/1.5 var(--font-ui);
  }
  .summary {
    margin: 8px 0 0;
    font: 15px/1.5 var(--font-ui);
  }
  .body {
    flex: 1;
    min-height: 0;
    overflow: auto;
    margin: 4px 20px 0;
    border-radius: var(--radius-small);
    background: var(--view-bg);
    box-shadow: 0 0 0 1px var(--border);
    max-height: 340px;
  }
  footer {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding: 14px 20px;
  }
</style>
