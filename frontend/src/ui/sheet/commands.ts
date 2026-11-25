import type { SheetController } from "./sheet.svelte";

/**
 * What the sheet's commands reach: its controller (absent while it opens), its find bar, and what
 * only the sheet's page can do (the system clipboard, the filter menu at its column, a measure).
 */
export interface GridTarget {
  ctrl(): SheetController | undefined;
  /** Opens the find bar, with the replace field or not. */
  find(replace: boolean): void;
  /** The next or previous match; undefined while the find bar is closed. */
  step(): ((direction: 1 | -1) => void) | undefined;
  /** Copies (or cuts) the selection to the system clipboard. */
  copy(cut: boolean): void;
  /** Pastes the system clipboard, its values only or not. */
  paste(valuesOnly: boolean): void;
  /** Opens the filter menu of the active column. */
  filter(): void;
  /** Fits the selected columns' widths to their contents. */
  autofit(): void;
}

/**
 * The grid surface's handlers (`tables.grid`): the commands its manifest declares, from ext/tables
 * and the delegable ones of ext/app. F3 with the find bar closed opens it.
 */
export function gridHandlers(g: GridTarget): Record<string, () => boolean> {
  const on = (act: (c: SheetController) => void) => () => {
    const c = g.ctrl();
    if (!c) return false;
    act(c);
    return true;
  };
  // Typing in a cell, Ctrl+± are the cell's: the grid does not zoom under the text being typed.
  const zoom = (to: (c: SheetController) => number) => () => {
    const c = g.ctrl();
    if (!c || c.editing) return false;
    c.setZoom(to(c));
    return true;
  };
  const find = (direction: 1 | -1) => () => {
    const step = g.step();
    if (step) step(direction);
    else g.find(false);
    return true;
  };
  // What only the sheet's page does, once its controller is there.
  const page = (act: () => void) => () => {
    if (!g.ctrl()) return false;
    act();
    return true;
  };
  const open = (replace: boolean) => () => {
    g.find(replace);
    return true;
  };
  return {
    "app.find": open(false),
    "app.replace": open(true),
    "app.find-next": find(1),
    "app.find-previous": find(-1),
    "app.zoom-in": zoom((c) => c.zoom + 0.1),
    "app.zoom-out": zoom((c) => c.zoom - 0.1),
    "app.zoom-reset": zoom(() => 1),
    "app.undo": on((c) => c.undo()),
    "app.redo": on((c) => c.redo()),
    "tables.fill-down": on((c) => c.fillDown()),
    "tables.fill-right": on((c) => c.fillRight()),
    "tables.insert-today": on((c) => c.insertToday()),
    "tables.insert-cells": on((c) => c.insertAtSelection()),
    "tables.delete-cells": on((c) => c.deleteAtSelection()),
    // The bar's and the context menu's (EX0c2): what they did by hand before.
    "tables.cut": page(() => g.copy(true)),
    "tables.copy": page(() => g.copy(false)),
    "tables.paste": page(() => g.paste(false)),
    "tables.paste-values": page(() => g.paste(true)),
    "tables.insert-row-above": on((c) => c.insertRows("above")),
    "tables.insert-row-below": on((c) => c.insertRows("below")),
    "tables.insert-col-left": on((c) => c.insertCols("left")),
    "tables.insert-col-right": on((c) => c.insertCols("right")),
    "tables.delete-rows": on((c) => c.deleteRows()),
    "tables.delete-cols": on((c) => c.deleteCols()),
    "tables.duplicate-row": on((c) => c.duplicateRow()),
    "tables.clear": on((c) => c.clear()),
    "tables.sort-asc": on((c) => c.sortBy(c.active.c, 1)),
    "tables.sort-desc": on((c) => c.sortBy(c.active.c, -1)),
    "tables.filter": page(() => g.filter()),
    "tables.filter-by-value": on((c) =>
      c.setFilter(c.active.c, { values: new Set([c.value(c.active.r, c.active.c)]) }),
    ),
    "tables.freeze": on((c) => c.freezeToSelection()),
    "tables.unfreeze": on((c) => c.unfreeze()),
    "tables.autofit": page(() => g.autofit()),
  };
}
