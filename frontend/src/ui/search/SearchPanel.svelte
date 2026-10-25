<script lang="ts">
import type { next as A } from "@automerge/automerge";
import { SearchTooCostly } from "../../editor/search/find";
import {
  findInProject,
  type Hit,
  type ProjectFileInput,
  type ProjectHits,
  type ResolvedHit,
  resolveHit,
} from "../../editor/search/project";
import { compileQuery, type Matcher, type QuerySpec } from "../../editor/search/query";
import {
  type PreviewLine,
  previewReplace,
  type ReplaceOptions,
  type ReplaceTarget,
  replaceAll,
  replaceInProject,
} from "../../editor/search/replace";
import type { SearchRunner } from "../../editor/search/runner";
import { t } from "../../i18n";
import type { FileKind } from "../../store/files";
import {
  clearHistory,
  type HistoryEntry,
  loadHistory,
  pushHistory,
  SCOPES,
  type Scope,
  type UiMode,
} from "./history";
import ResultsTree from "./ResultsTree.svelte";
import { countLabel, linesOf, liveHits, type SearchRun, stepIndex } from "./results";
import SearchField from "./SearchField.svelte";

type Doc = A.Doc<{ text: string }>;

let {
  projectKey,
  files,
  current,
  anchor,
  resolve,
  handleOf,
  isLocked,
  readOnly = false,
  version = 0,
  runner,
  onnavigate,
  onquery,
  onclose,
}: {
  /** Names the project for the saved history. */
  projectKey: string;
  /** The project's files. Binary ones are never read. */
  files: () => ProjectFileInput[];
  /** The open file and its selection, for "Ce fichier" / "Sélection". */
  current?: () =>
    | { path: string; kind: FileKind; text: string; selection?: { from: number; to: number } }
    | undefined;
  /** The open document of a file: hits get cursors and follow other people's edits. */
  anchor?: (path: string) => Doc | undefined;
  /** Overrides where a hit is now (default: through `anchor`). */
  resolve?: (path: string, hit: Hit) => ResolvedHit | undefined;
  /** What a replacement writes into. Without it the panel cannot replace. */
  handleOf?: (path: string) => ReplaceTarget | undefined;
  /** True for a range in a locked zone (ADR 0022); those occurrences are never replaced. */
  isLocked?: (path: string, range: { from: number; to: number }) => boolean;
  /** A reader can search but not replace. */
  readOnly?: boolean;
  /** Bump when documents change, so results re-check their words. */
  version?: number;
  runner?: SearchRunner;
  /** Jump to an occurrence: open the file, select the range, centre it. */
  onnavigate: (target: { path: string; from: number; to: number }) => void;
  /** The query as it is typed, for highlighting it in the editor; null when empty or invalid. */
  onquery?: (matcher: Matcher | null) => void;
  onclose?: () => void;
} = $props();

let field: SearchField;
let query = $state("");
let mode = $state<UiMode>("normal");
let extended = $state(false);
let caseSensitive = $state(false);
let wholeWord = $state(false);
let ignoreAccents = $state(false);
let dotAll = $state(false);
let visibleText = $state(false);
let scope = $state<Scope>("project");
let filters = $state("");
let replacement = $state("");
let showReplace = $state(false);

let runs = $state.raw<SearchRun[]>([]);
let active = $state({ run: 0, key: "" });
let busy = $state(false);
let failure = $state("");
let notice = $state("");
// svelte-ignore state_referenced_locally
let history = $state.raw<HistoryEntry[]>(loadHistory(projectKey));
let historyOpen = $state(false);
let preview = $state.raw<{
  files: Array<{ path: string; lines: PreviewLine[] }>;
  total: number;
  locked: number;
} | null>(null);
let nextId = 1;

const canReplace = $derived(!readOnly && handleOf !== undefined);

