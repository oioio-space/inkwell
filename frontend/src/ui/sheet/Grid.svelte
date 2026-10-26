<script lang="ts">
import { tick, untrack } from "svelte";
import { colName } from "../../editor/csv/address";
import type { Range } from "../../editor/csv/clipboard";
import { t } from "../../i18n";
import Icon from "../icons/Icon.svelte";
import { personIndex } from "../people";
import { gridKey } from "./keys";
import type { SheetController } from "./sheet.svelte";

/**
 * The grid: rows and columns are virtual (only what is on screen exists in the page), so a
 * file of half a million rows scrolls like a short one. It follows the WAI-ARIA grid pattern:
 * one tab stop, the active cell named by `aria-activedescendant`, arrow keys inside.
 * Positions go through CSSOM (`style:`), never inline `style` attributes: the CSP forbids those.
 */
let {
  ctrl,
  oncontext,
  onfilter,
  onshowtext,
  onfind,
}: {
  ctrl: SheetController;
  oncontext: (x: number, y: number) => void;
  onfilter: (col: number, anchor: HTMLElement) => void;
  onshowtext: () => void;
  onfind: (replace: boolean) => void;
} = $props();

let scroller: HTMLDivElement | undefined = $state();
let scrollTop = $state(0);
let scrollLeft = $state(0);
let viewW = $state(900);
let viewH = $state(500);

const OVERSCAN = 4;
const rowH = $derived(ctrl.rowH);
const hw = $derived(ctrl.headerW);
const rowCount = $derived(ctrl.rowCount);
const colCount = $derived(ctrl.colCount);
const pinned = $derived(Math.min(ctrl.pinnedRows, rowCount));
const frozen = $derived(Math.min(ctrl.pinnedCols, colCount));
const topH = $derived(rowH + pinned * rowH);

/** Left edge of each column; the last entry is the total width. */
const xs = $derived.by(() => {
  const n = colCount;
  const out = new Array<number>(n + 1);
  out[0] = 0;
  for (let i = 0; i < n; i++) out[i + 1] = (out[i] as number) + ctrl.colW(i);
  return out;
});
const totalW = $derived(hw + (xs[colCount] as number));
const frozenW = $derived(xs[frozen] as number);

const rowFirst = $derived(Math.max(pinned, pinned + Math.floor(scrollTop / rowH) - OVERSCAN));
const rowLast = $derived(
  Math.min(rowCount - 1, pinned + Math.ceil((scrollTop + viewH - topH) / rowH) + OVERSCAN),
);
const visibleRows = $derived.by(() => {
  const out: number[] = [];
  for (let d = rowFirst; d <= rowLast; d++) out.push(d);
  return out;
});

function colAt(x: number): number {
  // Binary search on the left edges.
  let lo = 0;
  let hi = colCount - 1;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if ((xs[mid] as number) <= x) lo = mid;
    else hi = mid - 1;
  }
  return lo;
}
const colFirst = $derived(Math.max(frozen, colAt(Math.max(0, scrollLeft + frozenW - hw)) - 1));
const colLast = $derived(Math.min(colCount - 1, colAt(Math.max(0, scrollLeft + viewW - hw)) + 1));
const visibleCols = $derived.by(() => {
  const out: number[] = [];
  for (let c = colFirst; c <= colLast; c++) out.push(c);
  return out;
});
const frozenCols = $derived(Array.from({ length: frozen }, (_, i) => i));
const spacer = $derived(colFirst > frozen ? (xs[colFirst] as number) - frozenW : 0);
const pinnedList = $derived(Array.from({ length: pinned }, (_, i) => i));

// ---- scroll and size ----
let raf = 0;
function onscroll() {
  if (raf) return;
  raf = requestAnimationFrame(() => {
    raf = 0;
    if (!scroller) return;
    scrollTop = scroller.scrollTop;
    scrollLeft = scroller.scrollLeft;
  });
}

$effect(() => {
  if (!scroller) return;
  const el = scroller;
  const ro = new ResizeObserver(() => {
    viewW = el.clientWidth;
    viewH = el.clientHeight;
  });
  ro.observe(el);
  viewW = el.clientWidth;
  viewH = el.clientHeight;
  return () => ro.disconnect();
});

