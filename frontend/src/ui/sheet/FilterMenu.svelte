<script lang="ts">
import { tick } from "svelte";
import { type ColumnFilter, distinctValues } from "../../editor/csv/view";
import { t } from "../../i18n";
import Icon from "../icons/Icon.svelte";
import type { SheetController } from "./sheet.svelte";

/**
 * The dropdown of a column header, as in a spreadsheet's AutoFilter: sort, a searchable
 * checklist of the column's values, and number conditions. Everything here is a view: the
 * file only changes if the person applies the sort to it.
 */
let {
  ctrl,
  col,
  anchor,
  onclose,
}: { ctrl: SheetController; col: number; anchor: HTMLElement; onclose: () => void } = $props();

// The menu is opened for one column and rebuilt for the next: its starting values are read once.
// svelte-ignore state_referenced_locally
const current = ctrl.view.filters.get(col);
// svelte-ignore state_referenced_locally
const all = distinctValues(ctrl.model, col, 400);
let search = $state("");
let picked = $state(new Set<string>(current?.values ?? all.map((v) => v.value)));
let op = $state<"" | ">" | "<" | "=" | "between">(current?.cond?.op ?? "");
let a = $state(current?.cond ? String(current.cond.a) : "");
let b = $state(current?.cond?.b !== undefined ? String(current.cond.b) : "");
let panel: HTMLDivElement | undefined = $state();
let left = $state(0);
let top = $state(0);

const shown = $derived(
  all.filter((v) => (v.value || t("sheet.blank")).toLowerCase().includes(search.toLowerCase())),
);
const everything = $derived(all.every((v) => picked.has(v.value)));

$effect(() => {
  if (!panel) return;
  const host = anchor.getBoundingClientRect();
  const box = panel.getBoundingClientRect();
  left = Math.max(8, Math.min(host.left, window.innerWidth - box.width - 8));
  top = Math.min(host.bottom + 6, window.innerHeight - box.height - 8);
  void tick().then(() => panel?.querySelector<HTMLElement>("input[type=search]")?.focus());
});

function toggle(v: string) {
  const next = new Set(picked);
  if (next.has(v)) next.delete(v);
  else next.add(v);
  picked = next;
}

function toggleAll() {
  picked = everything ? new Set() : new Set(all.map((v) => v.value));
}

function apply() {
  const f: ColumnFilter = {};
  if (!everything) f.values = new Set(picked);
  const n = Number(a.replace(",", "."));
  const m = Number(b.replace(",", "."));
  if (op && a.trim() !== "" && Number.isFinite(n)) {
    f.cond = op === "between" ? { op, a: n, b: Number.isFinite(m) ? m : n } : { op, a: n };
  }
  ctrl.setFilter(col, f.values || f.cond ? f : undefined);
  onclose();
}

function key(e: KeyboardEvent) {
  if (e.key === "Escape") {
    e.preventDefault();
    e.stopPropagation();
    onclose();
  } else if (e.key === "Enter" && (e.target as HTMLElement).tagName === "INPUT") {
    e.preventDefault();
    apply();
  }
}

function outside(e: PointerEvent) {
  const el = e.target as Node;
  if (!panel?.contains(el) && !anchor.contains(el)) onclose();
}
</script>

<svelte:window onpointerdown={outside} />

<div
  bind:this={panel}
  class="pop filter"
  role="dialog"
  aria-label={t("sheet.filterColumn", { name: ctrl.headerName(col) })}
  tabindex="-1"
  style:left="{left}px"
  style:top="{top}px"
  onkeydown={key}
>
  <button type="button" class="mi" onclick={() => { ctrl.sortBy(col, 1); onclose(); }}>
    <Icon name="sortasc" /><span>{t("sheet.sortAsc")}</span>
  </button>
  <button type="button" class="mi" onclick={() => { ctrl.sortBy(col, -1); onclose(); }}>
    <Icon name="sortdesc" /><span>{t("sheet.sortDesc")}</span>
  </button>
  {#if ctrl.view.sort?.col === col}
    <button type="button" class="mi" onclick={() => { ctrl.clearSort(); onclose(); }}>
      <Icon name="close" /><span>{t("sheet.clearSort")}</span>
    </button>
  {/if}
  <div class="sep"></div>
  <div class="cap">{t("sheet.conditions")}</div>
  <div class="cond">
    <select bind:value={op} aria-label={t("sheet.condition")}>
      <option value="">{t("sheet.cond.none")}</option>
      <option value=">">{t("sheet.cond.gt")}</option>
      <option value="<">{t("sheet.cond.lt")}</option>
      <option value="=">{t("sheet.cond.eq")}</option>
      <option value="between">{t("sheet.cond.between")}</option>
    </select>
    {#if op}
      <input type="text" inputmode="decimal" bind:value={a} aria-label={t("sheet.cond.value")} />
    {/if}
    {#if op === "between"}
      <input type="text" inputmode="decimal" bind:value={b} aria-label={t("sheet.cond.to")} />
    {/if}
  </div>
  <div class="sep"></div>
  <input type="search" class="find" bind:value={search} placeholder={t("sheet.searchValues")} aria-label={t("sheet.searchValues")} />
  <label class="mi check">
    <input type="checkbox" checked={everything} onchange={toggleAll} />
    <span>{t("sheet.selectAllValues")}</span>
  </label>
  <div class="list" role="group" aria-label={t("sheet.values")}>
    {#each shown as v (v.value)}
      <label class="mi check">
        <input type="checkbox" checked={picked.has(v.value)} onchange={() => toggle(v.value)} />
        <span class="val">{v.value || t("sheet.blank")}</span>
        <span class="n">{v.count}</span>
      </label>
    {/each}
  </div>
  <div class="foot">
    <button type="button" class="btn" onclick={() => { ctrl.setFilter(col, undefined); onclose(); }}>{t("sheet.clearFilter")}</button>
    <button type="button" class="sug" onclick={apply}>{t("sheet.apply")}</button>
  </div>
</div>

<style>
  .filter {
    position: fixed;
    z-index: 60;
    width: 280px;
    max-height: min(560px, calc(100vh - 24px));
    overflow: auto;
  }
  .filter::before {
    display: none;
  }
  .cond {
    display: flex;
    gap: 6px;
    padding: 0 6px 6px;
  }
  .cond select,
  .cond input,
  .find {
    min-height: 30px;
    box-sizing: border-box;
    border: 0;
    border-radius: var(--radius-small);
    background: var(--btn-bg);
    color: var(--fg);
    padding: 0 8px;
    font: inherit;
  }
  .cond input {
    width: 70px;
  }
  .find {
    width: calc(100% - 12px);
    margin: 4px 6px;
  }
  .list {
    max-height: 220px;
    overflow: auto;
  }
  .check {
    min-height: 30px;
    cursor: default;
  }
  .val {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .n {
    color: var(--fg-dim);
    font-size: 12px;
  }
  .foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding: 8px 6px 2px;
  }
</style>