const spec = $derived<QuerySpec>({
  text: query,
  mode: mode === "normal" && extended ? "extended" : mode,
  caseSensitive,
  wholeWord: wholeWord && mode !== "regex",
  ignoreAccents,
  dotAll,
  visibleText,
});
const compiled = $derived(compileQuery(spec));
const queryError = $derived(compiled.ok ? "" : compiled.error);
/** Nothing to run: busy, empty or invalid query. */
const cannotSearch = $derived(busy || query === "" || queryError !== "");
const signature = $derived(JSON.stringify([spec, scope, filters]));

$effect(() => {
  onquery?.(compiled.ok && !compiled.matcher.empty ? compiled.matcher : null);
});

const replaceOptions = (): ReplaceOptions => ({
  regex: mode === "regex",
  extended: mode === "normal" && extended,
});

/** The replace options for one file: its occurrences in a locked zone are left alone. */
function fileReplaceOptions(path: string): ReplaceOptions {
  if (!isLocked) return replaceOptions();
  return { ...replaceOptions(), skip: (range) => isLocked(path, range) };
}

/** Where a hit is now, re-checked whenever `version` changes. */
function locate(path: string, hit: Hit): ResolvedHit {
  void version;
  const given = resolve?.(path, hit);
  if (given) return given;
  const doc = anchor?.(path);
  return doc ? resolveHit(doc, hit) : { from: hit.from, to: hit.to, stale: false };
}

const top = $derived(runs[0]);
const topCurrent = $derived(top !== undefined && top.signature === signature);

function apply(e: HistoryEntry) {
  query = e.query;
  mode = e.mode;
  extended = e.extended;
  caseSensitive = e.caseSensitive;
  wholeWord = e.wholeWord;
  ignoreAccents = e.ignoreAccents;
  dotAll = e.dotAll;
  visibleText = e.visibleText;
  filters = e.filters;
  scope = e.scope;
  historyOpen = false;
  field?.focus(false);
}

function entryNow(): HistoryEntry {
  return {
    query,
    mode,
    extended,
    caseSensitive,
    wholeWord,
    ignoreAccents,
    dotAll,
    visibleText,
    filters,
    scope,
  };
}

function describe(e: HistoryEntry): string {
  const bits = [t(`search.mode.${e.mode}`)];
  if (e.caseSensitive) bits.push(t("search.opt.case"));
  if (e.wholeWord && e.mode !== "regex") bits.push(t("search.opt.word"));
  if (e.ignoreAccents) bits.push(t("search.opt.accents"));
  if (e.scope !== "project") bits.push(t(`search.scope.${e.scope}`));
  return bits.join(" · ");
}

function restriction(within: SearchRun | undefined) {
  if (!within) return undefined;
  return new Map(
    within.results.files.map((f) => [f.path, new Set(f.hits.map((h) => h.line))] as const),
  );
}

/** "Tout trouver": runs the search and stacks its results on top. */
async function runSearch(within?: SearchRun, keepNotice = false): Promise<SearchRun | undefined> {
  if (!compiled.ok || compiled.matcher.empty) return undefined;
  const matcher = compiled.matcher;
  busy = true;
  failure = "";
  if (!keepNotice) notice = "";
  try {
    const common = { anchor, runner, restrictTo: restriction(within) };
    let results: ProjectHits;
    if (scope === "project") {
      results = await findInProject(files(), matcher, filters, common);
    } else {
      const cur = current?.();
      results = cur
        ? await findInProject(
            [{ path: cur.path, kind: cur.kind, text: () => cur.text }],
            matcher,
            "",
            {
              ...common,
              range: scope === "selection" && cur.selection ? cur.selection : {},
            },
          )
        : { files: [], total: 0, scanned: 0 };
    }
    const run: SearchRun = {
      id: nextId++,
      label: within ? `${query} ‹ ${within.label}` : query,
      signature,
      results,
      open: true,
      collapsed: new Set(),
    };
    runs = [run, ...runs];
    active = { run: run.id, key: "" };
    history = pushHistory(projectKey, entryNow());
    return run;
  } catch (e) {
    failure = e instanceof SearchTooCostly ? t("search.error.costly") : t("search.error.failed");
    return undefined;
  } finally {
    busy = false;
  }
}

