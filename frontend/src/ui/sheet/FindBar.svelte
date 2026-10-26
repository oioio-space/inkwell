<script lang="ts">
import { tick } from "svelte";
import { refOf } from "../../editor/csv/address";
import type { CellHit } from "../../editor/csv/cellSearch";
import type { QuerySpec, SearchMode } from "../../editor/search/query";
import { t, tn } from "../../i18n";
import Icon from "../icons/Icon.svelte";
import type { SheetController } from "./sheet.svelte";

/**
 * Find and replace in the cells (the B6 engine, applied to cell values): a match is never the
 * file's separator or quote, and never spans two cells. Replacing writes the cell again with
 * the quoting it needs, as one change.
 */
let {
  ctrl,
  withReplace = false,
  onclose,
}: { ctrl: SheetController; withReplace?: boolean; onclose: () => void } = $props();

let text = $state("");
let replacement = $state("");
let mode = $state<SearchMode>("normal");
let scope = $state<"sheet" | "column" | "selection" | "headers">("sheet");
let caseSensitive = $state(false);
let ignoreAccents = $state(false);
let entireCell = $state(false);
let showReplace = $state(false);
let at = $state(0);
let input: HTMLInputElement | undefined = $state();

$effect(() => {
  showReplace = withReplace;
});

const spec = $derived<QuerySpec>({
  text,
  mode,
  caseSensitive,
  wholeWord: false,
  ignoreAccents,
  dotAll: false,
});

const found = $derived.by(() => {
  ctrl.tick;
  return ctrl.search(spec, scope, entireCell);
});
const hits = $derived<CellHit[]>(found.hits);
const occurrences = $derived(hits.reduce((n, h) => n + h.matches.length, 0));
const here = $derived(hits[Math.min(at, hits.length - 1)]);

$effect(() => {
  void text;
  at = 0;
});

$effect(() => {
  void tick().then(() => input?.focus());
});

function go(i: number) {
  if (hits.length === 0) return;
  at = ((i % hits.length) + hits.length) % hits.length;
  const h = hits[at];
  if (h) ctrl.goTo(h);
}

function replaceOne() {
  const h = here;
  if (!h) return;
  ctrl.replaceHits([h], replacement, { regex: mode === "regex", extended: mode === "extended" });
}

function replaceEvery() {
  const n = ctrl.replaceHits(hits, replacement, {
    regex: mode === "regex",
    extended: mode === "extended",
  });
  if (n > 0) ctrl.notice = { text: t("sheet.find.replaced", { count: n }) };
}

function key(e: KeyboardEvent) {
  if (e.key === "Escape") {
    e.preventDefault();
    onclose();
  } else if (e.key === "Enter" || e.key === "F3") {
    e.preventDefault();
    go(at + (e.shiftKey ? -1 : 1));
  }
}

