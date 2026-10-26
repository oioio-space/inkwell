<script lang="ts">
import { getChunks, MergeView } from "@codemirror/merge";
import { EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { tick } from "svelte";
import { cspNonce } from "../../editor/setup";
import { cellDiff } from "../../history/csvdiff";
import { compareTexts, entryDiff, type Frag } from "../../history/diffview";
import { moveSections, stamp } from "../../history/stamp";
import { summarize } from "../../history/summary";
import { sessionAt } from "../../history/track";
import { t, tn } from "../../i18n";
import { workspace } from "../../workspace/workspace.svelte";
import Icon from "../icons/Icon.svelte";
import DiffInline from "./DiffInline.svelte";
import { type DiffRequest, history } from "./history.svelte";

/**
 * What changed between two moments, for a person who has never seen a diff: a sentence first
 * (« Marie a ajouté 2 phrases… »), then the text with the changes at the word, who made each
 * run and when. Inline or side by side; F7 and Shift+F7 go from change to change.
 */
let { req, file = workspace.current }: { req: DiffRequest; file?: string } = $props();

let dialog: HTMLDialogElement | undefined = $state();
let mode = $state<"inline" | "side">("inline");
let current = $state(0);
let sideHost: HTMLDivElement | undefined = $state();
let body: HTMLDivElement | undefined = $state();
let sideCount = $state(0);
let merge: MergeView | undefined;

const isTable = $derived(/\.(csv|tsv)$/i.test(file));
const blame = $derived(req.attributed && history.isCurrent() ? history.analysis?.blame : undefined);
const diff = $derived(
  req.session
    ? entryDiff(req.before, req.after, req.session)
    : compareTexts(req.before, req.after, blame),
);
const who = $derived(req.session ? history.authorOf(req.session) : undefined);
const summary = $derived(summarize(req.before, req.after, diff.ops, who?.name));
const moves = $derived(moveSections(req.before, req.after, diff.ops));
const cells = $derived(isTable ? cellDiff(req.before, req.after) : []);
const total = $derived(mode === "inline" ? diff.hunks.length : sideCount);

$effect(() => {
  dialog?.showModal();
  return () => dialog?.close();
});

function whenOf(f: Frag): string | undefined {
  if (req.session) return stamp(req.session.end, req.session.skewed, who?.name ?? "");
  const s = f.s === undefined ? undefined : blame?.sessions[f.s];
  return s ? stamp(s.end, s.skewed, history.author(s.actor).name) : undefined;
}

/** Who made run `i` and when, for the line under the toolbar. */
const info = $derived.by(() => {
  if (total === 0) return t("diff.none");
  const head = t("diff.step", { n: current + 1, total });
  if (req.session && who)
    return `${head} · ${who.name} · ${stamp(req.session.end, req.session.skewed, who.name)}`;
  if (mode === "inline") {
    const h = diff.hunks[current];
    const first = h ? diff.frags.slice(h.from, h.to + 1).find((f) => f.actor) : undefined;
    if (!first?.actor) return head;
    return `${head} · ${history.author(first.actor).name}${whenOf(first) ? ` · ${whenOf(first)}` : ""}`;
  }
  if (!merge || !blame) return head;
  const chunk = getChunks(merge.b.state)?.chunks[current];
  const s = chunk ? sessionAt(blame, chunk.fromB) : -1;
  const session = s >= 0 ? blame.sessions[s] : undefined;
  return session
    ? `${head} · ${history.author(session.actor).name} · ${stamp(session.end, session.skewed, history.author(session.actor).name)}`
    : head;
});

// Side by side: the two texts in CodeMirror's merge view, read only, with the zoom of the page.
const readOnly = [
  EditorView.editable.of(false),
  EditorState.readOnly.of(true),
  EditorView.lineWrapping,
  cspNonce(),
  EditorView.theme({
    "&": {
      fontSize: "calc(14px * var(--text-zoom, 1))",
      backgroundColor: "var(--view-bg)",
      color: "var(--fg)",
    },
    ".cm-scroller": { fontFamily: "var(--font-mono)", lineHeight: "1.7" },
    ".cm-gutters": { backgroundColor: "var(--view-bg)", border: "none", color: "var(--fg-dim)" },
    "&.cm-merge-a .cm-changedLine, .cm-deletedChunk": { backgroundColor: "var(--diff-del-bg)" },
    "&.cm-merge-b .cm-changedLine, .cm-inlineChangedLine": {
      backgroundColor: "var(--diff-add-bg)",
    },
    "&.cm-merge-a .cm-changedText, .cm-deletedChunk .cm-deletedText": {
      background: "none",
      textDecoration: "line-through",
      backgroundColor: "color-mix(in srgb, var(--destructive) 28%, transparent)",
    },
    "&.cm-merge-b .cm-changedText": {
      background: "none",
      backgroundColor: "color-mix(in srgb, var(--success) 32%, transparent)",
    },
    "&.cm-focused": { outline: "none" },
  }),
];

$effect(() => {
  if (mode !== "side" || !sideHost) return;
  const view = new MergeView({
    a: { doc: req.before, extensions: readOnly },
    b: { doc: req.after, extensions: readOnly },
    parent: sideHost,
    highlightChanges: true,
    gutter: true,
    collapseUnchanged: { margin: 3, minSize: 4 },
  });
  merge = view;
  sideCount = getChunks(view.b.state)?.chunks.length ?? 0;
  current = 0;
  return () => {
    view.destroy();
    merge = undefined;
  };
});

async function go(step: 1 | -1) {
  if (total === 0) return;
  current = (current + step + total) % total;
  await tick();
  if (mode === "inline") {
    body
      ?.querySelector(`[data-hunk="${current}"]`)
      ?.scrollIntoView({ block: "center", behavior: "smooth" });
  } else if (merge) {
    const chunk = getChunks(merge.b.state)?.chunks[current];
    if (chunk) {
      merge.b.dispatch({
        selection: { anchor: chunk.fromB },
        effects: EditorView.scrollIntoView(chunk.fromB, { y: "center" }),
      });
    }
  }
}

function onKey(e: KeyboardEvent) {
  if (e.key === "F7") {
    e.preventDefault();
    void go(e.shiftKey ? -1 : 1);
  }
}

function close() {
  history.diff = undefined;
}

const csvLabel = (v: string | undefined) =>
  v === undefined || v === "" ? t("diff.cell.empty") : v;
</script>

<svelte:window onkeydown={onKey} />

<dialog bind:this={dialog} onclose={close} aria-labelledby="diff-title" data-testid="diff-view">
  <header>
    <div class="head">
      <h2 id="diff-title">{req.title}</h2>
      <p class="summary" data-testid="diff-summary">{summary.text}</p>
      {#if who && req.session}
        <p class="by">
          <span class="chip" style:--who="var(--person-{who.index})"><span class="dot" aria-hidden="true"></span>{who.name}</span>
          <span class="when" title={t("history.device.hint", { name: who.name })}>{stamp(req.session.end, req.session.skewed, who.name)}</span>
        </p>
      {/if}
    </div>
    <button type="button" class="fb close" aria-label={t("diff.close")} onclick={close}><Icon name="close" size={14} /></button>
  </header>

  <div class="bar">
    {#if !isTable}
      <div class="tg" role="radiogroup" aria-label={t("diff.mode")}>
        <button type="button" role="radio" aria-checked={mode === "inline"} onclick={() => { mode = "inline"; current = 0; }}>{t("diff.mode.inline")}</button>
        <button type="button" role="radio" aria-checked={mode === "side"} onclick={() => { mode = "side"; }}>{t("diff.mode.side")}</button>
      </div>
      <span class="sp"></span>
      <button type="button" class="fb lbl" onclick={() => go(-1)} disabled={total === 0} title={t("diff.prev.hint")}>
        {t("diff.prev")}
      </button>
      <button type="button" class="fb lbl" onclick={() => go(1)} disabled={total === 0} title={t("diff.next.hint")}>
        {t("diff.next")}
      </button>
    {/if}
  </div>
  <p class="info" role="status" aria-live="polite" data-testid="diff-info">{isTable ? tn("diff.cells", cells.length) : info}</p>

  <div class="body" bind:this={body}>
    {#if isTable}
      <ul class="cells" data-testid="diff-cells">
        {#each cells as c}
          <li><b>{c.ref}</b> : <del>{csvLabel(c.before)}</del> → <ins>{csvLabel(c.after)}</ins></li>
        {/each}
      </ul>
    {:else if mode === "inline"}
      <DiffInline
        frags={diff.frags}
        hunks={diff.hunks}
        {current}
        author={history.author}
        when={whenOf}
        {moves}
        showAuthors={!req.session}
        label={t("diff.text")}
      />
    {:else}
      <div class="side" bind:this={sideHost} data-zoomable></div>
    {/if}
  </div>

  {#if !history.readOnly && (req.revert || req.undo)}
    <footer>
      {#if req.undo}
        <button type="button" class="btn" onclick={() => history.askUndo((req.undo as { session: NonNullable<DiffRequest["session"]> }).session)}>
          {t("history.undo.button")}
        </button>
      {/if}
      {#if req.revert}
        <button type="button" class="sug" onclick={() => history.askRevert((req.revert as { heads: string[]; label: string }).heads, (req.revert as { label: string }).label)}>
          {t("history.revert.button")}
        </button>
      {/if}
    </footer>
  {/if}
</dialog>

<style>
  dialog {
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    width: min(1100px, calc(100vw - 32px));
    height: min(760px, calc(100vh - 32px));
    max-width: none;
    max-height: none;
    padding: 0;
    border: 0;
    border-radius: var(--radius);
    background: var(--window-bg);
    color: var(--fg);
    box-shadow: 0 0 0 1px var(--border), 0 12px 32px rgb(0 0 0 / 25%);
  }
  dialog:not([open]) {
    display: none;
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 30%);
  }
  header {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 16px 16px 8px 20px;
  }
  .head {
    flex: 1;
    min-width: 0;
  }
  h2 {
    margin: 0;
    font: 700 15px/1.4 var(--font-ui);
  }
  .summary {
    margin: 4px 0 0;
    font: 400 15px/1.5 var(--font-ui);
  }
  .by {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 6px 0 0;
    color: var(--fg-dim);
    font: 13px/1.4 var(--font-ui);
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 1px 10px 1px 8px;
    border-radius: 99px;
    background: color-mix(in srgb, var(--who) 22%, var(--window-bg));
    color: var(--fg);
    font-weight: 700;
  }
  .dot {
    width: 9px;
    height: 9px;
    border-radius: 50%;
    background: var(--who);
  }
  .close {
    flex: none;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 16px;
    min-height: 40px;
  }
  .sp {
    flex: 1;
  }
  .info {
    margin: 0;
    padding: 2px 20px 8px;
    color: var(--fg-dim);
    font: 13px/1.4 var(--font-ui);
  }
  .body {
    flex: 1;
    min-height: 0;
    overflow: auto;
    margin: 0 16px;
    border-radius: var(--radius-small);
    background: var(--view-bg);
    box-shadow: 0 0 0 1px var(--border);
  }
  .side {
    min-height: 100%;
  }
  .side :global(.cm-mergeView) {
    min-height: 100%;
  }
  .cells {
    margin: 0;
    padding: 16px 28px;
    list-style: none;
    font: calc(15px * var(--text-zoom, 1)) / 1.9 var(--font-mono);
  }
  .cells del {
    background: var(--diff-del-bg);
    color: var(--diff-del-fg);
  }
  .cells ins {
    background: var(--diff-add-bg);
    color: var(--diff-add-fg);
    text-decoration: none;
  }
  footer {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding: 12px 16px;
  }
</style>
