import { Repo } from "@automerge/automerge-repo";
import { completionKeymap } from "@codemirror/autocomplete";
import { defaultKeymap, historyKeymap } from "@codemirror/commands";
import { type KeyBinding, keymap } from "@codemirror/view";
import { describe, expect, it } from "vitest";
import { createEditor } from "../editor/setup";
import { gridKey } from "../ui/sheet/keys";
import { unusedMasks } from "./bindings";
import { formatChord, isFunctionKey, parseChord, SYSTEMS, type System } from "./keys";
import { CODEMIRROR_ROWS, GRID_KEYS, OWNED_KEYS, WRITING_HELP_KEYS } from "./keytables";
import { builtinSources, globLoader } from "./loader";
import { buildRegistry } from "./registry";

/** A CodeMirror key name (`Shift-Mod-k`, `Alt-A`) as the registry writes it on a system. */
function fromCodeMirror(key: string, system: System): string {
  const parts = key.split(/-(?!$)/);
  let name = parts.pop() as string;
  const on = { ctrl: false, alt: false, shift: false, meta: false };
  for (const p of parts) {
    if (p === "Mod") on[system === "mac" ? "meta" : "ctrl"] = true;
    else if (p === "Cmd" || p === "Meta") on.meta = true;
    else if (p === "Ctrl") on.ctrl = true;
    else if (p === "Alt") on.alt = true;
    else if (p === "Shift") on.shift = true;
    else throw new Error(`modifier ${p}`);
  }
  if (/^[A-Z]$/.test(name)) {
    on.shift = true;
    name = name.toLowerCase();
  }
  if (name === " ") name = "Space";
  return formatChord({ ...on, key: name });
}

/** A keymap's chords on a system, binding by binding, its `shift` variant included. */
const chordsOf = (b: KeyBinding, system: System): string[] => {
  const k = b[system] ?? b.key;
  if (!k) return [];
  return [fromCodeMirror(k, system), ...(b.shift ? [fromCodeMirror(`Shift-${k}`, system)] : [])];
};

