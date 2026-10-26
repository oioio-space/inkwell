<script lang="ts">
import type { Delimiter } from "../../editor/csv/records";
import { formatFr } from "../../editor/csv/stats";
import { t, tn } from "../../i18n";
import type { FileInfo } from "../../store/files";
import type { Status } from "../../workspace/workspace.svelte";
import EncodingMenu from "../popover/EncodingMenu.svelte";
import MenuButton from "../popover/MenuButton.svelte";
import type { SheetController } from "./sheet.svelte";

/**
 * The status bar of a sheet: people and sync, the statistics of the selection (which ones
 * show is the person's choice), the file's way of being written (separator, header, decimal
 * sign: detected, and correctable here), its encoding, and the zoom.
 */
let {
  ctrl,
  status,
  peerCount,
  info,
}: { ctrl: SheetController; status: Status; peerCount: number; info: FileInfo | undefined } =
  $props();

let statsOpen = $state(false);
let dialectOpen = $state(false);

const DELIMS: {
  value: Delimiter;
  key: "sheet.delim.comma" | "sheet.delim.semicolon" | "sheet.delim.tab" | "sheet.delim.pipe";
}[] = [
  { value: ",", key: "sheet.delim.comma" },
  { value: ";", key: "sheet.delim.semicolon" },
  { value: "\t", key: "sheet.delim.tab" },
  { value: "|", key: "sheet.delim.pipe" },
];

const d = $derived(ctrl.dialect);
const delimLabel = $derived(
  t(DELIMS.find((x) => x.value === d.delimiter)?.key ?? "sheet.delim.comma"),
);
const uneven = $derived.by(() => {
  ctrl.tick;
  return ctrl.model.index.uneven;
});
const stats = $derived(ctrl.stats);
const n = (x: number) => formatFr(x);
const parts = $derived.by(() => {
  const s = stats;
  if (!s) return [];
  const out: { key: string; text: string }[] = [];
  if (ctrl.shown.count) out.push({ key: "count", text: t("sheet.stat.count", { n: s.count }) });
  if (s.numeric > 0) {
    if (ctrl.shown.sum) out.push({ key: "sum", text: t("sheet.stat.sum", { n: n(s.sum) }) });
    if (ctrl.shown.mean) out.push({ key: "mean", text: t("sheet.stat.mean", { n: n(s.mean) }) });
    if (ctrl.shown.min) out.push({ key: "min", text: t("sheet.stat.min", { n: n(s.min) }) });
    if (ctrl.shown.max) out.push({ key: "max", text: t("sheet.stat.max", { n: n(s.max) }) });
  }
  return out;
});
const STAT_KEYS = ["count", "sum", "mean", "min", "max"] as const;
</script>