function go(run: SearchRun, path: string, index: number) {
  const file = run.results.files.find((f) => f.path === path);
  const hit = file?.hits[index];
  if (!hit) return;
  const at = locate(path, hit);
  if (at.stale) return;
  active = { run: run.id, key: `${path}:${index}` };
  onnavigate({ path, from: at.from, to: at.to });
}

/** Next / previous live occurrence of the results list (Enter, F3, F4). */
export async function step(backwards = false): Promise<void> {
  let run = topCurrent ? top : await runSearch();
  if (!run) return;
  const live = liveHits(run.results.files, locate);
  const at = live.findIndex((l) => `${l.path}:${l.index}` === active.key && active.run === run?.id);
  const target = live[stepIndex(live.length, at, backwards)];
  if (target) go(run, target.path, target.index);
}

export const stepResult = step;

/** Opens the panel the way Ctrl+F / Ctrl+H / Ctrl+Maj+F do. */
export function open(kind: "file" | "replace" | "project"): void {
  if (kind === "project") scope = "project";
  else if (scope === "project") scope = "file";
  if (kind === "replace" && canReplace) showReplace = true;
  field?.focus();
}

export function focus(): void {
  field?.focus();
}

function toggleFile(run: SearchRun, path: string) {
  const collapsed = new Set(run.collapsed);
  if (!collapsed.delete(path)) collapsed.add(path);
  runs = runs.map((r) => (r.id === run.id ? { ...r, collapsed } : r));
}

function toggleRun(run: SearchRun) {
  runs = runs.map((r) => (r.id === run.id ? { ...r, open: !r.open } : r));
}

async function textOf(path: string): Promise<string> {
  const f = files().find((x) => x.path === path);
  if (f) return f.text();
  const cur = current?.();
  return cur?.path === path ? cur.text : "";
}

async function copyLines(run: SearchRun) {
  const out: string[] = [];
  for (const f of run.results.files) out.push(...linesOf(await textOf(f.path), f.hits));
  try {
    await navigator.clipboard.writeText(out.join("\n"));
    notice = t("search.copied", { count: out.length });
  } catch {
    notice = t("search.error.copy");
  }
}

function summary(replaced: number, locked: number, stale: number): string {
  const parts = [t("search.replaced", { count: replaced })];
  if (locked > 0) parts.push(t("search.locked", { count: locked }));
  if (stale > 0) parts.push(t("search.staleSkipped", { count: stale }));
  return parts.join(" · ");
}

/** Replaces the current occurrence, then moves on to the next. */
async function replaceOne() {
  if (!canReplace || !compiled.ok) return;
  const run = topCurrent ? top : await runSearch();
  if (!run) return;
  const live = liveHits(run.results.files, locate);
  const at = Math.max(
    0,
    live.findIndex((l) => `${l.path}:${l.index}` === active.key),
  );
  const target = live[at];
  const handle = target ? handleOf?.(target.path) : undefined;
  if (!target || !handle) return;
  const r = replaceAll(handle, [target.hit], replacement, fileReplaceOptions(target.path));
  notice = summary(r.replaced, r.skippedLocked, r.skippedStale);
  runs = runs.slice(1);
  const fresh = await runSearch(undefined, true);
  if (!fresh) return;
  const left = liveHits(fresh.results.files, locate);
  const next = left[Math.min(at, left.length - 1)];
  if (next) go(fresh, next.path, next.index);
}

async function replaceEverything() {
  if (!canReplace || !compiled.ok) return;
  const run = topCurrent ? top : await runSearch();
  if (!run || run.results.total === 0) return;
  if (scope !== "project") {
    commit(run);
    return;
  }
  // In the project, show before / after first.
  const out: Array<{ path: string; lines: PreviewLine[] }> = [];
  let locked = 0;
  let total = 0;
  for (const f of run.results.files) {
    const lines = previewReplace(
      await textOf(f.path),
      f.hits,
      replacement,
      fileReplaceOptions(f.path),
    );
    for (const l of lines) {
      locked += l.locked;
      total += l.replaced;
    }
    out.push({ path: f.path, lines });
  }
  preview = { files: out, total, locked };
}

