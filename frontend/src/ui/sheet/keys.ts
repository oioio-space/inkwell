import type { SheetController } from "./sheet.svelte";

export interface KeyHelpers {
  pageRows: number;
  openFilter: () => void;
  openContext: () => void;
}

/**
 * The grid's own keys, after Excel's and the WAI-ARIA grid pattern (each listed in
 * extensions/keytables.ts, `GRID_KEYS`): arrows move, Ctrl+arrows jump to the edge of the data,
 * Shift extends; Enter and Tab walk down and right (inside a selection they cycle in it); F2 or
 * typing edits; Delete clears; Ctrl+A selects all; Ctrl+Z and Ctrl+Y are the delegated undo and
 * redo. Find, fill, insert, delete and zoom are commands of the registry (ext/tables, ext/app),
 * run by the one key listener before this. Returns true when the key was handled. Copy, cut and
 * paste are left to the browser, which turns them into clipboard events.
 */
export function gridKey(ctrl: SheetController, e: KeyboardEvent, h: KeyHelpers): boolean {
  const mod = e.ctrlKey || e.metaKey;
  const key = e.key;
  const arrows: Record<string, [number, number]> = {
    ArrowUp: [-1, 0],
    ArrowDown: [1, 0],
    ArrowLeft: [0, -1],
    ArrowRight: [0, 1],
  };
  const a = arrows[key];
  if (a) {
    if (e.altKey) {
      if (key !== "ArrowDown" || mod || e.shiftKey) return false;
      h.openFilter();
      return true;
    }
    if (mod) ctrl.edge(a[0], a[1], e.shiftKey);
    else ctrl.move(a[0], a[1], e.shiftKey);
    return true;
  }
  // Only the chords of the table take a modifier: any other is left to whoever binds it.
  if (e.altKey) return false;
  if (mod && !["Home", "End", " "].includes(key) && key.length !== 1) return false;
  switch (key) {
    case "Home":
      if (mod) ctrl.select(0, 0, e.shiftKey);
      else ctrl.select(ctrl.active.r, 0, e.shiftKey);
      return true;
    case "End":
      if (mod) ctrl.select(ctrl.rowCount - 1, ctrl.colCount - 1, e.shiftKey);
      else ctrl.select(ctrl.active.r, ctrl.colCount - 1, e.shiftKey);
      return true;
    case "PageDown":
      ctrl.move(h.pageRows, 0, e.shiftKey);
      return true;
    case "PageUp":
      ctrl.move(-h.pageRows, 0, e.shiftKey);
      return true;
    case "Enter":
      ctrl.walk(e.shiftKey ? -1 : 1, 0);
      return true;
    case "Tab": {
      const p = ctrl.primary;
      const single = p.r0 === p.r1 && p.c0 === p.c1;
      const atEnd = ctrl.active.c >= ctrl.colCount - 1 && ctrl.active.r >= ctrl.rowCount - 1;
      const atStart = ctrl.active.c === 0 && ctrl.active.r === 0;
      // Leave the grid from its last or first cell, so the keyboard is never trapped.
      if (single && ((!e.shiftKey && atEnd) || (e.shiftKey && atStart))) return false;
      ctrl.walk(0, e.shiftKey ? -1 : 1);
      return true;
    }
    case "F2":
      if (e.shiftKey) return false; // Shift+F2 is a cell's comment (spec §8.2, N47)
      ctrl.startEdit();
      return true;
    case "Escape":
      if (ctrl.pending) ctrl.cancelPaste();
      else ctrl.select(ctrl.active.r, ctrl.active.c);
      return true;
    case "Delete":
    case "Backspace":
      ctrl.clear();
      return true;
    case "ContextMenu":
      h.openContext();
      return true;
    case "F10":
      if (e.shiftKey) {
        h.openContext();
        return true;
      }
      return false;
    case " ": {
      const p = ctrl.primary;
      if (mod && e.shiftKey) return false;
      if (mod) ctrl.selectColumns(p.c0, p.c1);
      else if (e.shiftKey) ctrl.selectRows(p.r0, p.r1);
      else {
        ctrl.startEdit(" ");
      }
      return true;
    }
    default:
  }
  if (mod) {
    switch (key.toLowerCase()) {
      case "a":
        if (e.shiftKey) return false;
        ctrl.selectAll();
        return true;
      case "z":
        if (e.shiftKey) ctrl.redo();
        else ctrl.undo();
        return true;
      case "y":
        if (e.shiftKey) return false;
        ctrl.redo();
        return true;
      default:
        return false;
    }
  }
  // Typing replaces the cell's contents and starts editing.
  if (key.length === 1 && !e.altKey) {
    ctrl.startEdit(key);
    return true;
  }
  return false;
}
