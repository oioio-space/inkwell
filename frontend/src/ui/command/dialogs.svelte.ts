/**
 * The palette and the shortcuts window (docs/plans/2026-11-25-ex0b2-palette.md §1): one of them
 * open at a time, each loaded on its first use and once (P-10), both acting on the target the
 * person was in when she opened them (P-5, P-7).
 */
import type { Component } from "svelte";
import { onEscape, returnTarget } from "../../extensions/commands";

export type CommandDialog = "palette" | "shortcuts";

/** What both dialogs are mounted with. */
export interface DialogProps {
  /** Where the commands act and the focus goes back. */
  readonly target: Element | undefined;
  /** Bumped when the dialog's own command runs while it is open: it focuses its field again. */
  readonly again: number;
  /** When the opening was asked (`performance.now()`), for PERF-PALETTE. */
  readonly start: number;
  readonly onclose: () => void;
}

/** The dynamic imports, one chunk each (criterion `chunks`); an object so tests can count them. */
export const loaders: Record<CommandDialog, () => Promise<{ default: Component<DialogProps> }>> = {
  palette: () => import("./Palette.svelte"),
  shortcuts: () => import("./ShortcutsWindow.svelte"),
};

const loading = new Map<CommandDialog, Promise<Component<DialogProps>>>();

export const dialogs = $state<{
  open?: CommandDialog | undefined;
  component?: Component<DialogProps> | undefined;
  target?: Element | undefined;
  again: number;
  start: number;
}>({ again: 0, start: 0 });

/**
 * Where a dialog opened from `opener` acts: the element that had the focus, or, when it left the
 * page (a menu's item, gone with its menu) or is a menu's, the last focused surface.
 */
export function targetOf(opener: Element | null | undefined): Element | undefined {
  const usable =
    opener instanceof Element &&
    opener.isConnected &&
    opener !== document.body &&
    opener.closest('[role="menu"]') === null;
  return usable ? opener : returnTarget();
}

/** The latest opening asked: a load that finishes after a newer request, or after Escape, is dropped. */
let request = 0;

/** Opens a dialog, or, when it is the open one, gives its field the focus again. */
export async function openDialog(
  which: CommandDialog,
  opener: Element | null = document.activeElement,
): Promise<void> {
  const start = performance.now();
  if (dialogs.open === which) {
    dialogs.again++;
    return;
  }
  // The other one is replaced: its target stays, never the field that had the focus in it.
  const target = dialogs.open === undefined ? targetOf(opener) : dialogs.target;
  let load = loading.get(which);
  if (!load) {
    load = loaders[which]().then((m) => m.default);
    loading.set(which, load);
    load.catch(() => loading.delete(which));
  }
  const ticket = ++request;
  // Escape while the chunk loads cancels the opening: the dialog never shows after it.
  const stop = onEscape(() => {
    if (ticket === request) request++;
  });
  let component: Component<DialogProps>;
  try {
    component = await load;
  } finally {
    stop();
  }
  if (ticket !== request) return;
  Object.assign(dialogs, { open: which, component, target, start });
}

/** The shell's part (`Shell.palette`, `Shell.shortcuts`): each opens on the focused element. */
export const dialogShell = {
  palette: (): void => void openDialog("palette"),
  shortcuts: (): void => void openDialog("shortcuts"),
};

export function closeDialog(): void {
  request++;
  Object.assign(dialogs, { open: undefined, component: undefined, target: undefined });
}
