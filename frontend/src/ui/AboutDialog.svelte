<script lang="ts">
import { t } from "../i18n";
import { session } from "./editorSession.svelte";
import Icon from "./icons/Icon.svelte";

/** What `inkwell --version` prints, as Go sends it (`Session.AppVersion`). */
export type BuildInfo = { Version: string; Commit: string; Date: string };

/** The About dialog (modal): name, version and licence. Empty until the version is known. */
let { open = $bindable(false), info }: { open: boolean; info?: BuildInfo | undefined } = $props();
let dialog: HTMLDialogElement | undefined = $state();

/** The product name is a brand, not a translated text. */
const NAME = "Inkwell";

$effect(() => {
  if (open && !dialog?.open) dialog?.showModal();
  if (!open && dialog?.open) dialog.close();
});

/** The short commit and the day, as "Révision 0123456, 2026-10-26"; empty when neither is known. */
const build = $derived.by(() => {
  const commit = info?.Commit.slice(0, 7) ?? "";
  const day = info?.Date.split("T")[0] ?? "";
  return commit && day ? t("about.build", { commit, date: day }) : commit || day;
});
</script>

<dialog
  bind:this={dialog}
  aria-labelledby="about-title"
  onclose={() => {
    open = false;
    session.focus();
  }}
>
  <header>
    <h2 id="about-title">{t("about.title")}</h2>
    <button type="button" class="close" aria-label={t("about.close")} onclick={() => (open = false)}>
      <span class="disc"><Icon name="close" size={12} /></span>
    </button>
  </header>

  <div class="body">
    <p class="name">{NAME}</p>
    <p class="tagline">{t("about.tagline")}</p>
    {#if info}
      <p class="version">{t("about.version", { version: info.Version })}</p>
      {#if build}<p class="build">{build}</p>{/if}
    {/if}
    <p class="license">{t("about.license")}</p>
  </div>
</dialog>

<style>
  dialog {
    box-sizing: border-box;
    width: min(380px, calc(100vw - 32px));
    padding: 0 22px 26px;
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
  .body {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    text-align: center;
  }
  p {
    margin: 0;
  }
  .name {
    font: 800 26px/1.2 var(--font-ui);
  }
  .tagline,
  .build,
  .license {
    color: var(--fg-dim);
    font-size: 13px;
  }
  .version {
    margin-top: 10px;
    font-weight: 700;
  }
</style>
