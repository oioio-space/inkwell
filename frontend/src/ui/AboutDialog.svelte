<script lang="ts">
import { PreviewService } from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { t } from "../i18n";
import Icon from "./icons/Icon.svelte";

/**
 * "À propos d'Inkwell": what the person may want to check about their document's privacy. Today
 * that is how the layout engine is kept offline (the network mode of the Typst sidecar).
 */
let { open = $bindable(false) }: { open: boolean } = $props();
let dialog: HTMLDialogElement | undefined = $state();
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

const text = $derived(
  mode === "namespace"
    ? t("about.network.namespace")
    : mode === "proxy only"
      ? t("about.network.proxy")
      : t("about.network.unknown"),
);
</script>

<dialog bind:this={dialog} onclose={() => (open = false)} aria-labelledby="about-title">
  <header>
    <h2 id="about-title">{t("about.title")}</h2>
    <button type="button" class="close" aria-label={t("about.close")} onclick={() => (open = false)}>
      <span class="disc"><Icon name="close" size={12} /></span>
    </button>
  </header>
  <section class="boxed">
    <h3>{t("about.offline")}</h3>
    <p data-testid="network-mode" data-mode={mode ?? "…"}>{mode === undefined ? "…" : text}</p>
  </section>
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
  h3 {
    margin: 0 0 var(--space-2);
    font: 700 14px/1.3 var(--font-ui);
  }
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
    margin: var(--space-3) 0 0;
    padding: var(--space-3);
    border-radius: var(--radius);
    background: var(--card-bg);
    box-shadow: 0 0 0 1px var(--card-shade);
  }
  p {
    margin: 0;
    font-size: 14px;
    line-height: 1.45;
  }
</style>
