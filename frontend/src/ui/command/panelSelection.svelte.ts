/**
 * Which panel each side shows, and how the core opens a side on a panel (EX0c2): kept apart from
 * the bodies (panels.svelte.ts), so the stores that open a panel (the problems, the history) import
 * no panel code.
 */
import { commandRegistry } from "../../extensions/commands";
import { SIDES, type Side } from "../../extensions/manifest";
import type { Panel } from "../../extensions/registry";

/**
 * The panels the core opens by name: the problems list (a click on the counter) and the history
 * (a click on a coloured passage). Each is checked against the manifests by a test.
 */
export const PANEL = { problems: "app.problems", history: "app.history" } as const;

/** The panel each side shows; the drawer opens on the history, as it always did. */
export const selection: Record<Side, string | undefined> = $state({
  start: undefined,
  end: PANEL.history,
});

/** What opens a side (the shell's, App.svelte): the files' setting, the drawer's state. */
const openers: Partial<Record<Side, () => void>> = {};

/** The panel a side shows now: the one chosen while it shows, else the first shown. */
export function selectedOf(side: Side, shown: readonly Panel[]): Panel | undefined {
  const id = selection[side];
  return shown.find((p) => p.id === id) ?? shown[0];
}

/** Chooses a side's panel (a tab), without opening the side. */
export function selectPanel(id: string): void {
  const p = panelById(id);
  if (p) selection[p.side] = id;
}

/** Opens a panel's side on it (the shell's opener) and chooses it there (P-5). */
export function openPanel(id: string): void {
  const p = panelById(id);
  if (!p) return;
  selection[p.side] = id;
  openers[p.side]?.();
}

/** The shell says how each side opens. */
export function configurePanels(next: Partial<Record<Side, () => void>>): void {
  Object.assign(openers, next);
}

function panelById(id: string): Panel | undefined {
  const r = commandRegistry();
  for (const side of Object.keys(SIDES) as Side[]) {
    const p = r?.panels(side).find((x) => x.id === id);
    if (p) return p;
  }
  return undefined;
}
