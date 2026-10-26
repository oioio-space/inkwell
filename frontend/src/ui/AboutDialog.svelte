<script lang="ts">
import { PreviewService } from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
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

/** How the layout engine is kept offline (the Typst sidecar's network mode); undefined while asking. */
let mode = $state<string | undefined>();

$effect(() => {
  if (open && !dialog?.open) dialog?.showModal();
  if (!open && dialog?.open) dialog.close();
});

$effect(() => {
  if (!open) return;
  mode = undefined;
  let current = true;
  PreviewService.NetworkMode()
    .then((m) => {
      if (current) mode = m;
    })
    .catch(() => {
      if (current) mode = "";
    });
  return () => {
    current = false;
  };
});

const networkText = $derived(
  mode === "namespace"
    ? t("about.network.namespace")
    : mode === "proxy only"
      ? t("about.network.proxy")
      : t("about.network.unknown"),
);

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
    <section class="offline">
      <h3>{t("about.offline")}</h3>
      <p data-testid="network-mode" data-mode={mode ?? "…"}>{mode === undefined ? "…" : networkText}</p>
    </section>
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
  .offline {
    margin-top: 12px;
    padding: 10px 12px;
    border-radius: var(--radius);
    background: var(--card-bg);
    box-shadow: 0 0 0 1px var(--card-shade);
  }
  h3 {
    margin: 0 0 4px;
    font: 700 13px/1.3 var(--font-ui);
  }
  .offline p {
    font-size: 13px;
  }
  .version {
    margin-top: 10px;
    font-weight: 700;
  }
</style>