<div class="bot" role="contentinfo" aria-label={t("status.bar")}>
  <span class="people" data-status={status}>
    <span class="dot" aria-hidden="true"></span>
    {peerCount === 0 ? t("status.alone") : tn("people.count", peerCount + 1)}
  </span>
  {#if uneven > 0}
    <span class="fb warn" aria-live="polite" data-testid="uneven">{tn("sheet.uneven", uneven)}</span>
  {/if}
  {#if ctrl.problems.length > 0}
    {@const first = ctrl.problems[0]}
    <span class="fb warn" aria-live="polite" data-testid="problems" title={first ? t(`sheet.problem.${first.kind}`) : ""}>
      {ctrl.problems.length === 1
        ? t("sheet.problems.one", { line: first?.line ?? 0 })
        : t("sheet.problems.other", { count: ctrl.problems.length, line: first?.line ?? 0 })}
    </span>
  {/if}
  <span class="sp"></span>
  <MenuButton bind:open={statsOpen} up align="end" label={t("sheet.stat.choose")}>
    {#snippet trigger()}
      <span data-testid="stats">
        {#if parts.length === 0}{t("sheet.stat.none")}{:else}{parts.map((p) => p.text).join("  ·  ")}{/if}
      </span>
    {/snippet}
    {#snippet children()}
      <div class="cap">{t("sheet.stat.choose")}</div>
      {#each STAT_KEYS as k (k)}
        <button
          type="button"
          role="menuitemcheckbox"
          class="mi"
          aria-checked={ctrl.shown[k]}
          onclick={() => (ctrl.shown[k] = !ctrl.shown[k])}
        >
          <span class="tick">{ctrl.shown[k] ? "✓" : ""}</span>
          <span>{t(`sheet.stat.name.${k}`)}</span>
        </button>
      {/each}
    {/snippet}
  </MenuButton>
  <MenuButton bind:open={dialectOpen} up align="end" width={300} label={t("sheet.dialect")}>
    {#snippet trigger()}
      <span data-testid="dialect">{delimLabel}{d.header ? `, ${t("sheet.header.short")}` : ""}</span>
    {/snippet}
    {#snippet children({ close })}
      <div class="cap">{t("sheet.delimiter")}</div>
      <p class="sub conf" data-testid="confidence">{ctrl.confidence === "high" ? t("sheet.confidence.high") : t("sheet.confidence.low")}</p>
      {#each DELIMS as x (x.value)}
        <button
          type="button"
          role="menuitemradio"
          class="mi"
          aria-checked={d.delimiter === x.value}
          onclick={() => { ctrl.setDialect({ ...d, delimiter: x.value }); close(); }}
        >
          <span class="tick">{d.delimiter === x.value ? "●" : ""}</span><span>{t(x.key)}</span>
        </button>
      {/each}
      <div class="sep"></div>
      <button
        type="button"
        role="menuitemcheckbox"
        class="mi"
        aria-checked={d.header}
        onclick={() => { ctrl.setDialect({ ...d, header: !d.header }); close(); }}
      >
        <span class="tick">{d.header ? "✓" : ""}</span><span>{t("sheet.header.first")}</span>
      </button>
      <button
        type="button"
        role="menuitemcheckbox"
        class="mi"
        aria-checked={ctrl.model.quoting}
        onclick={() => { ctrl.setDialect({ ...d, quote: !ctrl.model.quoting }); close(); }}
      >
        <span class="tick">{ctrl.model.quoting ? "✓" : ""}</span><span>{t("sheet.quotes")}</span>
      </button>
      <div class="cap">{t("sheet.decimal")}</div>
      {#each [",", "."] as sign (sign)}
        <button
          type="button"
          role="menuitemradio"
          class="mi"
          aria-checked={d.decimal === sign}
          onclick={() => { ctrl.setDialect({ ...d, decimal: sign as "," | "." }); close(); }}
        >
          <span class="tick">{d.decimal === sign ? "●" : ""}</span><span>{sign === "," ? t("sheet.decimal.comma") : t("sheet.decimal.point")}</span>
        </button>
      {/each}
      {#if !ctrl.readOnly}
        <div class="sep"></div>
        <div class="cap">{t("sheet.convert")}</div>
        {#each DELIMS.filter((x) => x.value !== d.delimiter) as x (x.value)}
          <button type="button" role="menuitem" class="mi" onclick={() => { ctrl.convertDelimiter(x.value); close(); }}>
            <span class="tick"></span><span>{t("sheet.convertTo", { to: t(x.key) })}</span>
          </button>
        {/each}
      {/if}
    {/snippet}
  </MenuButton>
  <EncodingMenu {info} />
  <span class="zoom">
    <button type="button" class="fb" aria-label={t("sheet.zoomOut")} onclick={() => ctrl.setZoom(ctrl.zoom - 0.1)}>−</button>
    <button type="button" class="fb pct" aria-label={t("sheet.zoomReset")} data-testid="zoom" onclick={() => ctrl.setZoom(1)}>{Math.round(ctrl.zoom * 100)} %</button>
    <button type="button" class="fb" aria-label={t("sheet.zoomIn")} onclick={() => ctrl.setZoom(ctrl.zoom + 0.1)}>+</button>
  </span>
</div>

<style>
  .bot {
    box-sizing: border-box;
    display: flex;
    align-items: center;
    gap: 2px;
    height: 36px;
    padding: 0 6px;
    background: var(--headerbar-bg);
    box-shadow: 0 -1px var(--shade);
    color: var(--fg-dim);
    font-size: 12.5px;
    position: relative;
    z-index: 2;
  }
  .bot :global(.fb) {
    height: 28px;
    padding: 0 10px;
    color: var(--fg);
    font-variant-numeric: tabular-nums;
    opacity: 0.85;
  }
  .sp {
    flex: 1;
  }
  .people {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 0 10px;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--fg-dim);
  }
  [data-status="synced"] .dot {
    background: var(--success);
  }
  .warn {
    color: var(--warning);
    opacity: 1;
  }
  .conf {
    margin: 0;
    padding: 0 10px 4px;
  }
  .tick {
    width: 14px;
    flex: none;
    text-align: center;
  }
  .zoom {
    display: inline-flex;
    align-items: center;
  }
  .pct {
    min-width: 54px;
  }
</style>