// Keep the active cell on screen after a key moves it.
$effect(() => {
  const { r, c } = ctrl.active;
  const el = scroller;
  if (!el) return;
  untrack(() => {
    if (r >= pinned) {
      const y = (r - pinned) * rowH;
      const room = el.clientHeight - topH;
      if (y < el.scrollTop) el.scrollTop = y;
      else if (y + rowH > el.scrollTop + room) el.scrollTop = y + rowH - room;
    }
    if (c >= frozen) {
      const x = xs[c] as number;
      const left = el.scrollLeft + frozenW;
      const room = el.clientWidth - hw;
      if (x < left) el.scrollLeft = x - frozenW;
      else if (x + ctrl.colW(c) > el.scrollLeft + room) el.scrollLeft = x + ctrl.colW(c) - room;
    }
  });
});

// ---- classes of a cell ----
const inRange = (g: Range, d: number, c: number) =>
  d >= g.r0 && d <= g.r1 && c >= g.c0 && c <= g.c1;

function selClass(d: number, c: number): string {
  let out = "";
  for (const g of ctrl.ranges) {
    if (!inRange(g, d, c)) continue;
    out = "sel";
    if (d === g.r0) out += " et";
    if (d === g.r1) out += " eb";
    if (c === g.c0) out += " el";
    if (c === g.c1) out += " er";
  }
  return out;
}

function remoteOf(d: number, c: number) {
  if (ctrl.remotes.length === 0) return undefined;
  for (const r of ctrl.remotes) if (inRange(r.range, d, c)) return r;
  return undefined;
}

function remoteClass(d: number, c: number): string {
  const r = remoteOf(d, c);
  if (!r) return "";
  const g = r.range;
  let out = `rem who-${personIndex(r.peerId)}`;
  if (d === g.r0) out += " rt";
  if (d === g.r1) out += " rb";
  if (c === g.c0) out += " rl";
  if (c === g.c1) out += " rr";
  return out;
}

function cellClass(d: number, c: number): string {
  const a = ctrl.active;
  let out = selClass(d, c);
  if (a.r === d && a.c === c) out += " act";
  const rc = remoteClass(d, c);
  if (rc) out += ` ${rc}`;
  if (ctrl.typeOf(c) === "number") out += " num";
  if (ctrl.isLocked(d, c)) out += " locked";
  if (ctrl.changed.has(`${ctrl.recordOf(d)}:${c}`)) out += " fresh";
  return out;
}

const colSelected = (c: number) =>
  ctrl.ranges.some((g) => c >= g.c0 && c <= g.c1 && g.r0 === 0 && g.r1 >= rowCount - 1);
const rowSelected = (d: number) =>
  ctrl.ranges.some((g) => d >= g.r0 && d <= g.r1 && g.c0 === 0 && g.c1 >= colCount - 1);
const colActive = (c: number) => ctrl.ranges.some((g) => c >= g.c0 && c <= g.c1);
const rowActive = (d: number) => ctrl.ranges.some((g) => d >= g.r0 && d <= g.r1);

const activeId = $derived(`cell-${ctrl.active.r}-${ctrl.active.c}`);

const announce = $derived.by(() => {
  const { r, c } = ctrl.active;
  if (rowCount === 0) return "";
  return t("sheet.announce", {
    row: ctrl.rowNumber(r),
    column: ctrl.headerName(c),
    value: ctrl.value(r, c) || t("sheet.empty"),
  });
});

// ---- the cell being edited ----
function focusEnd(node: HTMLTextAreaElement) {
  node.focus();
  node.setSelectionRange(node.value.length, node.value.length);
}

function editKey(e: KeyboardEvent) {
  const ed = ctrl.editing;
  if (!ed) return;
  if (e.key === "Escape") {
    e.preventDefault();
    ctrl.cancelEdit();
    scroller?.focus();
  } else if (e.key === "Enter" && e.altKey) {
    e.preventDefault();
    const ta = e.currentTarget as HTMLTextAreaElement;
    const { selectionStart: a, selectionEnd: b } = ta;
    const next = `${ta.value.slice(0, a)}\n${ta.value.slice(b)}`;
    ctrl.setEditValue(next);
    void tick().then(() => ta.setSelectionRange(a + 1, a + 1));
  } else if (e.key === "Enter") {
    e.preventDefault();
    ctrl.commitEdit({ dr: e.shiftKey ? -1 : 1, dc: 0 });
    scroller?.focus();
  } else if (e.key === "Tab") {
    e.preventDefault();
    ctrl.commitEdit({ dr: 0, dc: e.shiftKey ? -1 : 1 });
    scroller?.focus();
  }
  e.stopPropagation();
}

function editBlur() {
  if (ctrl.editing?.from === "cell") ctrl.commitEdit();
}

