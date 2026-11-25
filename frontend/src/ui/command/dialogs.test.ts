import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { activateStartup, registry } from "../../extensions";
import {
  configure,
  installKeys,
  menuOf,
  run,
  type Shell,
  useRegistry,
} from "../../extensions/commands";
import { SYSTEMS } from "../../extensions/keys";
import { setLive } from "../../extensions/live.svelte";
import { builtinSources, globLoader } from "../../extensions/loader";
import { buildRegistry } from "../../extensions/registry";
import { emptyContext } from "../../extensions/when";
import { t } from "../../i18n";
import { cleanup, render } from "../testkit.svelte";
import CommandDialogs from "./CommandDialogs.svelte";
import { closeDialog, dialogShell, loaders } from "./dialogs.svelte";

// Counted before anything of this file opens a dialog: each loader runs once, on first use.
const palette = vi.spyOn(loaders, "palette");
const shortcuts = vi.spyOn(loaders, "shortcuts");
let removeKeys = () => {};

beforeEach(() => {
  HTMLDialogElement.prototype.showModal = function showModal() {
    this.setAttribute("open", "");
  };
  HTMLDialogElement.prototype.close = function close() {
    this.removeAttribute("open");
  };
  useRegistry(registry, { system: "linux" });
  setLive({ ...emptyContext() });
  removeKeys = installKeys();
});
afterEach(() => {
  removeKeys();
  closeDialog();
  cleanup();
});

const mounted = (selector: string) =>
  vi.waitFor(() => {
    flushSync();
    const el = document.querySelector(selector);
    if (!el) throw new Error(`${selector} not mounted`);
    return el;
  });

describe("EX0b2 entry points", () => {
  it("EX0b2 P-10 the palette and the shortcuts window open from their commands, menus and chords, loaded on first use", async () => {
    // The chords of the plan on each system, the first one shown.
    const plan = {
      "app.palette": { linux: "Ctrl+Shift+p", mac: "Shift+Meta+p", win: "Ctrl+Shift+p" },
      "app.shortcuts": { linux: "Ctrl+?", mac: "Alt+Meta+/", win: "Ctrl+?" },
    };
    for (const system of SYSTEMS)
      for (const [id, chords] of Object.entries(plan)) {
        const active = registry.bindings(system).active.filter((a) => a.command === id);
        expect(
          active.map((a) => [a.chord, a.scope]),
          `${id} ${system}`,
        ).toEqual([[chords[system], "window"]]);
      }
    // The main menu's items, with their labels.
    const items = registry.menu("main");
    expect(items.find((m) => m.command === "app.palette")).toMatchObject({
      group: "project",
      order: 90,
    });
    expect(items.find((m) => m.command === "app.shortcuts")).toMatchObject({
      group: "app",
      order: 15,
    });
    expect(t("cmd.palette")).toBe("Rechercher une commande…");
    expect(t("cmd.shortcuts")).toBe("Raccourcis clavier");
    const shown = menuOf("main").flatMap((g) => g.items.map((c) => c.id));
    expect(shown).toContain("app.palette");
    expect(shown).toContain("app.shortcuts");

    // Building the registry and the start load neither component.
    buildRegistry(builtinSources(), globLoader());
    activateStartup();
    expect(palette).not.toHaveBeenCalled();
    expect(shortcuts).not.toHaveBeenCalled();

    // Each command opens its own dialog; two runs load each component once.
    for (const c of registry.commands) await registry.loadCommand(c);
    configure({ shell: dialogShell as unknown as Shell });
    render(CommandDialogs, {});
    for (let i = 0; i < 2; i++) {
      run("app.shortcuts");
      await mounted('[data-testid="shortcuts-window"]');
      expect(document.querySelector('[role="combobox"]')).toBeNull();
      closeDialog();
      flushSync();
      run("app.palette");
      await mounted('dialog [role="combobox"]');
      expect(document.querySelector('[data-testid="shortcuts-window"]')).toBeNull();
      closeDialog();
      flushSync();
    }
    expect(palette).toHaveBeenCalledTimes(1);
    expect(shortcuts).toHaveBeenCalledTimes(1);
  });
});
