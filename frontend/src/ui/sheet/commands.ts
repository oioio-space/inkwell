import type { SheetController } from "./sheet.svelte";

/** What the sheet's commands reach: its controller (absent while it opens) and its find bar. */
export interface GridTarget {
  ctrl(): SheetController | undefined;
  /** Opens the find bar, with the replace field or not. */
  find(replace: boolean): void;
  /** The next or previous match; undefined while the find bar is closed. */
  step(): ((direction: 1 | -1) => void) | undefined;
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
  };
}