// ---- pointer ----
type Drag =
  | { kind: "select" }
  | { kind: "cols"; anchor: number }
  | { kind: "rows"; anchor: number }
  | { kind: "resize"; c: number; x: number; w: number }
  | { kind: "movecol"; from: number; x: number; moved: boolean }
  | { kind: "moverow"; from: number; y: number; moved: boolean }
  | { kind: "fill" };
let drag: Drag | undefined;
let drop = $state<{ axis: "col" | "row"; at: number } | undefined>();
let fillTo = $state<Range | undefined>();

function cellAt(target: Element | null) {
  const el = target?.closest<HTMLElement>("[data-r][data-c]");
  if (!el) return undefined;
  return { r: Number(el.dataset.r), c: Number(el.dataset.c), el };
}

function pointerCell(x: number, y: number) {
  return cellAt(document.elementFromPoint(x, y));
}

function edgeScroll(e: PointerEvent) {
  const el = scroller;
  if (!el) return;
  const box = el.getBoundingClientRect();
  if (e.clientY > box.bottom - 24) el.scrollTop += 28;
  else if (e.clientY < box.top + topH + 16) el.scrollTop -= 28;
  if (e.clientX > box.right - 24) el.scrollLeft += 28;
  else if (e.clientX < box.left + hw + frozenW + 16) el.scrollLeft -= 28;
}

function down(e: PointerEvent) {
  if (e.button !== 0 || !scroller) return;
  const target = e.target as Element;
  if (target.closest("textarea, .filterbtn")) return;
  scroller.focus({ preventScroll: true });
  if (ctrl.editing) ctrl.commitEdit();
  const add = e.ctrlKey || e.metaKey;

  if (target.closest("[data-fill]")) {
    drag = { kind: "fill" };
    scroller.setPointerCapture(e.pointerId);
    e.preventDefault();
    return;
  }
  const colHead = target.closest<HTMLElement>("[data-colhead]");
  if (colHead) {
    const c = Number(colHead.dataset.colhead);
    const box = colHead.getBoundingClientRect();
    if (box.right - e.clientX <= 6) {
      drag = { kind: "resize", c, x: e.clientX, w: ctrl.colW(c) };
    } else if (
      !e.shiftKey &&
      !add &&
      ctrl.wholeColumns &&
      c >= ctrl.primary.c0 &&
      c <= ctrl.primary.c1
    ) {
      drag = { kind: "movecol", from: c, x: e.clientX, moved: false };
    } else {
      if (e.shiftKey) ctrl.selectColumns(ctrl.active.c, c);
      else ctrl.selectColumns(c, c, add);
      drag = { kind: "cols", anchor: e.shiftKey ? ctrl.active.c : c };
    }
    scroller.setPointerCapture(e.pointerId);
    e.preventDefault();
    return;
  }
  const rowHead = target.closest<HTMLElement>("[data-rowhead]");
  if (rowHead) {
    const d = Number(rowHead.dataset.rowhead);
    if (!e.shiftKey && !add && ctrl.wholeRows && d >= ctrl.primary.r0 && d <= ctrl.primary.r1) {
      drag = { kind: "moverow", from: d, y: e.clientY, moved: false };
    } else {
      if (e.shiftKey) ctrl.selectRows(ctrl.active.r, d);
      else ctrl.selectRows(d, d, add);
      drag = { kind: "rows", anchor: e.shiftKey ? ctrl.active.r : d };
    }
    scroller.setPointerCapture(e.pointerId);
    e.preventDefault();
    return;
  }
  if (target.closest("[data-corner]")) {
    ctrl.selectAll();
    return;
  }
  const cell = cellAt(target);
  if (cell) {
    ctrl.select(cell.r, cell.c, e.shiftKey, add);
    drag = { kind: "select" };
    scroller.setPointerCapture(e.pointerId);
    e.preventDefault();
  }
}

