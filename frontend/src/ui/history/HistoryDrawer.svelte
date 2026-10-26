<script lang="ts">
import type { Period } from "../../history/filters";
import { stamp } from "../../history/stamp";
import { t, tn } from "../../i18n";
import { workspace } from "../../workspace/workspace.svelte";
import ApplyDialog from "./ApplyDialog.svelte";
import DiffView from "./DiffView.svelte";
import HistoryRow from "./HistoryRow.svelte";
import { history, ROW_HEIGHT } from "./history.svelte";

/** The right drawer: history, comments, activity (tabs as in the mockup). */
let scroller: HTMLDivElement | undefined = $state();
let top = $state(0);
let height = $state(400);
let nameInput = $state("");

const rows = $derived(history.rows);
const start = $derived(Math.max(0, Math.floor(top / ROW_HEIGHT) - 4));
const end = $derived(Math.min(rows.length, Math.ceil((top + height) / ROW_HEIGHT) + 4));
const selected = $derived(history.rowOf(history.selected));
const why = $derived(history.readOnly ? t("history.readonly") : undefined);
const periods: Period[] = ["all", "today", "week", "month"];

$effect(() => {
  if (history.naming) nameInput = "";
});

function pick(key: string) {
  history.selected = key;
}

function onListKey(e: KeyboardEvent) {
  const i = rows.findIndex((r) => r.key === history.selected);
  if (e.key === "ArrowDown" || e.key === "ArrowUp") {
    e.preventDefault();
    const n = rows[Math.min(rows.length - 1, Math.max(0, i + (e.key === "ArrowDown" ? 1 : -1)))];
    if (n) {
      history.selected = n.key;
      const y = rows.indexOf(n) * ROW_HEIGHT;
      if (scroller && (y < scroller.scrollTop || y + ROW_HEIGHT > scroller.scrollTop + height))
        scroller.scrollTop = y - height / 2;
    }
  }
}

const tabs = ["history", "comments", "activity"] as const;

/** Events other than text changes, newest first: the feed of the « Activité » tab. */
const activity = $derived.by(() => {
  const doc = history.doc;
  if (!doc) return [];
  const out: { key: string; text: string; when: number }[] = [];
  for (const v of doc.meta?.versions ?? []) {
    if (!v.auto)
      out.push({
        key: `v${v.at}${v.name}`,
        text: t("history.act.version", { name: history.author(v.by).name, version: v.name }),
        when: v.at,
      });
  }
  for (const v of doc.meta?.variants ?? []) {
    out.push({
      key: `b${v.url}`,
      text: t("history.act.variant", { name: history.author(v.by).name, variant: v.name }),
      when: v.at,
    });
  }
  for (const s of history.entries) {
    if (s.action)
      out.push({
        key: `s${s.id}`,
        text: `${history.author(s.actor).name} · ${history.actionTitle(s)}`,
        when: s.end,
      });
  }
  return out.sort((a, b) => b.when - a.when);
});
</script>