function commit(run: SearchRun) {
  if (!handleOf) return;
  const r = replaceInProject(run.results.files, handleOf, replacement, {
    ...replaceOptions(),
    ...(isLocked ? { skipIn: isLocked } : {}),
  });
  preview = null;
  notice = summary(r.replaced, r.skippedLocked, r.skippedStale);
  runs = runs.filter((x) => x.id !== run.id);
  void runSearch(undefined, true);
}

function keydown(e: KeyboardEvent) {
  if (e.key === "Escape") {
    if (preview) preview = null;
    else if (historyOpen) historyOpen = false;
    else onclose?.();
    e.preventDefault();
  } else if (e.key === "F3" || e.key === "F4") {
    e.preventDefault();
    void step(e.shiftKey);
  }
}
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<section class="panel" aria-label={t("search.title")} onkeydown={keydown}>
  <div class="head">
    <div class="grow">
      <SearchField
        bind:this={field}
        bind:value={query}
        bind:mode
        bind:extended
        bind:caseSensitive
        bind:wholeWord
        bind:ignoreAccents
        bind:dotAll
        bind:visibleText
        error={queryError}
        onsubmit={(back) => void step(back)}
        onhistory={() => (historyOpen = true)}
      />
    </div>
    {#if onclose}
      <button type="button" class="icon" aria-label={t("search.close")} title={t("search.close")} onclick={() => onclose?.()}>×</button>
    {/if}
  </div>

  {#if canReplace && showReplace}
    <div class="row">
      <input
        class="text"
        type="text"
        aria-label={t("search.replace.label")}
        placeholder={t("search.replace.placeholder")}
        bind:value={replacement}
        autocomplete="off"
        spellcheck="false"
      />
      <button type="button" class="btn" disabled={cannotSearch} onclick={() => void replaceOne()}>{t("search.replace")}</button>
      <button type="button" class="btn" disabled={cannotSearch} onclick={() => void replaceEverything()}>{t("search.replaceAll")}</button>
    </div>
  {/if}

  <div class="row">
    <div class="seg" role="group" aria-label={t("search.scope.group")}>
      {#each SCOPES as s (s)}
        <button type="button" class:on={scope === s} aria-pressed={scope === s} onclick={() => (scope = s)}>{t(`search.scope.${s}`)}</button>
      {/each}
    </div>
    {#if scope === "project"}
      <input
        class="text filters"
        type="text"
        aria-label={t("search.filters.label")}
        placeholder={t("search.filters.placeholder")}
        bind:value={filters}
        autocomplete="off"
        spellcheck="false"
      />
    {/if}
    <span class="grow"></span>
    {#if canReplace && !showReplace}
      <button type="button" class="btn flat" onclick={() => (showReplace = true)}>{t("search.showReplace")}</button>
    {/if}
    <button type="button" class="btn flat" aria-expanded={historyOpen} onclick={() => (historyOpen = !historyOpen)}>{t("search.history")}</button>
    <button type="button" class="btn suggested" disabled={cannotSearch} onclick={() => void runSearch()}>{t("search.findAll")}</button>
  </div>

  {#if historyOpen}
    <div class="history" role="listbox" aria-label={t("search.history")}>
      {#each history as e, i (i)}
        <button type="button" role="option" aria-selected="false" class="entry" onclick={() => apply(e)}>
          <span class="q">{e.query}</span><span class="d">{describe(e)}</span>
        </button>
      {:else}
        <p class="empty">{t("search.history.empty")}</p>
      {/each}
      {#if history.length > 0}
        <button type="button" class="btn flat" onclick={() => { clearHistory(projectKey); history = []; }}>{t("search.history.clear")}</button>
      {/if}
    </div>
  {/if}

  {#if preview}
    <div class="preview" role="dialog" aria-label={t("search.preview.title")}>
      <h3>{t("search.preview.title")}</h3>
      <div class="lines">
        {#each preview.files as f (f.path)}
          <h4>{f.path}</h4>
          {#each f.lines as l (l.line)}
            <div class="pl"><span class="ln">{l.line}</span><del>{l.before}</del><ins>{l.after}</ins></div>
          {/each}
        {/each}
      </div>
      {#if preview.locked > 0}<p class="note">{t("search.locked", { count: preview.locked })}</p>{/if}
      <div class="row">
        <span class="grow"></span>
        <button type="button" class="btn" onclick={() => (preview = null)}>{t("search.preview.cancel")}</button>
        <button type="button" class="btn suggested" onclick={() => top && commit(top)}>{t("search.preview.confirm", { count: preview.total })}</button>
      </div>
    </div>
  {/if}

  <p class="status" role="status" aria-live="polite">{failure || notice}{busy ? t("search.busy") : ""}</p>

  <div class="results">
    {#each runs as run, i (run.id)}
      <section class="run" aria-label={run.label}>
        <div class="runhead">
          <button type="button" class="title" aria-expanded={run.open} onclick={() => toggleRun(run)}>
            <span class="chev" class:open={run.open} aria-hidden="true">›</span>
            <span class="q">{run.label}</span>
            <span class="count">{countLabel(run.results.total, run.results.files.length)}</span>
          </button>
          {#if i === 0 && run.results.total > 0}
            <button type="button" class="btn flat" onclick={() => void runSearch(run)} disabled={cannotSearch}>{t("search.within")}</button>
            <button type="button" class="btn flat" onclick={() => void copyLines(run)}>{t("search.copy")}</button>
          {/if}
        </div>
        {#if run.open}
          {#if run.results.total === 0}
            <p class="empty">{t("search.count.none")}</p>
          {:else}
            <ResultsTree
              {run}
              {locate}
              label={t("search.results.label")}
              active={active.run === run.id ? active.key : ""}
              ontoggle={(path) => toggleFile(run, path)}
              onactivate={(path, index) => go(run, path, index)}
            />
          {/if}
        {/if}
      </section>
    {/each}
    {#if runs.length > 0}
      <button type="button" class="btn flat clear" onclick={() => { runs = []; active = { run: 0, key: "" }; }}>{t("search.clear")}</button>
    {/if}
  </div>
</section>

<style>
  .panel {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    min-height: 0;
    height: 100%;
    box-sizing: border-box;
    padding: var(--space-3);
    background: var(--window-bg);
    color: var(--fg);
    font: 14px/1.4 var(--font-ui);
    border-top: 1px solid var(--border);
  }
  .head,
  .row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }
  .head {
    align-items: flex-start;
    flex-wrap: nowrap;
  }
  .grow {
    flex: 1 1 auto;
    min-width: 0;
  }
  .icon {
    min-width: 34px;
    min-height: 34px;
    border: 0;
    border-radius: 50%;
    background: color-mix(in srgb, var(--fg) 10%, transparent);
    color: var(--fg);
    font: 18px/1 var(--font-ui);
    cursor: pointer;
  }
  .text {
    flex: 1 1 12em;
    min-height: 34px;
    box-sizing: border-box;
    padding: 0 var(--space-3);
    border: 0;
    border-radius: 9px;
    background: color-mix(in srgb, var(--fg) 8%, transparent);
    color: var(--fg);
    font: 14px/1.3 var(--font-ui);
  }
  .text:focus-visible {
    outline: var(--focus-ring);
    outline-offset: 0;
  }
  .filters {
    flex: 0 1 16em;
    font-family: var(--font-mono);
    font-size: 13px;
  }
  .btn {
    min-height: 34px;
    padding: 0 var(--space-4);
    border: 0;
    border-radius: 9px;
    background: color-mix(in srgb, var(--fg) 10%, transparent);
    color: var(--fg);
    font: 700 14px/1 var(--font-ui);
    cursor: pointer;
  }
  .btn:disabled {
    opacity: 0.45;
    cursor: default;
  }
  .btn.flat {
    background: transparent;
    font-weight: 600;
  }
  .btn.flat:hover:not(:disabled) {
    background: color-mix(in srgb, var(--fg) 7%, transparent);
  }
  .btn.suggested {
    background: var(--accent);
    color: var(--accent-fg);
  }
  .seg {
    display: inline-flex;
    padding: 2px;
    gap: 2px;
    border-radius: 11px;
    background: color-mix(in srgb, var(--fg) 8%, transparent);
  }
  .seg button {
    min-height: 30px;
    padding: 0 var(--space-3);
    border: 0;
    border-radius: 9px;
    background: transparent;
    color: var(--fg);
    font: 600 13.5px/1 var(--font-ui);
    cursor: pointer;
  }
  .seg button.on {
    background: var(--card-bg);
    box-shadow: 0 0 0 1px var(--border), 0 1px 2px rgb(0 0 6 / 12%);
  }
  .history,
  .preview {
    padding: var(--space-2);
    border-radius: var(--radius);
    background: var(--card-bg);
    box-shadow: 0 0 0 1px var(--border);
    max-height: 40vh;
    overflow: auto;
  }
  .entry {
    display: flex;
    gap: var(--space-3);
    align-items: baseline;
    width: 100%;
    min-height: 34px;
    padding: 0 var(--space-3);
    border: 0;
    border-radius: var(--radius-small);
    background: transparent;
    color: var(--fg);
    font: 14px/1.3 var(--font-ui);
    text-align: start;
    cursor: pointer;
  }
  .entry:hover {
    background: color-mix(in srgb, var(--fg) 7%, transparent);
  }
  .entry .q {
    font-family: var(--font-mono);
  }
  .entry .d {
    margin-inline-start: auto;
    color: var(--fg-dim);
    font-size: 12.5px;
  }
  .preview h3 {
    margin: 0 0 var(--space-2);
    font-size: 15px;
  }
  .preview h4 {
    margin: var(--space-2) 0 var(--space-1);
    font-size: 13px;
  }
  .pl {
    display: grid;
    grid-template-columns: 4ch 1fr;
    gap: 0 var(--space-3);
    font: 12.5px/1.5 var(--font-mono);
  }
  .pl .ln {
    grid-row: span 2;
    color: var(--fg-dim);
    text-align: right;
  }
  .pl del,
  .pl ins {
    text-decoration: none;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .pl del {
    background: color-mix(in srgb, #e01b24 18%, transparent);
  }
  .pl ins {
    background: color-mix(in srgb, var(--success) 22%, transparent);
  }
  .note,
  .empty {
    margin: var(--space-1) 0;
    color: var(--fg-dim);
    font-size: 13.5px;
  }
  .status {
    min-height: 1.4em;
    margin: 0;
    color: var(--fg-dim);
    font-size: 13px;
  }
  .results {
    flex: 1 1 auto;
    min-height: 0;
    overflow: auto;
  }
  .run {
    margin-bottom: var(--space-2);
    padding-bottom: var(--space-2);
    border-bottom: 1px solid var(--border);
  }
  .runhead {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    flex-wrap: wrap;
  }
  .title {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex: 1 1 auto;
    min-height: 34px;
    min-width: 0;
    padding: 0 var(--space-2);
    border: 0;
    border-radius: var(--radius-small);
    background: transparent;
    color: var(--fg);
    font: 600 14px/1 var(--font-ui);
    text-align: start;
    cursor: pointer;
  }
  .title .q {
    font-family: var(--font-mono);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .title .count {
    margin-inline-start: auto;
    color: var(--fg-dim);
    font-weight: 400;
    white-space: nowrap;
  }
  .chev {
    display: inline-block;
    width: 1em;
    color: var(--fg-dim);
    transition: transform var(--duration);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  .clear {
    margin-top: var(--space-1);
  }
</style>
