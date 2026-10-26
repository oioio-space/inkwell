<script lang="ts">
import type { DocHandle } from "@automerge/automerge-repo";
import { onDestroy, untrack } from "svelte";
import { delimiterOf, detectDialect, quoteOf } from "../../editor/csv/dialect";
import { openCsv } from "../../editor/csv/open";
import { patchWindow } from "../../editor/csv/patchWindow";
import { handlePort, SheetDoc } from "../../editor/csv/sheetDoc";
import { t, tn } from "../../i18n";
import { type Doc, workspace } from "../../workspace/workspace.svelte";
import Icon from "../icons/Icon.svelte";
import type { IconName } from "../icons/paths";
import Toast from "../Toast.svelte";
import ContextMenu from "./ContextMenu.svelte";
import FilterMenu from "./FilterMenu.svelte";
import FindBar from "./FindBar.svelte";
import Grid from "./Grid.svelte";
import type { MenuItem } from "./menu";
import PasteDialog from "./PasteDialog.svelte";
import { sheetPresence } from "./presence";
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
  me,
  path,
  readOnly = false,
  isLocked,
  startOffset,
  onshowtext,
}: {
  handle: DocHandle<Doc>;
  me: { id: string; name: string };
  path: string;
  readOnly?: boolean;
  isLocked?: SheetOptions["isLocked"];
  /** Where the text view had its caret, to open on the matching cell. */
  startOffset?: number | undefined;
  onshowtext: (offset: number) => void;
} = $props();

let ctrl = $state.raw<SheetController | undefined>();
let progress = $state(0);
let failure = $state("");
let filter = $state<{ col: number; anchor: HTMLElement } | undefined>();
let context = $state<{ x: number; y: number } | undefined>();
let finding = $state<{ replace: boolean } | undefined>();
let gridHost: HTMLDivElement | undefined = $state();
let presence: ReturnType<typeof sheetPresence> | undefined;

