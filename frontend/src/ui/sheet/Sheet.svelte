<script lang="ts">
import type { DocHandle } from "@automerge/automerge-repo";
import { onDestroy, untrack } from "svelte";
import { type Start, startDialect, type TableType } from "../../editor/csv/dialect";
import { openCsv } from "../../editor/csv/open";
import { patchWindow } from "../../editor/csv/patchWindow";
import type { Delimiter } from "../../editor/csv/records";
import { type DocPort, handlePort, plainPort, SheetDoc } from "../../editor/csv/sheetDoc";
import { surface } from "../../extensions/commands";
import type { Viewer } from "../../extensions/registry";
import { t, tn } from "../../i18n";
import { problems } from "../../language/problems.svelte";
import { closeBuffer } from "../../workspace/closePage";
import { type Doc, workspace } from "../../workspace/workspace.svelte";
import LocationMenuAt from "../command/LocationMenuAt.svelte";
import LocationToolbar from "../command/LocationToolbar.svelte";
import { session } from "../editorSession.svelte";
import Icon from "../icons/Icon.svelte";
import Toast from "../Toast.svelte";
import { gridHandlers } from "./commands";
import FilterMenu from "./FilterMenu.svelte";
import FindBar from "./FindBar.svelte";
import Grid from "./Grid.svelte";
import PasteDialog from "./PasteDialog.svelte";
import { sheetPresence } from "./presence";
import { rememberDelimiter, rememberedDelimiter } from "./remember";
import SheetStatus from "./SheetStatus.svelte";
import { sessions } from "./sessions";
import { SheetController, type SheetOptions } from "./sheet.svelte";

/**
 * A CSV, TSV or PSV file as a spreadsheet. The text of the document stays the one source of
 * truth: every edit is a small splice of it, so two people editing different cells merge
 * like text, and "Voir le texte" shows the same file.
 */
let {
  handle,
  big,
  me,
  path,
  table,
  readOnly = false,
  readOnlyReason,
  isLocked,
  startOffset,
  onshowtext,
  viewer,
}: {
  /** The document holding the text; absent for a text too big for one (then `big`). */
  handle?: DocHandle<Doc> | undefined;
  /** A text read from the disk, in no document: the sheet only reads it (ADR 0028). */
  big?: { text: string } | undefined;
  me: { id: string; name: string };
  path: string;
  /** What the file's type says about the table (its delimiter); undefined for none. */
  table: TableType | undefined;
  readOnly?: boolean | undefined;
  /** Shown instead of the reader-role wording when the sheet is read-only for another reason. */
  readOnlyReason?: string | undefined;
  isLocked?: SheetOptions["isLocked"];
  /** Where the text view had its caret, to open on the matching cell. */
  startOffset?: number | undefined;
  onshowtext: (offset: number) => void;
  /** The viewer showing it (`tables.grid`): the commands its surface may handle. */
  viewer?: Viewer | undefined;
} = $props();

let ctrl = $state.raw<SheetController | undefined>();
let progress = $state(0);
let failure = $state("");
/** The content says another separator than the extension: the person is asked once. */
let contradiction = $state<Start["contradiction"]>();
let filter = $state<{ col: number; anchor: HTMLElement } | undefined>();
let context = $state<{ x: number; y: number } | undefined>();
let finding = $state<{ replace: boolean } | undefined>();
let findBar = $state<{ step(direction: 1 | -1): void } | undefined>();

const handlers = gridHandlers({
  ctrl: () => ctrl,
  find: (replace) => (finding = { replace }),
  step: () => findBar?.step,
  copy: (cut) => void menuCopy(cut),
  paste: (valuesOnly) => void menuPaste(valuesOnly),
  filter: openFilter,
  autofit,
});
/**
 * What the grid says of itself to its commands (EX0c2 P-6): its own read-only state (the reader
 * role, a lock, a file too big), a sort or filter that locks the rows, a filter, frozen panes.
 */
