<script lang="ts">
import type { FileType } from "../../filetypes";
import { t } from "../../i18n";
import FileIcon from "../icons/FileIcon.svelte";
import { formatSize } from "./explain";

/**
 * What the editor area shows for a file that is not text: what it is, how big it is, and the way
 * out, the system's own program for it. The bytes are never drawn as characters.
 */
let {
  name,
  size,
  type,
  openWithSystem,
}: {
  name: string;
  size: number;
  type: FileType;
  openWithSystem?: () => void;
} = $props();
</script>

<section class="card-view" aria-label={name}>
  <div class="big"><FileIcon mark={type.icon} size={48} /></div>
  <h2>{name}</h2>
  <p class="title">{t("blob.title")}</p>
  <p class="info">{t("blob.info", { kind: t(type.label), size: formatSize(size) })}</p>
  <p class="hint">{t("blob.hint")}</p>
  {#if openWithSystem}
    <button type="button" class="btn" onclick={openWithSystem}>{t("blob.open")}</button>
  {/if}
</section>

<style>
  .card-view {
    display: grid;
    justify-items: center;
    align-content: center;
    gap: var(--space-2);
    height: 100%;
    padding: var(--space-4);
    background: var(--view-bg);
    text-align: center;
  }
  h2 {
    margin: 0;
    font: 700 16px/1.3 var(--font-ui);
    overflow-wrap: anywhere;
  }
  .title {
    margin: 0;
    font-weight: 600;
  }
  .info {
    margin: 0;
    color: var(--fg-dim);
  }
  .hint {
    max-width: 36ch;
    margin: 0;
    color: var(--fg-dim);
    font-size: 13px;
  }
  .card-view :global(.btn) {
    min-height: var(--target-min);
    margin-top: var(--space-2);
  }
</style>
