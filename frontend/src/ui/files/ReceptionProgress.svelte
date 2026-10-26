<script lang="ts">
import { t } from "../../i18n";
import type { ProjectRuntime } from "../../project/runtime.svelte";
import { formatSize } from "./explain";

/**
 * « Réception : 12 sur 40 fichiers, 3,2 Mo » while the project's files arrive (PRJ-01, PRJ-02),
 * the question when the project is over its quota (PRJ-11), and the files that were not received
 * because they are too large.
 */
let { runtime }: { runtime: ProjectRuntime } = $props();

const p = $derived(runtime.progress);
const receiving = $derived(p.total > 0 && p.done < p.total);
</script>

{#if receiving || p.needsConfirmation || p.refused.length > 0 || runtime.treeRejected > 0}
  <div class="progress" role="status" aria-live="polite">
    {#if receiving}
      <span class="line">{t("files.progress", { done: p.done, total: p.total, size: formatSize(p.doneBytes) })}</span>
      <progress max={p.total} value={p.done} aria-label={t("files.progress", { done: p.done, total: p.total, size: formatSize(p.doneBytes) })}></progress>
    {/if}
    {#if p.needsConfirmation}
      <span class="line">{t("files.progress.confirm", { limit: formatSize(p.limit) })}</span>
      <button type="button" class="btn" onclick={() => void runtime.confirmDownload()}>
        {t("files.progress.go")}
      </button>
    {/if}
    {#each p.refused as path (path)}
      <span class="line warn">{t("files.progress.refused", { path, limit: formatSize(100 * 1024 * 1024) })}</span>
    {/each}
    {#if runtime.treeRejected > 0}
      <span class="line warn">{t("files.warn.rejected", { count: runtime.treeRejected })}</span>
    {/if}
  </div>
{/if}

<style>
  .progress {
    display: flex;
    flex: none;
    flex-direction: column;
    gap: 6px;
    padding: 8px 12px 10px;
    border-top: 1px solid var(--border);
    font-size: 13px;
  }
  .line.warn {
    color: var(--warning);
  }
  progress {
    width: 100%;
    height: 6px;
    accent-color: var(--accent-bg);
  }
  .progress :global(.btn) {
    align-self: flex-start;
    min-height: 32px;
  }
</style>