const gridState = () => {
  const c = ctrl;
  if (!c) return { readOnly: true };
  return {
    readOnly: c.readOnly,
    gridLocked: c.structureLocked,
    gridFiltered: c.filtered,
    gridFrozen: c.pinnedRows > 0 || c.pinnedCols > 0,
  };
};
let sheetEl: HTMLElement | undefined = $state();
let gridHost: HTMLDivElement | undefined = $state();
let presence: ReturnType<typeof sheetPresence> | undefined;

$effect(() => {
  const controller = ctrl;
  if (controller) return closeBuffer(() => controller.flushEdit());
});

$effect(() => {
  const controller = ctrl;
  const file = path;
  if (!controller) return;
  const entries = controller.problems.map((p) => ({
    line: p.line,
    message: t(`sheet.problem.${p.kind}`),
  }));
  const text = controller.model.text;
  untrack(() => problems.viewer(file, text, entries));
});

// Open: parse (in a worker for big files), then build the sheet over the document.
$effect(() => {
  const h = handle;
  const b = big;
  const p = path;
  const tab = table;
  let alive = true;
  let off: (() => void) | undefined;
  function start(c: SheetController, doc: DocHandle<Doc> | undefined) {
    const sheetDoc = c.doc;
    if (startOffset !== undefined) c.placeAtOffset(startOffset);
    if (doc) {
      const onChange = ({ patches }: { patches: Parameters<typeof patchWindow>[0] }) =>
        sheetDoc.receive(patchWindow(patches));
      doc.on("change", onChange);
      off = () => doc.off("change", onChange);
      presence = sheetPresence(doc, me, c, (people) => {
        workspace.hearNames(people);
        // While someone else has this file open, cells are written in the form that merges safely.
        c.model.safeQuoting = people.length > 0;
      });
    }
    ctrl = c;
  }
  untrack(() => {
    progress = 0;
    failure = "";
    contradiction = undefined;
    ctrl = undefined;
    const text = b ? b.text : (h?.doc()?.text ?? "");
    const key = b ?? h;
    if (!key) return;
    const port: DocPort = b ? plainPort(b.text) : handlePort(h as DocHandle<Doc>);
    const remembered = rememberedDelimiter(workspace.project, p);
    const opening = startDialect(tab, text, remembered);
    contradiction = opening.contradiction;
    // Rescans (coming back from the text view) keep the person's choice, else follow the content.
    const fixed = tab?.fixed ? tab.delimiter : undefined;
    const hint = {
      ...(remembered ? { forced: remembered } : {}),
      ...(fixed ? { prefer: fixed } : {}),
    };
    // Coming back from the text view: the same sheet document, so undo spans both views.
    const kept = sessions.get(key);
    if (kept) {
      kept.doc.rescan();
      start(kept, h);
      return;
    }
    void openCsv(text, {
      dialect: opening.dialect,
      onProgress: (d) => {
        if (alive) progress = d;
      },
    }).then(
      (model) => {
        if (!alive) return;
        const sheetDoc = new SheetDoc(port, model, hint);
        const made = new SheetController(sheetDoc, {
          readOnly: readOnly || b !== undefined,
          locked: () => workspace.locked,
          closing: () => workspace.closing,
          ...(readOnlyReason ? { readOnlyReason } : {}),
          onUndoKey: () => workspace.undoWithKey(),
          onDialect: (d) => rememberDelimiter(workspace.project, p, d.delimiter),
          ...(isLocked ? { isLocked } : {}),
        });
        sessions.set(key, made);
        start(made, h);
      },
      (err: unknown) => {
        if (alive) failure = err instanceof Error ? err.message : "failed";
      },
    );
  });
  return () => {
    alive = false;
    off?.();
    presence?.stop();
    presence = undefined;
  };
});

$effect(() => {
  // Read the name first: with no presence yet, an optional call would read nothing and never rerun.
  const name = me.name;
  presence?.rename(name);
});

// The selection goes out on the presence channel, a moment after it stops moving.
$effect(() => {
  const c = ctrl;
  if (!c) return;
  void c.ranges;
  void c.tick;
  const timer = setTimeout(() => {
    presence?.broadcast();
    // The rows moved or arrived: where the others are on the grid may have changed too.
    presence?.refresh();
  }, 40);
  return () => clearTimeout(timer);
});