const MODES: {
  id: SearchMode;
  key:
    | "sheet.find.mode.normal"
    | "sheet.find.mode.extended"
    | "sheet.find.mode.wildcard"
    | "sheet.find.mode.regex";
}[] = [
  { id: "normal", key: "sheet.find.mode.normal" },
  { id: "extended", key: "sheet.find.mode.extended" },
  { id: "wildcard", key: "sheet.find.mode.wildcard" },
  { id: "regex", key: "sheet.find.mode.regex" },
];
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div class="find" role="search" aria-label={t("sheet.find.label")} onkeydown={key}>
  <div class="line">
    <input
      bind:this={input}
      class="field"
      type="search"
      bind:value={text}
      placeholder={t("sheet.find.placeholder")}
      aria-label={t("sheet.find.placeholder")}
    />
    <span class="count" aria-live="polite" data-testid="find-count">
      {#if found.error}{found.error}{:else if text === ""}{:else if hits.length === 0}{t("sheet.find.none")}{:else}{t("sheet.find.count", { at: at + 1, total: hits.length })}{/if}
    </span>
    <button type="button" class="fb" aria-label={t("sheet.find.prev")} title={`${t("sheet.find.prev")} (Maj+F3)`} onclick={() => go(at - 1)}>
      <span class="up"><Icon name="caretdown" /></span>
    </button>
    <button type="button" class="fb" aria-label={t("sheet.find.next")} title={`${t("sheet.find.next")} (F3)`} onclick={() => go(at + 1)}>
      <Icon name="caretdown" />
    </button>
    <button type="button" class="fb" aria-pressed={showReplace} aria-label={t("sheet.find.toggleReplace")} onclick={() => (showReplace = !showReplace)}>
      <Icon name="right" />
    </button>
    <button type="button" class="fb" aria-label={t("sheet.find.close")} onclick={onclose}>
      <Icon name="close" size={12} />
    </button>
  </div>
  <div class="line opts">
    <select bind:value={scope} aria-label={t("sheet.find.scope")}>
      <option value="sheet">{t("sheet.find.scope.sheet")}</option>
      <option value="column">{t("sheet.find.scope.column")}</option>
      <option value="selection">{t("sheet.find.scope.selection")}</option>
      <option value="headers">{t("sheet.find.scope.headers")}</option>
    </select>
    <select bind:value={mode} aria-label={t("sheet.find.mode")}>
      {#each MODES as m (m.id)}<option value={m.id}>{t(m.key)}</option>{/each}
    </select>
    <label><input type="checkbox" bind:checked={caseSensitive} />{t("sheet.find.case")}</label>
    <label><input type="checkbox" bind:checked={ignoreAccents} />{t("sheet.find.accents")}</label>
    <label><input type="checkbox" bind:checked={entireCell} />{t("sheet.find.entire")}</label>
  </div>
  {#if showReplace && !ctrl.readOnly}
    <div class="line">
      <input class="field" type="text" bind:value={replacement} placeholder={t("sheet.find.replaceWith")} aria-label={t("sheet.find.replaceWith")} />
      <button type="button" class="btn" disabled={!here} onclick={replaceOne}>{t("sheet.find.replace")}</button>
      <button type="button" class="btn" disabled={hits.length === 0} onclick={replaceEvery}>
        {t("sheet.find.replaceAll", { count: occurrences })}
      </button>
    </div>
  {/if}
  {#if hits.length > 0}
    <ul class="results" aria-label={t("sheet.find.results")}>
      {#each hits.slice(0, 200) as h, i (`${h.r}:${h.c}`)}
        <li>
          <button type="button" class="mi" class:cur={i === at} onclick={() => go(i)}>
            <span class="ref">{refOf(h.r - ctrl.model.preamble, h.c)}</span>
            <span class="val">{h.value}</span>
          </button>
        </li>
      {/each}
    </ul>
    <p class="more">{tn("sheet.find.occurrences", occurrences)}</p>
  {/if}
</div>

<style>
  .find {
    position: absolute;
    top: 4px;
    right: 18px;
    z-index: 30;
    width: min(460px, calc(100% - 36px));
    padding: 8px;
    border-radius: var(--radius);
    background: var(--popover-bg);
    box-shadow: var(--shadow-popover);
  }
  .line {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-bottom: 6px;
  }
  .opts {
    flex-wrap: wrap;
    font-size: 12.5px;
  }
  .opts label {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .field,
  select {
    box-sizing: border-box;
    min-height: 32px;
    border: 0;
    border-radius: var(--radius-small);
    padding: 0 8px;
    background: var(--btn-bg);
    color: var(--fg);
    font: inherit;
  }
  .field {
    flex: 1;
    min-width: 0;
  }
  .count {
    min-width: 54px;
    color: var(--fg-dim);
    font-size: 12.5px;
    text-align: right;
  }
  .up {
    display: inline-flex;
    transform: rotate(180deg);
  }
  .results {
    max-height: 190px;
    margin: 0;
    padding: 0;
    overflow: auto;
    list-style: none;
  }
  .mi {
    min-height: 30px;
  }
  .mi.cur {
    background: var(--selected-bg);
  }
  .ref {
    flex: none;
    min-width: 54px;
    color: var(--accent);
    font-weight: 600;
  }
  .val {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .more {
    margin: 4px 0 0;
    color: var(--fg-dim);
    font-size: 12px;
  }
</style>
