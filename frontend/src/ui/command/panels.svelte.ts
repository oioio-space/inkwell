/**
 * The sides' bodies (EX0c2, plan §1), loaded from the panels' entries (`panels[id]`); which panel
 * a side shows is panelSelection.svelte.ts's. The left side's bodies are
 * part of the start (`builtinPanelEntry`); the drawer's are fetched together, at idle after the
 * first paint or at its first opening if that comes sooner, and the drawer opens only once they
 * are in: no frame shows an empty panel.
 */
import type { Component } from "svelte";
import { commandRegistry, panelsOf } from "../../extensions/commands";
import type { Side } from "../../extensions/manifest";
import { builtinPanelEntry } from "../../extensions/panelEntries";
import type { Panel, Registry } from "../../extensions/registry";
import { t } from "../../i18n";
import { notice } from "../notice.svelte";

const bodies: Record<string, Component> = $state({});
/** The panels whose entry failed this time: done for the side, shown as such, tried at the next opening. */
const failed: Record<string, true> = $state({});
/** One load of a side's bodies at a time, per registry. */
const loading = new WeakMap<Registry, Map<Side, Promise<void>>>();

const bodyIn = (m: unknown, id: string): Component | undefined =>
  (m as { panels?: Record<string, Component> } | undefined)?.panels?.[id];

/** A panel's body if it is there: a built-in start-side body always is (it is part of the start). */
export function bodyOf(p: Panel): Component | undefined {
  if (p.origin === "builtin") {
    const eager = bodyIn(builtinPanelEntry(p.folder, p.entry), p.id);
    if (eager) return eager;
  }
  return bodies[p.id];
}

/** Whether a panel's entry failed to load: its tab shows that instead of a body. */
export const panelFailed = (p: Panel): boolean => failed[p.id] === true;

/** Whether every panel of a side has its body or failed, so the side can show without an empty frame. */
export const sideReady = (side: Side): boolean =>
  panelsOf(side).every((p) => bodyOf(p) || panelFailed(p));

/** Loads every body of a side, once; the side's entries are asked for together. */
export function loadSide(side: Side): Promise<void> {
  const r = commandRegistry();
  if (!r) return Promise.resolve();
  const sides = loading.get(r) ?? new Map<Side, Promise<void>>();
  loading.set(r, sides);
  let p = sides.get(side);
  if (!p) {
    let again = false;
    p = Promise.all(
      r.panels(side).map(async (panel) => {
        if (bodyOf(panel)) return;
        let c: Component | undefined;
        try {
          c = bodyIn(await r.loadPanel(panel), panel.id);
        } catch (error) {
          console.warn(`panel ${panel.id}:`, error);
        }
        if (c) {
          bodies[panel.id] = c;
          delete failed[panel.id];
          return;
        }
        // Done for this opening, said, and asked again at the next one (the registry keeps no failure).
        failed[panel.id] = true;
        again = true;
        notice.show(t("panel.failed", { name: panel.id }), "alert");
      }),
    ).then(() => {
      if (again) sides.delete(side);
    });
    sides.set(side, p);
  }
  return p;
}

/**
 * The drawer's code at idle once the editor shows (after PERF-START's mark, `.cm-content`), so its
 * first opening waits on nothing and the start pays nothing for it.
 */
export function prefetchSide(side: Side, win: Window = window): void {
  const go = () => void loadSide(side);
  const idle = () => {
    if (typeof win.requestIdleCallback === "function")
      win.requestIdleCallback(go, { timeout: 4000 });
    else win.setTimeout(go, 1000);
  };
  const doc = win.document;
  if (doc.querySelector(".cm-content")) {
    idle();
    return;
  }
  const seen = new MutationObserver(() => {
    if (!doc.querySelector(".cm-content")) return;
    seen.disconnect();
    idle();
  });
  seen.observe(doc.body ?? doc.documentElement, { childList: true, subtree: true });
}