// A notice goes away by itself.
$effect(() => {
  const c = ctrl;
  const n = c?.notice;
  if (!c || !n) return;
  const timer = setTimeout(() => {
    if (c.notice === n) c.notice = undefined;
  }, 9000);
  return () => clearTimeout(timer);
});

// The external-change toast's « Voir » puts the caret on the document's changed cell.
$effect(() => {
  if (!handle) return;
  session.placeAt = (offset) => ctrl?.placeAtOffset(offset);
  return () => {
    session.placeAt = undefined;
  };
});
onDestroy(() => presence?.stop());

const DELIMITER_NAMES = {
  ",": "sheet.delim.name.comma",
  ";": "sheet.delim.name.semicolon",
  "\t": "sheet.delim.name.tab",
  "|": "sheet.delim.name.pipe",
} as const;
const delimiterName = (d: Delimiter): string => t(DELIMITER_NAMES[d]);

/** The one-click answer to the banner: this separator, remembered for this file. */
function chooseDelimiter(delimiter: Delimiter) {
  contradiction = undefined;
  if (ctrl) ctrl.setDialect({ ...ctrl.dialect, delimiter });
  focusGrid();
}

const focusGrid = () => gridHost?.querySelector<HTMLElement>('[role="grid"]')?.focus();

/** The filter menu of the active column (`tables.filter`), at its header's button. */
function openFilter() {
  const c = ctrl;
  if (!c) return;
  const el = gridHost?.querySelector<HTMLElement>(`[data-colhead="${c.active.c}"] .filterbtn`);
  if (el) filter = { col: c.active.c, anchor: el };
}

/** Fits the selected columns' widths to their contents (`tables.autofit`). */
function autofit() {
  const c = ctrl;
  const canvas = document.createElement("canvas").getContext("2d");
  if (!c || !canvas) return;
  canvas.font = "13px sans-serif";
  for (let k = c.primary.c0; k <= c.primary.c1; k++)
    c.autofit(k, (s) => canvas.measureText(s).width);
}

// ---- clipboard from menus (the keyboard goes through the grid's own events) ----
async function menuCopy(cut: boolean) {
  const c = ctrl;
  if (!c) return;
  const data = c.copyData(cut);
  if (!data) return;
  try {
    await navigator.clipboard.write([
      new ClipboardItem({
        "text/plain": new Blob([data.tsv], { type: "text/plain" }),
        "text/html": new Blob([data.html], { type: "text/html" }),
      }),
    ]);
  } catch {
    try {
      await navigator.clipboard.writeText(data.tsv);
    } catch {
      c.notice = { text: t("sheet.clipboardDenied") };
    }
  }
  if (data.neutralised > 0)
    c.notice = {
      text: t("sheet.neutralised", { count: data.neutralised }),
      action: {
        label: t("sheet.copyAsIs"),
        run: () => void navigator.clipboard?.writeText(data.raw),
      },
    };
}

async function menuPaste(valuesOnly: boolean) {
  const c = ctrl;
  if (!c) return;
  try {
    const items = await navigator.clipboard.read();
    let text = "";
    let html = "";
    for (const item of items) {
      if (item.types.includes("text/plain")) text = await (await item.getType("text/plain")).text();
      if (item.types.includes("text/html") && !valuesOnly)
        html = await (await item.getType("text/html")).text();
    }
    c.paste({ text, html });
  } catch {
    try {
      c.paste({ text: await navigator.clipboard.readText() });
    } catch {
      c.notice = { text: t("sheet.pasteDenied") };
    }
  }
}

// ---- the name box and the contents bar ----
let nameDraft = $state<string | undefined>();
function nameKey(e: KeyboardEvent) {
  const c = ctrl;
  if (!c) return;
  if (e.key === "Enter") {
    e.preventDefault();
    if (!c.jumpTo((e.currentTarget as HTMLInputElement).value))
      c.notice = { text: t("sheet.badRef") };
    nameDraft = undefined;
    focusGrid();
  } else if (e.key === "Escape") {
    nameDraft = undefined;
    focusGrid();
  }
}