// Open: parse (in a worker for big files), then build the sheet over the document.
$effect(() => {
  const h = handle;
  const p = path;
  let alive = true;
  let off: (() => void) | undefined;
  function start(c: SheetController, doc: DocHandle<Doc>) {
    const sheetDoc = c.doc;
    if (startOffset !== undefined) c.placeAtOffset(startOffset);
    const onChange = ({ patches }: { patches: Parameters<typeof patchWindow>[0] }) =>
      sheetDoc.receive(patchWindow(patches));
    doc.on("change", onChange);
    off = () => doc.off("change", onChange);
    presence = sheetPresence(doc, me, c, (people) => {
      workspace.names = Object.fromEntries(people.map((x) => [x.peerId, x.name]));
      // While someone else has this file open, cells are written in the form that merges safely.
      c.model.safeQuoting = people.length > 0;
    });
    ctrl = c;
  }
  untrack(() => {
    progress = 0;
    failure = "";
    ctrl = undefined;
    const text = h.doc()?.text ?? "";
    const hint = delimiterOf(p);
    // Coming back from the text view: the same sheet document, so undo spans both views.
    const kept = sessions.get(h);
    if (kept) {
      kept.doc.rescan();
      start(kept, h);
      return;
    }
    void openCsv(text, {
      dialect: { ...detectDialect(text, hint), ...(quoteOf(p) ? {} : { quote: false }) },
      onProgress: (d) => {
        if (alive) progress = d;
      },
    }).then(
      (model) => {
        if (!alive) return;
        const sheetDoc = new SheetDoc(handlePort(h), model, hint);
        const made = new SheetController(sheetDoc, {
          readOnly,
          ...(isLocked ? { isLocked } : {}),
        });
        sessions.set(h, made);
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

onDestroy(() => presence?.stop());

const focusGrid = () => gridHost?.querySelector<HTMLElement>('[role="grid"]')?.focus();

type Btn = {
  id: string;
  icon: IconName;
  label: string;
  run: () => void;
  off?: boolean;
  on?: boolean;
};

const groups = $derived.by((): Btn[][] => {
  const c = ctrl;
  if (!c) return [];
  const ro = c.readOnly;
  const locked = c.structureLocked;
  return [
    [
      { id: "undo", icon: "undo", label: t("sheet.undo"), run: () => c.undo(), off: ro },
      { id: "redo", icon: "redo", label: t("sheet.redo"), run: () => c.redo(), off: ro },
    ],
    [
      {
        id: "rowAdd",
        icon: "rowadd",
        label: t("sheet.insertRow"),
        run: () => c.insertRows("above"),
        off: ro || locked,
      },
      {
        id: "rowDel",
        icon: "rowdel",
        label: t("sheet.deleteRow"),
        run: () => c.deleteRows(),
        off: ro,
      },
      {
        id: "colAdd",
        icon: "coladd",
        label: t("sheet.insertCol"),
        run: () => c.insertCols("left"),
        off: ro,
      },
      {
        id: "colDel",
        icon: "coldel",
        label: t("sheet.deleteCol"),
        run: () => c.deleteCols(),
        off: ro,
      },
    ],
    [
      {
        id: "sortAsc",
        icon: "sortasc",
        label: t("sheet.sortAsc"),
        run: () => c.sortBy(c.active.c, 1),
      },
      {
        id: "sortDesc",
        icon: "sortdesc",
        label: t("sheet.sortDesc"),
        run: () => c.sortBy(c.active.c, -1),
      },
      {
        id: "filter",
        icon: "filter",
        label: t("sheet.filter"),
        run: () => {
          const el = gridHost?.querySelector<HTMLElement>(
            `[data-colhead="${c.active.c}"] .filterbtn`,
          );
          if (el) filter = { col: c.active.c, anchor: el };
        },
        on: c.filtered,
      },
    ],
    [
      {
        id: "freeze",
        icon: "freeze",
        label: c.pinnedRows || c.pinnedCols ? t("sheet.unfreeze") : t("sheet.freeze"),
        run: () => (c.pinnedRows || c.pinnedCols ? c.unfreeze() : c.freezeToSelection()),
        on: c.pinnedRows > 0 || c.pinnedCols > 0,
      },
    ],
  ];
});

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

const menuItems = $derived.by((): MenuItem[] => {
  const c = ctrl;
  if (!c) return [];
  const ro = c.readOnly;
  const items: MenuItem[] = [];
  if (!ro)
    items.push({
      label: t("sheet.cut"),
      icon: "copy",
      shortcut: "Ctrl+X",
      run: () => void menuCopy(true),
    });
  items.push({
    label: t("sheet.copy"),
    icon: "copy",
    shortcut: "Ctrl+C",
    run: () => void menuCopy(false),
  });
  if (!ro) {
    items.push({ label: t("sheet.paste"), shortcut: "Ctrl+V", run: () => void menuPaste(false) });
    items.push({
      label: t("sheet.pasteValues"),
      shortcut: "Ctrl+Maj+V",
      run: () => void menuPaste(true),
    });
    items.push({ separator: true, label: "" });
    items.push({
      label: t("sheet.insertRowAbove"),
      icon: "rowadd",
      shortcut: "Ctrl++",
      disabled: c.structureLocked,
      run: () => c.insertRows("above"),
    });
    items.push({
      label: t("sheet.insertRowBelow"),
      icon: "rowadd",
      disabled: c.structureLocked,
      run: () => c.insertRows("below"),
    });
    items.push({
      label: t("sheet.insertColLeft"),
      icon: "coladd",
      run: () => c.insertCols("left"),
    });
    items.push({
      label: t("sheet.insertColRight"),
      icon: "coladd",
      run: () => c.insertCols("right"),
    });
    items.push({
      label: t("sheet.deleteRows"),
      icon: "rowdel",
      shortcut: "Ctrl+-",
      run: () => c.deleteRows(),
    });
    items.push({ label: t("sheet.deleteCols"), icon: "coldel", run: () => c.deleteCols() });
    items.push({
      label: t("sheet.duplicateRow"),
      disabled: c.structureLocked,
      run: () => c.duplicateRow(),
    });
    items.push({ label: t("sheet.clear"), shortcut: "Suppr", run: () => c.clear() });
  }
  items.push({ separator: true, label: "" });
  const col = c.active.c;
  items.push({ label: t("sheet.sortAsc"), icon: "sortasc", run: () => c.sortBy(col, 1) });
  items.push({ label: t("sheet.sortDesc"), icon: "sortdesc", run: () => c.sortBy(col, -1) });
  items.push({
    label: t("sheet.filterByValue"),
    icon: "filter",
    run: () => c.setFilter(col, { values: new Set([c.value(c.active.r, col)]) }),
  });
  items.push({ separator: true, label: "" });
  items.push({ label: t("sheet.freezeHere"), icon: "freeze", run: () => c.freezeToSelection() });
  items.push({
    label: t("sheet.autofit"),
    run: () => {
      const canvas = document.createElement("canvas").getContext("2d");
      if (!canvas) return;
      canvas.font = "13px sans-serif";
      for (let k = c.primary.c0; k <= c.primary.c1; k++)
        c.autofit(k, (s) => canvas.measureText(s).width);
    },
  });
  return items;
});

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

<section class="sheet" aria-label={t("sheet.region")}>
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
    <div class="toolbar" role="toolbar" aria-label={t("sheet.toolbar")}>
      {#each groups as group, gi (gi)}
        {#if gi > 0}<span class="vsep" aria-hidden="true"></span>{/if}
        {#each group as b (b.id)}
          <button
            type="button"
            class="fb"
            data-sheet={b.id}
            aria-label={b.label}
            title={b.label}
            aria-pressed={b.on}
            disabled={b.off}
            onmousedown={(e) => e.preventDefault()}
            onclick={() => {
              b.run();
              if (b.id !== "filter") focusGrid();
            }}
          >
            <Icon name={b.icon} />
          </button>
        {/each}
      {/each}
      <span class="grow"></span>
      <button
        type="button"
        class="fb lbl"
        data-sheet="text"
        onclick={() => onshowtext(ctrl?.textOffset ?? 0)}
      >
        <Icon name="code" /><span>{t("sheet.showText")}</span>
      </button>
    </div>
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
        onblur={() => {
          if (ctrl?.editing?.from === "bar") ctrl.commitEdit();
        }}
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
    {#if ctrl.readOnly}
      <div class="banner" aria-live="polite">{t("sheet.readOnly")}</div>
    {/if}
    <div class="host" bind:this={gridHost}>
      <Grid
        {ctrl}
        oncontext={(x, y) => (context = { x, y })}
        onfilter={(col, anchor) => (filter = { col, anchor })}
        onshowtext={() => onshowtext(ctrl?.textOffset ?? 0)}
        onfind={(replace) => (finding = { replace })}
      />
      {#if finding}
        <FindBar
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
      <ContextMenu x={context.x} y={context.y} items={menuItems} label={t("sheet.contextMenu")} onclose={() => { context = undefined; focusGrid(); }} />
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
  .toolbar {
    display: flex;
    align-items: center;
    gap: 2px;
    min-height: 40px;
    padding: 3px 6px;
    background: var(--headerbar-bg);
    box-shadow: 0 1px var(--shade);
    position: relative;
    z-index: 5;
  }
  .vsep {
    width: 1px;
    height: 20px;
    margin: 0 6px;
    background: var(--shade);
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