function move(e: PointerEvent) {
  if (!drag) return;
  if (drag.kind === "resize") {
    ctrl.setWidth(drag.c, drag.w + (e.clientX - drag.x));
    return;
  }
  edgeScroll(e);
  if (drag.kind === "select") {
    const cell = pointerCell(e.clientX, e.clientY);
    if (
      cell &&
      (cell.r !== ctrl.primary.r1 ||
        cell.c !== ctrl.primary.c1 ||
        ctrl.primary.r0 !== ctrl.active.r ||
        ctrl.primary.c0 !== ctrl.active.c)
    )
      ctrl.select(cell.r, cell.c, true);
  } else if (drag.kind === "cols") {
    const h = document
      .elementFromPoint(e.clientX, e.clientY)
      ?.closest<HTMLElement>("[data-colhead]");
    const body = pointerCell(e.clientX, e.clientY);
    const c = h ? Number(h.dataset.colhead) : body?.c;
    if (c !== undefined) ctrl.selectColumns(drag.anchor, c);
  } else if (drag.kind === "rows") {
    const h = document
      .elementFromPoint(e.clientX, e.clientY)
      ?.closest<HTMLElement>("[data-rowhead]");
    const body = pointerCell(e.clientX, e.clientY);
    const d = h ? Number(h.dataset.rowhead) : body?.r;
    if (d !== undefined) ctrl.selectRows(drag.anchor, d);
  } else if (drag.kind === "movecol") {
    if (!drag.moved && Math.abs(e.clientX - drag.x) < 5) return;
    drag.moved = true;
    const box = scroller?.getBoundingClientRect();
    const x = e.clientX - (box?.left ?? 0) + scrollLeft - hw;
    const c = colAt(Math.max(0, x));
    const mid = (xs[c] as number) + ctrl.colW(c) / 2;
    drop = { axis: "col", at: x > mid ? c + 1 : c };
  } else if (drag.kind === "moverow") {
    if (!drag.moved && Math.abs(e.clientY - drag.y) < 5) return;
    drag.moved = true;
    const cell = document.elementFromPoint(e.clientX, e.clientY)?.closest<HTMLElement>("[data-r]");
    if (cell) {
      const d = Number(cell.dataset.r ?? cell.dataset.rowhead);
      const box = cell.getBoundingClientRect();
      drop = { axis: "row", at: e.clientY > box.top + box.height / 2 ? d + 1 : d };
    }
  } else if (drag.kind === "fill") {
    const cell = pointerCell(e.clientX, e.clientY);
    if (!cell) return;
    const p = ctrl.primary;
    const dr = cell.r > p.r1 ? cell.r - p.r1 : cell.r < p.r0 ? cell.r - p.r0 : 0;
    const dc = cell.c > p.c1 ? cell.c - p.c1 : cell.c < p.c0 ? cell.c - p.c0 : 0;
    if (Math.abs(dr) >= Math.abs(dc) && dr !== 0)
      fillTo = { ...p, r0: dr < 0 ? p.r0 + dr : p.r0, r1: dr > 0 ? p.r1 + dr : p.r1 };
    else if (dc !== 0)
      fillTo = { ...p, c0: dc < 0 ? p.c0 + dc : p.c0, c1: dc > 0 ? p.c1 + dc : p.c1 };
    else fillTo = undefined;
  }
}

function up(e: PointerEvent) {
  const d = drag;
  drag = undefined;
  if (!d) return;
  scroller?.releasePointerCapture(e.pointerId);
  if (d.kind === "movecol") {
    if (d.moved && drop?.axis === "col") ctrl.moveCols(drop.at);
    else ctrl.selectColumns(d.from, d.from);
  } else if (d.kind === "moverow") {
    if (d.moved && drop?.axis === "row") ctrl.moveRows(drop.at);
    else ctrl.selectRows(d.from, d.from);
  } else if (d.kind === "fill") {
    const to = pointerCell(e.clientX, e.clientY);
    if (to && fillTo) ctrl.fillHandle(ctrl.primary, { r: to.r, c: to.c });
  }
  drop = undefined;
  fillTo = undefined;
}

function measure() {
  const canvas = document.createElement("canvas").getContext("2d");
  if (!canvas || !scroller) return (s: string) => s.length * 8;
  canvas.font = getComputedStyle(scroller).font;
  return (s: string) => canvas.measureText(s).width;
}

function dblclick(e: MouseEvent) {
  // The pointer was captured by the first press: the event's target is the grid, not the cell.
  const target = document.elementFromPoint(e.clientX, e.clientY) ?? (e.target as Element);
  if (target.closest("[data-fill]")) {
    ctrl.fillHandleToEnd();
    return;
  }
  const colHead = target.closest<HTMLElement>("[data-colhead]");
  if (colHead) {
    const c = Number(colHead.dataset.colhead);
    if (colHead.getBoundingClientRect().right - e.clientX <= 6) ctrl.autofit(c, measure());
    else ctrl.startHeaderEdit(c);
    return;
  }
  if (target.closest("[data-rowhead]")) return;
  if (cellAt(target)) ctrl.startEdit();
}