function barKey(e: KeyboardEvent) {
  const c = ctrl;
  if (!c) return;
  if (e.key === "Enter") {
    e.preventDefault();
    c.commitEdit({ dr: 1, dc: 0 });
    focusGrid();
  } else if (e.key === "Escape") {
    c.cancelEdit();
    focusGrid();
  }
}

const bar = $derived.by(() => {
  const c = ctrl;
  if (!c || c.rowCount === 0) return "";
  if (c.editing) return c.editing.value;
  return c.value(c.active.r, c.active.c);
});

const isFormula = $derived(/^[=+\-@]/.test(bar) && !/^[-+]?\d+([.,]\d+)?$/.test(bar));
const visibleRows = $derived(ctrl?.rowCount ?? 0);
const totalRows = $derived.by(() => {
  const c = ctrl;
  if (!c) return 0;
  c.tick;
  return Math.max(0, c.model.records - c.model.dataStart);
});
</script>

<section class="sheet" aria-label={t("sheet.region")} bind:this={sheetEl} use:surface={{ kind: "grid", handlers, state: gridState, ...(viewer ? { viewer } : {}) }}>
  {#if failure}
    <div class="banner err" role="alert">
      <span>{t("sheet.openFailed")}</span>
      <button type="button" class="btn" onclick={() => onshowtext(0)}>{t("sheet.showText")}</button>
    </div>
  {:else if !ctrl}
    <div class="loading" role="progressbar" aria-label={t("sheet.opening")} aria-valuemin="0" aria-valuemax="100" aria-valuenow={Math.round(progress * 100)}>
      <span>{t("sheet.opening")}</span>
      <progress max="1" value={progress}></progress>
    </div>
  {:else}
    <LocationToolbar
      location="grid/toolbar"
      label={t("sheet.toolbar")}
      target={sheetEl}
      after={() => {
        // The grid takes its focus back, unless the command opened its filter menu.
        if (!filter) focusGrid();
      }}
    >
      {#snippet end()}
        <button type="button" class="fb lbl" data-fixed="show-text" onclick={() => onshowtext(ctrl?.textOffset ?? 0)}>
          <Icon name="code" /><span>{t("sheet.showText")}</span>
        </button>
      {/snippet}
    </LocationToolbar>
    <div class="formula">
      <input
        class="namebox"
        aria-label={t("sheet.nameBox")}
        value={nameDraft ?? ctrl.reference}
        oninput={(e) => (nameDraft = e.currentTarget.value)}
        onfocus={(e) => e.currentTarget.select()}
        onblur={() => (nameDraft = undefined)}
        onkeydown={nameKey}
      />
      <span class="fx" aria-hidden="true">{"ƒx"}</span>
      <input
        class="contents"
        aria-label={t("sheet.contentsBar")}
        value={bar}
        readonly={ctrl.readOnly}
        onfocus={() => {
          if (ctrl && !ctrl.editing && !ctrl.readOnly) ctrl.startEdit(undefined, "bar");
        }}
        oninput={(e) => ctrl?.setEditValue(e.currentTarget.value)}
        onblur={() => ctrl?.blurEdit("bar")}
        onkeydown={barKey}
      />
    </div>
    {#if isFormula}
      <p class="hint" role="note">{t("sheet.formulaHint")}</p>
    {/if}
    {#if ctrl.sorted || ctrl.filtered}
      <div class="banner" aria-live="polite">
        <span>
          {ctrl.filtered ? t("sheet.filterActive", { shown: visibleRows, total: totalRows }) : ""}
          {ctrl.sorted ? t("sheet.sortActive") : ""}
        </span>
        <span class="grow"></span>
        {#if ctrl.sorted && !ctrl.readOnly}
          <button type="button" class="btn" onclick={() => ctrl?.applyToFile()}>{t("sheet.applyToFile")}</button>
        {/if}
        <button type="button" class="btn" onclick={() => ctrl?.clearView()}>{t("sheet.clearView")}</button>
      </div>
    {/if}
    {#if contradiction}
      {@const c = contradiction}
      <div class="banner" data-testid="detected" aria-live="polite">
        <span>{t("sheet.detected", { found: delimiterName(c.found), expected: delimiterName(c.expected) })}</span>
        <span class="grow"></span>
        <button type="button" class="btn" data-testid="use-expected" onclick={() => chooseDelimiter(c.expected)}>
          {t("sheet.detected.use", { name: delimiterName(c.expected) })}
        </button>
        <button type="button" class="btn" data-testid="keep-found" onclick={() => chooseDelimiter(c.found)}>
          {t("sheet.detected.use", { name: delimiterName(c.found) })}
        </button>
      </div>
    {/if}
    {#if ctrl.readOnly}
      <div class="banner" data-testid="readonly" aria-live="polite">
        {ctrl.readOnlyText}
      </div>
    {/if}
    <div class="host" bind:this={gridHost}>
      <Grid
        {ctrl}
        oncontext={(x, y) => (context = { x, y })}
        onfilter={(col, anchor) => (filter = { col, anchor })}
      />
      {#if finding}
        <FindBar
          bind:this={findBar}
          {ctrl}
          withReplace={finding.replace}
          onclose={() => {
            finding = undefined;
            focusGrid();
          }}
        />
      {/if}
    </div>
    <SheetStatus {ctrl} status={workspace.status} peerCount={workspace.peers.length} info={workspace.fileInfo} />
    {#if filter}
      <FilterMenu {ctrl} col={filter.col} anchor={filter.anchor} onclose={() => { filter = undefined; focusGrid(); }} />
    {/if}
    {#if context}
      <LocationMenuAt location="grid/context" x={context.x} y={context.y} label={t("sheet.contextMenu")} target={sheetEl} onclose={() => { context = undefined; focusGrid(); }} />
    {/if}
    {#if ctrl.pending}
      <PasteDialog
        rows={ctrl.pending.rows}
        onyes={() => { ctrl?.confirmPaste(); focusGrid(); }}
        onno={() => { ctrl?.cancelPaste(); focusGrid(); }}
      />
    {/if}
    {#if ctrl.notice}
      {@const n = ctrl.notice}
      <Toast onclose={() => { if (ctrl) ctrl.notice = undefined; }}>
        {n.text}
        {#snippet actions()}
          {#if n.action}
            <button type="button" class="btn" onclick={() => { n.action?.run(); if (ctrl) ctrl.notice = undefined; }}>{n.action.label}</button>
          {/if}
        {/snippet}
      </Toast>
    {/if}
  {/if}
</section>

<style>
  .sheet {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    background: var(--view-bg);
  }
  .grow {
    flex: 1;
  }
  .formula {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 6px;
    background: var(--view-bg);
    border-bottom: 1px solid var(--border);
  }
  .namebox,
  .contents {
    box-sizing: border-box;
    min-height: 30px;
    border: 0;
    border-radius: var(--radius-small);
    padding: 0 10px;
    background: var(--btn-bg);
    color: var(--fg);
    font: inherit;
  }
  .namebox {
    width: 110px;
    text-align: center;
    font-weight: 600;
  }
  .fx {
    color: var(--fg-dim);
    font-style: italic;
    font-family: var(--font-serif);
  }
  .contents {
    flex: 1;
    min-width: 0;
    font-family: var(--font-mono);
    font-size: 13px;
  }
  .namebox:focus-visible,
  .contents:focus-visible {
    outline: var(--focus-ring);
    outline-offset: -2px;
  }
  .hint {
    margin: 0;
    padding: 4px 12px;
    background: var(--info-bg);
    font-size: 12.5px;
  }
  .banner {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 36px;
    padding: 2px 12px;
    background: var(--info-bg);
    font-size: 13px;
  }
  .banner.err {
    background: var(--warning-bg);
  }
  .host {
    position: relative;
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
  }
  .loading {
    display: grid;
    place-content: center;
    gap: 12px;
    height: 100%;
    color: var(--fg-dim);
    text-align: center;
  }
  progress {
    width: 260px;
    accent-color: var(--accent-bg);
  }
</style>
