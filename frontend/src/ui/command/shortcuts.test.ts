import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { registry } from "../../extensions";
import { installKeys, useRegistry } from "../../extensions/commands";
import { renderChord, SYSTEMS } from "../../extensions/keys";
import { OWNED_KEYS } from "../../extensions/keytables";
import { builtinSources, globLoader } from "../../extensions/loader";
import { CATEGORIES } from "../../extensions/manifest";
import { buildRegistry } from "../../extensions/registry";
import { t } from "../../i18n";
import { cleanup, render } from "../testkit.svelte";
import CommandDialogs from "./CommandDialogs.svelte";
import { closeDialog, openDialog } from "./dialogs.svelte";
import { shortcutSections } from "./shortcuts";

let removeKeys = () => {};
beforeEach(() => {
  HTMLDialogElement.prototype.showModal = function showModal() {
    this.setAttribute("open", "");
  };
  HTMLDialogElement.prototype.close = function close() {
    this.removeAttribute("open");
  };
  removeKeys = installKeys();
});
afterEach(() => {
  removeKeys();
  closeDialog();
  cleanup();
});

const BARE = new Set([
  "ArrowUp",
  "ArrowDown",
  "ArrowLeft",
  "ArrowRight",
  "Home",
  "End",
  "PageUp",
  "PageDown",
]);

describe("EX0b2 shortcuts window", () => {
  it("EX0b2 P-9 the shortcuts window lists every chord in force on this system, hers marked", async () => {
    for (const system of SYSTEMS) {
      useRegistry(registry, { system });
      const sections = shortcutSections();
      const commands = sections.filter((s) => s.id !== "write");
      // Sections in the categories' order, then « Écrire » last.
      expect(sections.map((s) => s.id)).toEqual([
        ...CATEGORIES.filter((c) => commands.some((s) => s.id === c)),
        "write",
      ]);
      // Each command row holds exactly its chords in force, each once, as the system shows them.
      const want = new Map<string, Set<string>>();
      for (const a of registry.bindings(system).active)
        want.set(
          a.command,
          (want.get(a.command) ?? new Set()).add(renderChord(a.chord, system, "fr")),
        );
      const rows = commands.flatMap((s) =>
        s.rows.map((r) => {
          expect(registry.command(r.id)?.category, r.id).toBe(s.id);
          return r;
        }),
      );
      expect(rows.map((r) => r.id).sort(), system).toEqual([...want.keys()].sort());
      for (const r of rows) {
        expect(new Set(r.chords).size, r.id).toBe(r.chords.length);
        expect(new Set(r.chords), `${system} ${r.id}`).toEqual(want.get(r.id));
      }
      // A widget's chord is the editor's.
      expect(rows.find((r) => r.id === "app.undo")?.marks).toEqual([t("shortcuts.editor")]);
      expect(rows.find((r) => r.id === "app.save")?.marks).toEqual([]);
      // « Écrire »: exactly the labelled keys of keytables.ts, never a bare cursor move, marked.
      const write = sections.at(-1);
      const labelled = OWNED_KEYS.filter((k) => k.label !== undefined && k[system].length > 0);
      expect(write?.rows.map((r) => r.id).sort()).toEqual(
        [...new Set(labelled.map((k) => k.label as string))].sort(),
      );
      for (const r of write?.rows ?? []) {
        expect(r.marks, r.id).toEqual([t("shortcuts.editor")]);
        const raw = labelled.filter((k) => k.label === r.id).flatMap((k) => k[system]);
        expect(new Set(r.chords), r.id).toEqual(
          new Set(raw.map((c) => renderChord(c, system, "fr"))),
        );
        for (const c of raw) expect(BARE.has(c), `${r.id} ${c}`).toBe(false);
      }
    }

    // Her own binding is marked, the default it took absent; `null` removes a command's row.
    useRegistry(registry, {
      system: "linux",
      user: [
        { command: "app.about", key: "Ctrl+," },
        { command: "app.save", key: null },
      ],
    });
    let rows = shortcutSections().flatMap((s) => s.rows);
    expect(rows.find((r) => r.id === "app.about")).toMatchObject({
      chords: ["Ctrl+,"],
      marks: [t("shortcuts.mine")],
    });
    expect(rows.find((r) => r.id === "app.preferences")).toBeUndefined();
    expect(rows.find((r) => r.id === "app.save")).toBeUndefined();

    // A chord the system handles is marked so.
    const lab = {
      origin: "builtin" as const,
      folder: "lab",
      files: new Set<string>(),
      text: JSON.stringify({
        id: "lab",
        version: "1.0.0",
        contributes: {
          commands: [{ id: "lab.help", label: "menu.about", category: "app" }],
          keybindings: [
            {
              command: "lab.help",
              linux: ["F1"],
              mac: ["Meta+?"],
              win: ["F1"],
              scope: "window",
              delegated: "system",
            },
          ],
        },
      }),
    };
    const withLab = buildRegistry([...builtinSources(), lab], globLoader());
    expect(withLab.refused).toEqual([]);
    useRegistry(withLab, { system: "linux" });
    rows = shortcutSections().flatMap((s) => s.rows);
    expect(rows.find((r) => r.id === "lab.help")).toMatchObject({
      chords: ["F1"],
      marks: [t("shortcuts.system")],
    });

    // The search folds and matches as the palette does: « zoom » leaves the zoom rows.
    useRegistry(registry, { system: "linux" });
    render(CommandDialogs, {});
    await openDialog("shortcuts", document.body);
    flushSync();
    const field = document.querySelector<HTMLInputElement>(
      '[data-testid="shortcuts-window"] input',
    );
    if (!field) throw new Error("no shortcuts window");
    // What the search leaves is said once the typing stops, inside the dialog.
    const status = document.querySelector<HTMLElement>(
      '[data-testid="shortcuts-window"] [role=status]',
    );
    expect(status).not.toBeNull();
    vi.useFakeTimers();
    field.value = "ZOOM";
    field.dispatchEvent(new Event("input", { bubbles: true }));
    flushSync();
    const before = status?.textContent;
    vi.advanceTimersByTime(299);
    flushSync();
    expect(status?.textContent).toBe(before);
    vi.advanceTimersByTime(1);
    flushSync();
    expect(status?.textContent).toBe("3 raccourcis");
    field.value = "zzz";
    field.dispatchEvent(new Event("input", { bubbles: true }));
    flushSync();
    vi.advanceTimersByTime(300);
    flushSync();
    expect(status?.textContent).toBe(t("shortcuts.none"));
    vi.useRealTimers();
    field.value = "ZOOM";
    field.dispatchEvent(new Event("input", { bubbles: true }));
    flushSync();
    expect(
      [...document.querySelectorAll<HTMLElement>("[data-shortcut]")]
        .map((r) => r.dataset.shortcut)
        .sort(),
    ).toEqual(["app.zoom-in", "app.zoom-out", "app.zoom-reset"]);
  });
});