describe("EX0b1 keys owned by others", () => {
  it("EX0b1 C-6 every key owned by the editor or a widget is in one table and resolved by name", () => {
    // CodeMirror's keymaps, recomputed from the installed packages, equal the table.
    const installed = { defaultKeymap, historyKeymap, completionKeymap };
    const computed = Object.entries(installed).flatMap(([map, km]) =>
      km.map((b) => [map, ...SYSTEMS.map((s) => chordsOf(b, s))]),
    );
    expect(CODEMIRROR_ROWS.map(([map, , linux, mac, win]) => [map, linux, mac, win])).toEqual(
      computed,
    );
    for (const k of OWNED_KEYS)
      for (const s of SYSTEMS) for (const c of k[s]) expect(() => parseChord(c), c).not.toThrow();

    // The registry's chords that meet one of them name it: Ctrl+I, Ctrl+M and Ctrl+U are taken knowingly.
    const r = buildRegistry(builtinSources(), globLoader());
    expect(r.refused).toEqual([]);
    const masks = Object.fromEntries(
      r.keybindings.filter((b) => b.masks.length).map((b) => [b.command, [...b.masks]]),
    );
    expect(masks).toEqual({
      "format.italic": ["selectParentSyntax"],
      "format.underline": ["undoSelection"],
      "format.formula": ["toggleTabFocusMode"],
      "tables.insert-cells": ["app.zoom-in"],
      "tables.delete-cells": ["app.zoom-out"],
    });
    // A chord meeting an owned key without naming it is refused, naming it; a mask hiding nothing too.
    const extra = (binding: Record<string, unknown>) => ({
      origin: "builtin" as const,
      folder: "extra",
      files: new Set<string>(),
      text: JSON.stringify({
        id: "extra",
        version: "1.0.0",
        contributes: {
          commands: [{ id: "extra.go", label: "Go", category: "edit" }],
          keybindings: [{ command: "extra.go", ...binding }],
        },
      }),
    });
    expect(
      buildRegistry(
        [...builtinSources(), extra({ linux: ["Ctrl+Shift+k"], scope: "editor" })],
        globLoader(),
      ).bindings("linux").refused,
    ).toEqual([
      {
        folder: "extra",
        reason: "meets deleteLine on Ctrl+Shift+k (linux)",
        with: ["deleteLine", "extra.go"],
      },
    ]);
    expect(
      buildRegistry(
        [...builtinSources(), extra({ linux: ["Ctrl+Shift+k"], scope: "grid" })],
        globLoader(),
      ).bindings("linux").refused,
    ).toEqual([]);
    // A mask hiding nothing on any system: the static check of the manifests, here rather than at start.
    expect(unusedMasks(r.keybindings)).toEqual([]);
    const idle = buildRegistry(
      [
        ...builtinSources(),
        extra({ linux: ["Ctrl+Shift+l"], scope: "editor", masks: ["deleteLine"] }),
      ],
      globLoader(),
    );
    expect(unusedMasks(idle.keybindings)).toEqual([["extra.go", "deleteLine"]]);
    // The grid's own keys meet a binding of the grid's scope the same way.
    expect(
      buildRegistry(
        [...builtinSources(), extra({ linux: ["Alt+ArrowDown"], scope: "grid" })],
        globLoader(),
      ).bindings("linux").refused[0]?.with,
    ).toEqual(["grid.filter", "extra.go"]);

    // Census: an editor made by editor/setup.ts holds exactly CodeMirror's keymaps and the delegated undo.
    const repo = new Repo({ network: [] });
    for (const lang of ["plain", "typst"] as const) {
      const handle = repo.create<{ text: string }>({ text: "" });
      const parent = document.createElement("div");
      document.body.append(parent);
      let view: import("@codemirror/view").EditorView | undefined;
      const editor = createEditor(parent, handle, { id: "me", name: "Moi" }, undefined, {
        lang,
        onView: (v) => (view = v),
      });
      const bindings = (view?.state.facet(keymap) ?? []).flat();
      const expected = [
        ...defaultKeymap,
        ...historyKeymap,
        ...(lang === "typst" ? completionKeymap : []),
      ];
      const other = bindings.filter((b) => !expected.includes(b));
      expect(other.map((b) => b.key).sort(), lang).toEqual(
        lang === "typst" ? ["Escape", "Mod-z"] : ["Mod-z"],
      );
      expect(new Set(bindings.filter((b) => expected.includes(b))).size, lang).toBe(
        new Set(expected).size,
      );
      editor.destroy();
    }
    expect(WRITING_HELP_KEYS.map((k) => k.linux)).toEqual([["Escape"]]);

    // Census: the chords the grid's keyboard takes (modified, or a function key) are exactly its table.
    const ctrl = new Proxy(
      {
        active: { r: 1, c: 1 },
        primary: { r0: 1, r1: 1, c0: 1, c1: 1 },
        rowCount: 9,
        colCount: 9,
        pending: undefined,
      } as Record<string, unknown>,
      { get: (o, name) => (name in o ? o[name as string] : () => {}) },
    );
    const keys = [
      ..."abcdefghijklmnopqrstuvwxyz0123456789`-=[]\\;',./",
      ...["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "Home", "End", "PageUp", "PageDown"],
      ...["Enter", "Tab", "Escape", "Delete", "Backspace", "ContextMenu", " "],
      ...Array.from({ length: 12 }, (_, i) => `F${i + 1}`),
    ];
    const taken = new Set<string>();
    for (const k of keys)
      for (const [ctrlKey, altKey, shiftKey] of [
        [false, false, false],
        [false, false, true],
        [true, false, false],
        [true, false, true],
        [false, true, false],
        [false, true, true],
        [true, true, false],
      ] as const) {
        const e = new KeyboardEvent("keydown", {
          key: shiftKey && /^[a-z]$/.test(k) ? k.toUpperCase() : k,
          ctrlKey,
          altKey,
          shiftKey,
        });
        const name = k === " " ? "Space" : k;
        if (!(ctrlKey || altKey || isFunctionKey(name))) continue;
        if (gridKey(ctrl as never, e, { pageRows: 5, openFilter: () => {}, openContext: () => {} }))
          taken.add(
            formatChord({ ctrl: ctrlKey, alt: altKey, shift: shiftKey, meta: false, key: name }),
          );
      }
    // Alt+Enter is the cell editor's (Grid.svelte), not the grid's keyboard.
    const table = GRID_KEYS.filter((g) => g.name !== "grid.newline-in-cell")
      .flatMap((g) => g.linux)
      .filter((c) => {
        const p = parseChord(c);
        return p.ctrl || p.alt || isFunctionKey(p.key);
      });
    expect([...taken].sort()).toEqual([...new Set(table)].sort());
  });
});
