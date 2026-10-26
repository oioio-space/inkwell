<script lang="ts">
import { sizeOf } from "../../editor/csv/clipboard";
import { t, tn } from "../../i18n";

/** Asks before a big paste (more than 1 000 cells), with a look at what will land. */
let { rows, onyes, onno }: { rows: string[][]; onyes: () => void; onno: () => void } = $props();

let dialog: HTMLDialogElement | undefined = $state();
const [h, w] = $derived(sizeOf(rows));
const preview = $derived(rows.slice(0, 5).map((r) => r.slice(0, 6)));

$effect(() => {
  if (dialog && !dialog.open) dialog.showModal();
});
</script>

<dialog bind:this={dialog} aria-labelledby="paste-title" onclose={onno}>
  <h2 id="paste-title">{t("sheet.paste.title")}</h2>
  <p>{t("sheet.paste.size", { rows: h, cols: w })} ({tn("sheet.cells", h * w)})</p>
  <table class="preview" aria-label={t("sheet.paste.preview")}>
    <tbody>
      {#each preview as row, i (i)}
        <tr>
          {#each row as v, j (j)}<td>{v}</td>{/each}
        </tr>
      {/each}
    </tbody>
  </table>
  <div class="foot">
    <button type="button" class="btn" onclick={onno}>{t("sheet.paste.no")}</button>
    <button type="button" class="sug" onclick={onyes}>{t("sheet.paste.yes")}</button>
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
    max-width: min(560px, calc(100vw - 32px));
  }
  dialog::backdrop {
    background: rgb(0 0 6 / 35%);
  }
  h2 {
    margin: 0 0 8px;
    font-size: 17px;
  }
  .preview {
    border-collapse: collapse;
    margin: 8px 0;
    font-size: 13px;
  }
  .preview td {
    max-width: 120px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    padding: 3px 8px;
    border: 1px solid var(--border);
  }
  .foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 12px;
    min-height: var(--target-min);
    align-items: center;
  }
</style>