{#if history.open}
  <aside class="drawer" aria-label={t("history.title")} data-testid="history-drawer">
    <div class="tg tabs" role="tablist" aria-label={t("history.title")}>
      {#each tabs as tab}
        <button type="button" role="tab" aria-selected={history.tab === tab} onclick={() => (history.tab = tab)}>
          {t(`history.tab.${tab}`)}
        </button>
      {/each}
    </div>

    {#if history.tab === "history"}
      <div class="filters">
        <label>
          <span class="vh">{t("history.filter.person")}</span>
          <select aria-label={t("history.filter.person")} value={history.filters.person ?? ""} onchange={(e) => (history.filters = { ...history.filters, person: e.currentTarget.value || undefined })}>
            <option value="">{t("history.filter.everyone")}</option>
            {#each history.people as p}<option value={p}>{p}</option>{/each}
          </select>
        </label>
        <label>
          <span class="vh">{t("history.filter.period")}</span>
          <select aria-label={t("history.filter.period")} value={history.filters.period} onchange={(e) => (history.filters = { ...history.filters, period: e.currentTarget.value as Period })}>
            {#each periods as p}<option value={p}>{t(`history.period.${p}`)}</option>{/each}
          </select>
        </label>
        <label>
          <span class="vh">{t("history.filter.section")}</span>
          <select aria-label={t("history.filter.section")} value={history.filters.section ?? ""} onchange={(e) => (history.filters = { ...history.filters, section: e.currentTarget.value || undefined })}>
            <option value="">{t("history.filter.allSections")}</option>
            {#each history.sections as s}<option value={s}>{s}</option>{/each}
          </select>
        </label>
        <label>
          <span class="vh">{t("history.filter.file")}</span>
          <select aria-label={t("history.filter.file")} value={history.filters.file ?? ""} onchange={(e) => (history.filters = { ...history.filters, file: e.currentTarget.value || undefined })}>
            <option value="">{t("history.filter.allFiles")}</option>
            <option value={workspace.current}>{workspace.current}</option>
          </select>
        </label>
      </div>
      <div class="toggles">
        <label class="sw"><input type="checkbox" bind:checked={history.showWho} /> {t("history.who")}</label>
        <label class="sw"><input type="checkbox" bind:checked={history.showAbandoned} /> {t("history.abandoned")}</label>
        <span class="count" data-testid="history-count" role="status">
          {history.filtering ? t("history.counter", { shown: history.filtered.shown.length, total: history.filtered.total }) : tn("history.entries", history.filtered.total)}
        </span>
      </div>
      {#if history.showWho}
        <p class="legend" data-testid="who-legend">
          {#each history.people as p}
            <span class="chip"><span class="dot" style:background="var(--person-{history.legendIndex(p)})" aria-hidden="true"></span>{p}</span>
          {/each}
          {#if history.busy}<span class="busy">{t("history.computing")}</span>{/if}
        </p>
      {/if}

      {#if history.error}<p class="note" role="alert" data-testid="history-error">{t("history.error", { reason: history.error })}</p>{/if}
      <div
        class="list"
        role="listbox"
        tabindex="0"
        aria-label={t("history.list")}
        bind:this={scroller}
        bind:clientHeight={height}
        onscroll={(e) => (top = e.currentTarget.scrollTop)}
        onkeydown={onListKey}
      >
        <div style:height="{rows.length * ROW_HEIGHT}px" style:position="relative">
          <div style:transform="translateY({start * ROW_HEIGHT}px)">
            {#each rows.slice(start, end) as row, i (row.key)}
              <HistoryRow {row} first={start + i === 0} last={start + i === rows.length - 1} selected={history.selected === row.key} onpick={() => pick(row.key)} />
            {/each}
          </div>
        </div>
      </div>

      {#if selected}
        <section class="card actions" aria-label={t("history.actions")} data-testid="history-actions">
          {#if history.naming}
            <form onsubmit={(e) => { e.preventDefault(); history.confirmNaming(nameInput); }}>
              <label>
                <span>{t(history.naming.kind === "version" ? "history.name.version" : "history.name.variant")}</span>
                <input bind:value={nameInput} maxlength="60" required />
              </label>
              <button type="submit" class="sug">{t("history.name.ok")}</button>
              <button type="button" class="btn" onclick={() => (history.naming = undefined)}>{t("history.apply.cancel")}</button>
            </form>
          {:else}
            {@const s = selected.session}
            {@const heads = s?.after ?? selected.version?.heads ?? history.analysisHeads}
            {@const label = selected.kind === "now" ? t("history.now") : (s ? history.entryTitle(s) : (selected.version?.name ?? ""))}
            <div class="btns">
              {#if s && !s.action}
                <button type="button" class="btn" onclick={() => history.viewEntry(s)}>{t("history.view")}</button>
              {/if}
              {#if selected.kind !== "now" && selected.kind !== "variant" && selected.kind !== "merge"}
                <button type="button" class="btn" onclick={() => history.compareWithNow(heads, label)}>{t("history.compare")}</button>
              {/if}
              {#if selected.kind === "variant" && selected.variant}
                {@const v = selected.variant}
                <button type="button" class="btn" onclick={() => void history.compareVariant(v)}>{t("history.compare.main")}</button>
                {#if v.state === "open"}
                  <button type="button" class="btn" onclick={() => void history.editVariant(v)}>{t("history.variant.edit")}</button>
                  <button type="button" class="sug" disabled={history.readOnly} title={why} onclick={() => history.merge(v)}>{t("history.merge")}</button>
                  <button type="button" class="btn" disabled={history.readOnly} title={why} onclick={() => history.abandon(v)}>{t("history.abandon")}</button>
                {:else if v.state === "abandoned"}
                  <button type="button" class="btn" disabled={history.readOnly} title={why} onclick={() => history.reopen(v)}>{t("history.restore")}</button>
                {/if}
              {:else if selected.kind !== "merge"}
                <button type="button" class="btn" disabled={history.readOnly} title={why} onclick={() => history.startNaming("variant", heads)}>{t("history.fork")}</button>
                {#if !s?.action}
                  <button type="button" class="btn" disabled={history.readOnly} title={why} onclick={() => history.startNaming("version", heads)}>{t("history.nameit")}</button>
                {/if}
                {#if selected.kind !== "now"}
                  <button type="button" class="sug" disabled={history.readOnly} title={why} onclick={() => history.askRevert(heads, label)}>{t("history.revert")}</button>
                {/if}
              {/if}
            </div>
          {/if}
        </section>
      {/if}
      <p class="note">{t("history.note")}</p>
    {:else if history.tab === "comments"}
      <p class="empty" data-testid="comments-empty">{t("history.comments.empty")}</p>
    {:else}
      <ul class="feed" aria-label={t("history.tab.activity")}>
        {#each activity as a (a.key)}
          <li><span>{a.text}</span><small>{stamp(a.when, false, "")}</small></li>
        {:else}
          <li class="empty">{t("history.activity.empty")}</li>
        {/each}
      </ul>
    {/if}
  </aside>
{/if}

{#if history.diff}<DiffView req={history.diff} />{/if}
{#if history.apply}<ApplyDialog req={history.apply} />{/if}

<style>
  .drawer {
    box-sizing: border-box;
    display: flex;
    flex: none;
    flex-direction: column;
    gap: 8px;
    width: min(380px, 42vw);
    min-height: 0;
    padding: 10px 12px;
    overflow: hidden;
    background: var(--sidebar-bg);
    box-shadow: -1px 0 var(--shade);
  }
  .tabs {
    align-self: center;
  }
  .filters {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 6px;
  }
  select {
    box-sizing: border-box;
    width: 100%;
    min-height: 32px;
    border: 0;
    border-radius: var(--radius-button);
    background: var(--btn-bg);
    color: var(--fg);
    font: inherit;
    padding: 0 8px;
  }
  .toggles {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px 12px;
    font-size: 13px;
  }
  .sw {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .count {
    margin-left: auto;
    color: var(--fg-dim);
  }
  .legend {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 10px;
    margin: 0;
    font-size: 12.5px;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 5px;
  }
  .dot {
    width: 9px;
    height: 9px;
    border-radius: 50%;
  }
  .busy {
    color: var(--fg-dim);
  }
  .list {
    flex: 1 1 auto;
    min-height: 0;
    overflow: auto;
  }
  .list:focus-visible {
    outline: var(--focus-ring);
    border-radius: var(--radius-button);
  }
  .actions {
    flex: none;
    padding: 10px;
  }
  .btns {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  form {
    display: flex;
    flex-wrap: wrap;
    align-items: end;
    gap: 6px;
  }
  form label {
    display: flex;
    flex: 1 1 100%;
    flex-direction: column;
    gap: 4px;
    font-size: 13px;
  }
  form input {
    min-height: 34px;
    border: 0;
    border-radius: var(--radius-button);
    padding: 0 10px;
    background: var(--btn-bg);
    color: var(--fg);
    font: inherit;
  }
  .note,
  .empty {
    flex: none;
    margin: 0;
    color: var(--fg-dim);
    font-size: 12.5px;
    line-height: 1.5;
  }
  .feed {
    margin: 0;
    padding: 0;
    overflow: auto;
    list-style: none;
  }
  .feed li {
    display: flex;
    flex-direction: column;
    padding: 8px 4px;
    box-shadow: inset 0 -1px var(--card-shade);
  }
  .feed small {
    color: var(--fg-dim);
  }
  .vh {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
  .tabs button[aria-selected="true"] {
    background: var(--popover-bg);
    box-shadow: 0 1px 2px rgb(0 0 6 / 25%);
  }
</style>