function contextmenu(e: MouseEvent) {
  e.preventDefault();
  const target = e.target as Element;
  const cell = cellAt(target);
  const colHead = target.closest<HTMLElement>("[data-colhead]");
  const rowHead = target.closest<HTMLElement>("[data-rowhead]");
  if (colHead) {
    const c = Number(colHead.dataset.colhead);
    if (!ctrl.ranges.some((g) => ctrl.wholeColumns && c >= g.c0 && c <= g.c1))
      ctrl.selectColumns(c, c);
  } else if (rowHead) {
    const d = Number(rowHead.dataset.rowhead);
    if (!ctrl.ranges.some((g) => ctrl.wholeRows && d >= g.r0 && d <= g.r1)) ctrl.selectRows(d, d);
  } else if (cell && !ctrl.ranges.some((g) => inRange(g, cell.r, cell.c))) {
    ctrl.select(cell.r, cell.c);
  }
  oncontext(e.clientX, e.clientY);
}

function wheel(e: WheelEvent) {
  if (!(e.ctrlKey || e.metaKey)) return;
  e.preventDefault();
  ctrl.setZoom(ctrl.zoom - Math.sign(e.deltaY) * 0.1);
}

// ---- clipboard ----
function copy(e: ClipboardEvent, cut = false) {
  if (ctrl.editing) return;
  const data = ctrl.copyData(cut);
  if (!data || !e.clipboardData) return;
  e.preventDefault();
  e.clipboardData.setData("text/plain", data.tsv);
  e.clipboardData.setData("text/html", data.html);
  if (data.neutralised > 0) {
    ctrl.notice = {
      text: t("sheet.neutralised", { count: data.neutralised }),
      action: {
        label: t("sheet.copyAsIs"),
        run: () => void navigator.clipboard?.writeText(data.raw),
      },
    };
  }
}

let plainPaste = false;
function paste(e: ClipboardEvent) {
  if (ctrl.editing || !e.clipboardData) return;
  e.preventDefault();
  const values = plainPaste;
  plainPaste = false;
  ctrl.paste({
    text: e.clipboardData.getData("text/plain"),
    ...(values ? {} : { html: e.clipboardData.getData("text/html") }),
  });
}

function keydown(e: KeyboardEvent) {
  if (ctrl.editing) return;
  // Ctrl+Shift+V pastes the values only: the page's formatting and tables are left out.
  plainPaste = (e.ctrlKey || e.metaKey) && e.shiftKey && e.key.toLowerCase() === "v";
  const done = gridKey(ctrl, e, {
    pageRows: Math.max(1, Math.floor((viewH - topH) / rowH) - 1),
    zoomReset: () => ctrl.setZoom(1),
    showText: onshowtext,
    openFind: onfind,
    openFilter: () => {
      const el = scroller?.querySelector<HTMLElement>(
        `[data-colhead="${ctrl.active.c}"] .filterbtn`,
      );
      if (el) onfilter(ctrl.active.c, el);
    },
    openContext: () => {
      const el = scroller?.querySelector<HTMLElement>(`#${activeId}`);
      const box = el?.getBoundingClientRect();
      oncontext((box?.left ?? 100) + 20, (box?.bottom ?? 100) - 6);
    },
  });
  if (done) {
    e.preventDefault();
    e.stopPropagation();
  }
}

// The fill handle sits on the corner of the selection's last cell.
const handleAt = $derived.by(() => {
  const p = ctrl.primary;
  return { r: p.r1, c: p.c1 };
});
const canFill = $derived(!ctrl.readOnly && !ctrl.editing && rowCount > 0);

const dropX = $derived(
  drop?.axis === "col" ? hw + (xs[Math.min(drop.at, colCount)] as number) : -1,
);
const dropY = $derived(
  drop?.axis === "row" ? topH + (Math.max(pinned, drop.at) - pinned) * rowH : -1,
);
const fillRect = $derived.by(() => {
  const g = fillTo;
  if (!g) return undefined;
  const topRow = Math.max(g.r0, pinned);
  return {
    x: hw + (xs[g.c0] as number),
    y: topH + (topRow - pinned) * rowH,
    w: (xs[g.c1 + 1] as number) - (xs[g.c0] as number),
    h: (g.r1 - topRow + 1) * rowH,
  };
});
</script>

