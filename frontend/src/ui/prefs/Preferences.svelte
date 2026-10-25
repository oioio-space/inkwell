<script lang="ts">
import { t, tn } from "../../i18n";
import {
  type ExternalPolicy,
  SAVE_DELAYS,
  type SaveDelay,
  type SaveMode,
  settings,
} from "../../store/settings.svelte";
import { session } from "../editorSession.svelte";
import Icon from "../icons/Icon.svelte";

/**
 * Preferences (a modal dialog). The values are stored in `settings`; what they change — when
 * files are written, what happens to outside edits — is applied by the saving work.
 */
let { open = $bindable(false) }: { open: boolean } = $props();
let dialog: HTMLDialogElement | undefined = $state();

$effect(() => {
  if (open && !dialog?.open) dialog?.showModal();
  if (!open && dialog?.open) dialog.close();
});

const MODES: [SaveMode, "prefs.save.auto" | "prefs.save.manual"][] = [
  ["auto", "prefs.save.auto"],
  ["manual", "prefs.save.manual"],
];
const POLICIES: ExternalPolicy[] = ["ask", "import", "keep"];

function onChange(e: Event, set: (value: string) => void) {
  set((e.currentTarget as HTMLSelectElement).value);
}
</script>

<dialog
  bind:this={dialog}
  aria-labelledby="prefs-title"
  onclose={() => {
    open = false;
    session.focus();
  }}
>
  <header>
    <h2 id="prefs-title">{t("prefs.title")}</h2>
    <button type="button" class="close" aria-label={t("prefs.close")} onclick={() => (open = false)}>
      <span class="disc"><Icon name="close" size={12} /></span>
    </button>
  </header>

  <h3>{t("prefs.save.title")}</h3>
  <div class="card boxed">
    <div class="row">
      <span id="save-mode">{t("prefs.save.mode")}</span>
      <div class="tg" role="radiogroup" aria-labelledby="save-mode">
        {#each MODES as [mode, label] (mode)}
          <button
            type="button"
            role="radio"
            aria-checked={settings.saveMode === mode}
            onclick={() => settings.setSaveMode(mode)}
          >
            {t(label)}
          </button>
        {/each}
      </div>
    </div>
    <label class="row" class:off={settings.saveMode === "manual"}>
      <span>{t("prefs.save.delay")}</span>
      <select
        class="btn"
        disabled={settings.saveMode === "manual"}
        value={String(settings.saveDelay)}
        onchange={(e) => onChange(e, (v) => settings.setSaveDelay(Number(v) as SaveDelay))}
      >
        {#each SAVE_DELAYS as seconds (seconds)}
          <option value={String(seconds)}>{tn("prefs.save.seconds", seconds)}</option>
        {/each}
      </select>
    </label>
  </div>
  <p class="note">{t("prefs.save.note")}</p>

  <h3>{t("prefs.external.title")}</h3>
  <div class="card boxed">
    <label class="row">
      <span class="text">
        <span>{t("prefs.external.when")}</span>
        <span class="sub">{t("prefs.external.hint")}</span>
      </span>
      <select
        class="btn"
        value={settings.externalPolicy}
        onchange={(e) => onChange(e, (v) => settings.setExternalPolicy(v as ExternalPolicy))}
      >
        {#each POLICIES as policy (policy)}
          <option value={policy}>{t(`prefs.external.${policy}`)}</option>
        {/each}
      </select>
    </label>
  </div>
  <p class="note">{t("prefs.external.note")}</p>
</dialog>

<style>
  dialog {
    box-sizing: border-box;
    width: min(520px, calc(100vw - 32px));
    max-height: calc(100vh - 32px);
    padding: 0 22px 22px;
    border: 0;
    border-radius: var(--radius-window);
    background: var(--window-bg);
    color: var(--fg);
    box-shadow:
      0 0 0 1px var(--border),
      0 12px 40px rgb(0 0 0 / 30%);
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 30%);
  }
  header {
    position: relative;
    display: grid;
    place-items: center;
    height: 56px;
    margin: 0 -22px;
  }
  h2 {
    margin: 0;
    font: 700 14.5px/1.3 var(--font-ui);
  }
  /* A 44 px target around a 26 px round button. */
  .close {
    position: absolute;
    inset-inline-end: 8px;
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
  h3 {
    margin: 20px 4px 8px;
    font: 700 14.5px/1.3 var(--font-ui);
  }
  .row {
    min-height: 52px;
    justify-content: space-between;
  }
  .row.off {
    opacity: 0.45;
  }
  .text {
    display: flex;
    flex-direction: column;
    line-height: 1.35;
  }
  .tg button {
    min-height: 36px;
  }
  select {
    min-height: var(--target-min);
    padding: 0 12px;
    font: inherit;
    font-weight: 600;
  }
  .note {
    margin: 8px 4px 0;
    color: var(--fg-dim);
    font-size: 13px;
    line-height: 1.5;
  }
</style>