{#snippet dataRow(d: number, top: number | undefined)}
  <div
    class="row"
    role="row"
    aria-rowindex={d + 2}
    aria-selected={rowActive(d) || undefined}
    class:pinrow={top === undefined}
    style:height="{rowH}px"
    style:top={top === undefined ? undefined : `${top}px`}
    style:width="{totalW}px"
  >
    <div
      class="rh"
      class:cur={rowActive(d)}
      class:selected={rowSelected(d)}
      data-rowhead={d}
      data-r={d}
      role="rowheader"
      style:width="{hw}px"
    >
      {ctrl.rowNumber(d)}
    </div>
    {#each frozenCols as c (c)}
      {@render cellView(d, c, true)}
    {/each}
    {#if spacer > 0}<div class="gap" style:width="{spacer}px"></div>{/if}
    {#each visibleCols as c (c)}
      {@render cellView(d, c, false)}
    {/each}
  </div>
{/snippet}

{#snippet cellView(d: number, c: number, frozenCell: boolean)}
  {@const ed = ctrl.editing}
  <div
    id="cell-{d}-{c}"
    class="cell {cellClass(d, c)}"
    class:frozen={frozenCell}
    class:lastfrozen={frozenCell && c === frozen - 1}
    role="gridcell"
    aria-colindex={c + 1}
    aria-selected={inRange(ctrl.primary, d, c) || undefined}
    aria-readonly={!ctrl.canEdit(d, c) || undefined}
    data-r={d}
    data-c={c}
    style:width="{ctrl.colW(c)}px"
    style:left={frozenCell ? `${hw + (xs[c] as number)}px` : undefined}
  >
    {#if ed && ed.r === d && ed.c === c && ed.from === "cell"}
      <textarea
        class="edit"
        aria-label={t("sheet.editCell", { name: ctrl.headerName(c) })}
        value={ed.value}
        use:focusEnd
        oninput={(e) => ctrl.setEditValue(e.currentTarget.value)}
        onkeydown={editKey}
        onblur={editBlur}
      ></textarea>
    {:else}
      <span class="txt">{ed && ed.from === "bar" && ed.r === d && ed.c === c ? ed.value : ctrl.display(d, c)}</span>
    {/if}
    {#if canFill && handleAt.r === d && handleAt.c === c && ctrl.ranges.length === 1}
      <span class="fill" data-fill title={t("sheet.fillHandle")}></span>
    {/if}
    {#if remoteOf(d, c) && remoteOf(d, c)?.range.r0 === d && remoteOf(d, c)?.range.c0 === c}
      <span class="tag who-{personIndex(remoteOf(d, c)?.peerId ?? '')}" aria-hidden="true">{remoteOf(d, c)?.name}</span>
    {/if}
  </div>
{/snippet}

{#snippet headCell(c: number, frozenCell: boolean)}
  {@const sorted = ctrl.view.sort?.col === c}
  {@const filtered = ctrl.view.filters.has(c)}
  <div
    class="ch"
    class:cur={colActive(c)}
    class:selected={colSelected(c)}
    class:frozen={frozenCell}
    class:lastfrozen={frozenCell && c === frozen - 1}
    role="columnheader"
    aria-colindex={c + 1}
    aria-sort={sorted ? (ctrl.view.sort?.dir === 1 ? "ascending" : "descending") : undefined}
    data-colhead={c}
    style:width="{ctrl.colW(c)}px"
    style:left={frozenCell ? `${hw + (xs[c] as number)}px` : undefined}
  >
    {#if ctrl.editing && ctrl.editing.r === -1 && ctrl.editing.c === c}
      <textarea
        class="edit"
        aria-label={t("sheet.renameColumn")}
        value={ctrl.editing.value}
        use:focusEnd
        oninput={(e) => ctrl.setEditValue(e.currentTarget.value)}
        onkeydown={editKey}
        onblur={() => ctrl.commitEdit()}
      ></textarea>
    {:else}
      <span class="letter">{ctrl.dialect.header ? colName(c) : ""}</span>
      <span class="name">{ctrl.headerName(c)}</span>
      {#if sorted}<span class="ind" aria-hidden="true"><Icon name={ctrl.view.sort?.dir === 1 ? "sortasc" : "sortdesc"} size={12} /></span>{/if}
      <button
        type="button"
        class="filterbtn"
        class:on={filtered}
        tabindex="-1"
        aria-label={t("sheet.filterColumn", { name: ctrl.headerName(c) })}
        aria-haspopup="dialog"
        onclick={(e) => onfilter(c, e.currentTarget)}
      >
        <Icon name={filtered ? "filter" : "caretdown"} size={12} />
      </button>
    {/if}
  </div>
{/snippet}

<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<div
  bind:this={scroller}
  class="grid"
  role="grid"
  tabindex="0"
  aria-label={t("sheet.label")}
  aria-rowcount={rowCount + 1}
  aria-colcount={colCount}
  aria-multiselectable="true"
  aria-readonly={ctrl.readOnly || undefined}
  aria-activedescendant={rowCount > 0 ? activeId : undefined}
  style:--zoom={ctrl.zoom}
  onscroll={onscroll}
  onkeydown={keydown}
  onpointerdown={down}
  onpointermove={move}
  onpointerup={up}
  onpointercancel={up}
  ondblclick={dblclick}
  oncontextmenu={contextmenu}
  onwheel={wheel}
  oncopy={(e) => copy(e)}
  oncut={(e) => copy(e, true)}
  onpaste={paste}
>
  <div class="top" role="rowgroup" style:width="{totalW}px" style:height="{topH}px">
    <div class="row head" role="row" aria-rowindex="1" style:height="{rowH}px" style:width="{totalW}px">
      <div class="corner" data-corner role="columnheader" aria-label={t("sheet.selectAll")} style:width="{hw}px"></div>
      {#each frozenCols as c (c)}
        {@render headCell(c, true)}
      {/each}
      {#if spacer > 0}<div class="gap" style:width="{spacer}px"></div>{/if}
      {#each visibleCols as c (c)}
        {@render headCell(c, false)}
      {/each}
    </div>
    {#each pinnedList as d (d)}
      {@render dataRow(d, undefined)}
    {/each}
  </div>
  <div class="body" role="rowgroup" style:width="{totalW}px" style:height="{Math.max(0, rowCount - pinned) * rowH}px">
    {#each visibleRows as d (d)}
      {@render dataRow(d, (d - pinned) * rowH)}
    {/each}
    {#if dropY >= 0}<div class="drop h" style:top="{dropY - topH}px" style:width="{totalW}px"></div>{/if}
    {#if fillRect}
      <div
        class="fillprev"
        style:left="{fillRect.x}px"
        style:top="{fillRect.y - topH}px"
        style:width="{fillRect.w}px"
        style:height="{fillRect.h}px"
      ></div>
    {/if}
  </div>
  {#if dropX >= 0}<div class="drop v" style:left="{dropX}px" style:height="{topH + Math.max(0, rowCount - pinned) * rowH}px"></div>{/if}
  {#if rowCount === 0}
    <p class="empty">{t("sheet.empty.table")}</p>
  {/if}
</div>
<div class="sr" aria-live="polite">{announce}</div>

<style>
  .grid {
    --line: var(--border);
    --sel: var(--accent-bg);
    position: relative;
    flex: 1;
    min-height: 0;
    overflow: auto;
    background: var(--view-bg);
    color: var(--fg);
    font: calc(13px * var(--zoom, 1)) / 1.2 var(--font-ui);
    outline: none;
  }
  .grid:focus-visible {
    box-shadow: inset 0 0 0 2px color-mix(in srgb, var(--sel) 55%, transparent);
  }
  .top {
    position: sticky;
    top: 0;
    z-index: 4;
    background: var(--view-bg);
  }
  .body {
    position: relative;
  }
  .row {
    display: flex;
    box-sizing: border-box;
  }
  .body > .row {
    position: absolute;
    left: 0;
  }
  .head {
    background: var(--side-bg-2);
  }
  .rh,
  .corner {
    position: sticky;
    left: 0;
    z-index: 3;
    box-sizing: border-box;
    flex: none;
    display: grid;
    place-items: center;
    background: var(--side-bg-2);
    color: var(--fg-dim);
    font-size: 0.92em;
    font-variant-numeric: tabular-nums;
    border-right: 1px solid var(--line);
    border-bottom: 1px solid var(--line);
    user-select: none;
  }
  .rh.cur,
  .ch.cur {
    background: color-mix(in srgb, var(--sel) 16%, var(--side-bg-2));
    color: var(--fg);
    font-weight: 600;
  }
  .rh.selected,
  .ch.selected {
    background: color-mix(in srgb, var(--sel) 30%, var(--side-bg-2));
  }
  .corner {
    border-bottom: 1px solid var(--line);
  }
  .ch {
    position: relative;
    box-sizing: border-box;
    flex: none;
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 0 6px 0 8px;
    background: var(--side-bg-2);
    border-right: 1px solid var(--line);
    border-bottom: 1px solid var(--line);
    overflow: hidden;
    user-select: none;
    cursor: default;
  }
  .ch.frozen,
  .cell.frozen {
    position: sticky;
    z-index: 2;
  }
  .ch.lastfrozen,
  .cell.lastfrozen {
    box-shadow: 2px 0 0 var(--line);
  }
  .ch::after {
    content: "";
    position: absolute;
    top: 0;
    right: 0;
    bottom: 0;
    width: 6px;
    cursor: col-resize;
  }
  .letter {
    color: var(--fg-dim);
    font-size: 0.85em;
  }
  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ind {
    color: var(--accent);
    display: inline-flex;
  }
  .filterbtn {
    display: inline-grid;
    place-items: center;
    flex: none;
    width: 20px;
    height: 20px;
    padding: 0;
    border: 0;
    border-radius: 5px;
    background: transparent;
    color: var(--fg-dim);
    opacity: 0;
  }
  .ch:hover .filterbtn,
  .filterbtn:focus-visible,
  .filterbtn.on {
    opacity: 1;
  }
  .filterbtn.on {
    color: var(--accent);
  }
  .filterbtn:hover {
    background: var(--hover-bg);
  }
  .cell {
    position: relative;
    box-sizing: border-box;
    flex: none;
    display: flex;
    align-items: center;
    padding: 0 8px;
    background: var(--view-bg);
    border-right: 1px solid var(--line);
    border-bottom: 1px solid var(--line);
    white-space: nowrap;
    cursor: cell;
  }
  .cell.num {
    justify-content: flex-end;
    font-variant-numeric: tabular-nums;
  }
  .txt {
    min-width: 0;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .cell.sel {
    background: color-mix(in srgb, var(--sel) 13%, var(--view-bg));
  }
  .cell.act {
    background: var(--view-bg);
  }
  .cell.locked {
    background: repeating-linear-gradient(135deg, transparent 0 6px, var(--hover-bg) 6px 7px);
    color: var(--fg-dim);
  }
  .cell.fresh {
    animation: fresh 4s ease-out;
    background: color-mix(in srgb, var(--warning) 22%, var(--view-bg));
  }
  @keyframes fresh {
    from {
      background: color-mix(in srgb, var(--warning) 45%, var(--view-bg));
    }
  }
  /* The outline of a selected range: the edge classes draw only its border. */
  .cell.sel::after,
  .cell.rem::before {
    content: "";
    position: absolute;
    inset: 0;
    pointer-events: none;
    border: 0 solid var(--sel);
  }
  .cell.sel.et::after {
    border-top-width: 2px;
  }
  .cell.sel.eb::after {
    border-bottom-width: 2px;
  }
  .cell.sel.el::after {
    border-left-width: 2px;
  }
  .cell.sel.er::after {
    border-right-width: 2px;
  }
  .fill {
    position: absolute;
    right: -4px;
    bottom: -4px;
    z-index: 3;
    width: 9px;
    height: 9px;
    box-sizing: border-box;
    border: 1.5px solid var(--view-bg);
    background: var(--sel);
    cursor: crosshair;
  }
  .cell.rem::before {
    z-index: 1;
    border-color: var(--who);
  }
  .cell.rem.rt::before {
    border-top-width: 2px;
  }
  .cell.rem.rb::before {
    border-bottom-width: 2px;
  }
  .cell.rem.rl::before {
    border-left-width: 2px;
  }
  .cell.rem.rr::before {
    border-right-width: 2px;
  }
  .tag {
    position: absolute;
    left: -2px;
    bottom: calc(100% - 1px);
    z-index: 5;
    padding: 0 4px;
    border-radius: 4px 4px 4px 0;
    background: var(--who);
    color: #000;
    font: 600 11px/1.5 var(--font-ui);
    white-space: nowrap;
    pointer-events: none;
  }
  .edit {
    position: absolute;
    inset: 0;
    z-index: 6;
    box-sizing: border-box;
    min-height: 100%;
    margin: 0;
    padding: 4px 8px;
    border: 2px solid var(--sel);
    border-radius: 0;
    outline: none;
    resize: none;
    overflow: hidden;
    background: var(--view-bg);
    color: var(--fg);
    font: inherit;
    white-space: pre;
  }
  .ch .edit {
    padding: 0 6px;
  }
  .drop {
    position: absolute;
    z-index: 6;
    background: var(--sel);
    pointer-events: none;
  }
  .drop.h {
    left: 0;
    height: 2px;
  }
  .drop.v {
    top: 0;
    width: 2px;
  }
  .fillprev {
    position: absolute;
    z-index: 4;
    box-sizing: border-box;
    border: 2px dashed var(--sel);
    pointer-events: none;
  }
  .empty {
    position: absolute;
    inset: 80px 0 auto;
    text-align: center;
    color: var(--fg-dim);
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
  .who-1 { --who: var(--person-1); }
  .who-2 { --who: var(--person-2); }
  .who-3 { --who: var(--person-3); }
  .who-4 { --who: var(--person-4); }
  .who-5 { --who: var(--person-5); }
  .who-6 { --who: var(--person-6); }
  .who-7 { --who: var(--person-7); }
  .who-8 { --who: var(--person-8); }
  @media (prefers-reduced-motion: reduce) {
    .cell.fresh {
      animation: none;
    }
  }
</style>
