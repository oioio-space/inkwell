import { flushSync, tick } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { formatsOf } from "../../editor/format";
import { MARKDOWN, TYPST } from "../../editor/format/testkit";
import { registry } from "../../extensions";
import * as commands from "../../extensions/commands";
import { manySources } from "../../extensions/e2eMany";
import { setLive } from "../../extensions/live.svelte";
import { builtinSources, globLoader } from "../../extensions/loader";
import { ManifestError, type ManifestSource, parseManifest } from "../../extensions/manifest";
import { buildRegistry, type Registry } from "../../extensions/registry";
import { emptyContext, evalWhen, whenStats } from "../../extensions/when";
import { t, textIn } from "../../i18n";
import { cleanup, render } from "../testkit.svelte";
import CommandDialogs from "./CommandDialogs.svelte";
import { closeDialog, dialogShell, dialogs, openDialog } from "./dialogs.svelte";
import { paletteEntries } from "./entries";
import { fold } from "./fold";
import { classOf, prepare, query, rank, rankStats } from "./rank";
import { readRecents, recordRecent } from "./recent";

// Every run the palette makes goes through `run()`: a spy that still runs the command.
vi.mock("../../extensions/commands", async (original) => {
  const m = await original<typeof import("../../extensions/commands")>();
  return { ...m, run: vi.fn(m.run) };
});

const run = vi.mocked(commands.run);
const announced: string[] = [];
let removeKeys = () => {};

beforeEach(() => {
  // happy-dom has no modal dialog: the two methods the dialogs call.
  HTMLDialogElement.prototype.showModal = function showModal() {
    this.setAttribute("open", "");
  };
  HTMLDialogElement.prototype.close = function close() {
    this.removeAttribute("open");
  };
  localStorage.clear();
  announced.length = 0;
  commands.configure({ announce: (text) => announced.push(text), shell: dialogShell as never });
  commands.useRegistry(registry, { system: "linux" });
  setLive({ ...emptyContext(), formats: formatsOf(TYPST), fileType: "typst", canPresent: true });
  removeKeys = commands.installKeys();
  run.mockClear();
});
afterEach(() => {
  removeKeys();
  closeDialog();
  cleanup();
  hosted = false;
  vi.useRealTimers();
});

const source = (text: string, origin: ManifestSource["origin"] = "plugin"): ManifestSource => ({
  origin,
  folder: JSON.parse(text).id,
  text,
  files: new Set(),
});
/** A data-only extension of another origin with these commands. */
const plugin = (id: string, commands: { id: string; label: string; keywords?: string[] }[]) =>
  source(
    JSON.stringify({
      id,
      version: "1.0.0",
      contributes: { commands: commands.map((c) => ({ category: "app", ...c })) },
    }),
  );
const withPlugins = (...sources: ManifestSource[]): Registry =>
  buildRegistry([...builtinSources(), ...sources], globLoader());

/** A focusable element inside a registered surface. */
function surfaceChild(kind: "editor" | "grid", handlers: Record<string, () => unknown> = {}) {
  const host = document.createElement("div");
  const child = document.createElement("div");
  child.tabIndex = 0;
  host.append(child);
  document.body.append(host);
  commands.registerSurface(host, kind, handlers, {
    ...(kind === "editor"
      ? { viewer: registry.essentialViewer("text") }
      : { viewer: registry.viewers.find((v) => v.id === "tables.grid") as never }),
  });
  return child;
}

/** Opens the palette from `opener` (the focused element by default) and mounts it. */
let hosted = false;
async function openPalette(opener?: Element | null) {
  if (!hosted) render(CommandDialogs, {});
  hosted = true;
  await openDialog("palette", opener ?? document.activeElement);
  flushSync();
  const field = document.querySelector<HTMLInputElement>('dialog [role="combobox"]');
  if (!field) throw new Error("no palette");
  return field;
}

function type(field: HTMLInputElement, text: string) {
  field.value = text;
  field.dispatchEvent(new Event("input", { bubbles: true }));
  flushSync();
}

function key(el: Element, init: KeyboardEventInit & { keyCode?: number; isComposing?: boolean }) {
  const e = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init });
  for (const f of ["keyCode", "isComposing"] as const)
    if (init[f] !== undefined) Object.defineProperty(e, f, { value: init[f] });
  el.dispatchEvent(e);
  flushSync();
  return e;
}

const option = (id: string) => document.getElementById(`palette-${id}`);
const options = () => [...document.querySelectorAll<HTMLElement>('[role="option"]')];

/** The class of one label (and its keywords) for a query. */
function cls(label: string, q: string, keywords: string[] = []) {
  const p = prepare(
    [{ label, keywords }],
    (x) => x.label,
    (x) => x.keywords,
    "fr",
  );
  return classOf(p.entries[0] as (typeof p.entries)[number], query(q));
}

describe("EX0b2 command palette", () => {
  it("EX0b2 P-1 text is folded once: accents, case, ligatures and spaces", async () => {
    for (const s of ["Écrire", "ÉCRIRE", "ecrire"]) expect(fold(s)).toBe("ecrire");
    expect(fold("cœur")).toBe(fold("coeur"));
    expect(fold("Æ")).toBe("ae");
    expect(fold("Straße")).toBe("strasse");
    expect(fold("Rechercher une commande…")).toBe("rechercher une commande...");
    expect(fold("« Gras »")).toBe("gras");
    expect(fold("« Gras »")).toBe("gras");
    expect(fold("l’aperçu").split(" ")).toEqual(["l", "apercu"]);
    expect(fold("l'aperçu").split(" ")).toEqual(["l", "apercu"]);
    expect(fold("mise-en-page").split(" ")).toEqual(["mise", "en", "page"]);
    expect(fold("a/b").split(" ")).toEqual(["a", "b"]);
    expect(fold("x:y").split(" ")).toEqual(["x", "y"]);
    expect(fold("(z)").split(" ")).toEqual(["z"]);

    // A registry's labels and keywords are folded once per language, never per query or keystroke.
    const r = withPlugins();
    const texts = r.commands
      .filter((c) => c.id !== "app.palette")
      .reduce((n, c) => n + 1 + c.keywords.length, 0);
    rankStats.folds = 0;
    const fr = paletteEntries(r, "fr");
    expect(rankStats.folds).toBe(texts);
    expect(paletteEntries(r, "fr")).toBe(fr);
    for (let i = 0; i < 100; i++)
      rank(fr, query(`gras${i}`), { available: () => true, recency: () => -1 });
    expect(rankStats.folds).toBe(texts);
    commands.useRegistry(r, { system: "linux" });
    const field = await openPalette();
    for (let i = 0; i < 100; i++) type(field, "pan gau".slice(0, (i % 7) + 1));
    expect(rankStats.folds).toBe(texts);
    paletteEntries(r, "en");
    expect(rankStats.folds).toBe(2 * texts);
  });

  it("EX0b2 P-2 a query matches a label or a synonym by its start, its word starts or as one piece, never scattered", () => {
    const rows: [label: string, q: string, expected: number, keywords?: string[]][] = [
      ["Gras", "gras", 1],
      ["Gras", "GRAS", 1],
      ["Réduire", "reduire", 1],
      ["Mise en page", "mise-en-page", 1],
      ["Panneau de gauche", "pan", 2],
      ["Panneau de gauche", "panneau de", 2],
      ["Taille normale", "taille", 2],
      ["Panneau de gauche", "pan gau", 3],
      ["Panneau de gauche", "panneau gauche", 3],
      ["Panneau de gauche", "gau", 3],
      ["Afficher l’aperçu", "apercu", 3],
      ["Afficher l’aperçu", "l'aperçu", 3],
      ["Exporter en PDF", "pdf", 3],
      ["Mode gauche", "de gauche", 4],
      ["Panneau de gauche", "eau", 4],
      ["Exporter en PDF", "port", 4],
      ["Panneau de gauche", "gau pan", 0],
      ["Panneau de gauche", "gauche gauche", 0],
      ["Panneau de gauche", "pdg", 0],
      ["Gras", "gra s", 0],
      ["Gras", "", 0],
      ["Préférences", "réglages", 5, ["réglages paramètres"]],
      ["Préférences", "param", 5, ["réglages paramètres"]],
      ["Préférences", "glag", 6, ["réglages"]],
      ["Préférences", "xyz", 0, ["réglages"]],
    ];
    expect(rows.length).toBeGreaterThanOrEqual(24);
    for (const [label, q, expected, keywords] of rows)
      expect({ label, q, c: cls(label, q, keywords) }).toEqual({ label, q, c: expected });

    // A built-in's keyword is an i18n key read in the language; another extension's is literal.
    const r = withPlugins(plugin("lab", [{ id: "lab.go", label: "Aller", keywords: ["lorem"] }]));
    const entry = (language: "fr" | "en", id: string) =>
      paletteEntries(r, language).entries.find((e) => e.item.id === id) as never;
    expect(classOf(entry("fr", "app.preferences"), query("réglages"))).toBe(5);
    expect(classOf(entry("en", "app.preferences"), query("settings"))).toBe(5);
    expect(classOf(entry("en", "app.preferences"), query("réglages"))).toBe(0);
    expect(classOf(entry("fr", "lab.go"), query("lorem"))).toBe(5);
    expect(classOf(entry("en", "lab.go"), query("lorem"))).toBe(5);
    // A query is cut at 64 characters and 8 words.
    expect(query("a".repeat(65)).s.length).toBe(64);
    expect(query("a b c d e f g h i").words).toEqual(["a", "b", "c", "d", "e", "f", "g", "h"]);
  });

  it("EX0b2 P-3 available first, then match quality, then recency, then the label", () => {
    const items = ["Panneau", "Japan", "Panier", "Pâte", "Pantalon", "Ecran", "Écrire", "Effacer"];
    const p = prepare(
      items,
      (x) => x,
      () => [],
      "fr",
    );
    const order = (q: string, recents: string[], off: string[] = []) =>
      rank(p, query(q), {
        available: (x) => !off.includes(x),
        recency: (x) => recents.indexOf(x),
      });
    // A recent weaker match stays below a better one.
    expect(order("pan", ["Japan", "Panneau"])).toEqual(["Panneau", "Panier", "Pantalon", "Japan"]);
    // Two prefix matches: the recent one, then the others by the French collation.
    expect(order("pa", ["Pantalon"])).toEqual(["Pantalon", "Panier", "Panneau", "Pâte", "Japan"]);
    expect(order("e", []).slice(0, 3)).toEqual(["Ecran", "Écrire", "Effacer"]);
    // Without a query: recents in their order, then by label.
    expect(order("", ["Pâte", "Effacer"])).toEqual([
      "Pâte",
      "Effacer",
      "Ecran",
      "Écrire",
      "Japan",
      "Panier",
      "Panneau",
      "Pantalon",
    ]);
    // Every unavailable item below every available one, whatever its class or recency.
    expect(order("pan", ["Panneau"], ["Panneau"])).toEqual([
      "Panier",
      "Pantalon",
      "Japan",
      "Panneau",
    ]);
  });

  it("EX0b2 P-4 recent commands are this device's last palette runs, kept and read safely", () => {
    const all = () => true;
    recordRecent("A");
    recordRecent("B");
    recordRecent("A");
    expect(readRecents(all)).toEqual(["A", "B"]);
    // A key or a menu runs nothing into the recents.
    localStorage.clear();
    commands.run("app.about");
    document.body.dispatchEvent(
      new KeyboardEvent("keydown", { key: "s", ctrlKey: true, bubbles: true }),
    );
    expect(localStorage.getItem("inkwell.palette.recent")).toBeNull();
    for (let i = 0; i < 60; i++) recordRecent(`c${i}`);
    expect(readRecents(all)).toHaveLength(50);
    expect(readRecents(all)[0]).toBe("c59");
    expect(readRecents((id) => id !== "c59")[0]).toBe("c58");
    for (const bad of ["x", "{}", "[1,2]", '["a", 2]']) {
      localStorage.setItem("inkwell.palette.recent", bad);
      expect(readRecents(all), bad).toEqual([]);
    }
    const denied = () => {
      throw new Error("denied");
    };
    vi.stubGlobal("localStorage", { getItem: denied, setItem: denied });
    expect(() => recordRecent("A")).not.toThrow();
    expect(readRecents(all)).toEqual([]);
    vi.unstubAllGlobals();
  });

  it("EX0b2 P-5 an unavailable command is greyed with its reason and Enter only announces it", async () => {
    // Opened from a Typst editor, an editor-only command is available.
    const editor = surfaceChild("editor");
    editor.focus();
    whenStats.evaluated = 0;
    let field = await openPalette();
    const opening = whenStats.evaluated;
    expect(opening).toBeLessThanOrEqual(registry.commands.length);
    expect(option("typst.signature-help")?.getAttribute("aria-disabled")).toBeNull();
    for (let i = 0; i < 20; i++) type(field, "abcdefghijklmnopqrst".slice(0, i + 1));
    expect(whenStats.evaluated).toBe(opening);
    // A surface-only command the editor does not declare says why.
    type(field, "rechercher");
    expect(option("app.find")?.getAttribute("aria-disabled")).toBe("true");
    const findWhy = option("app.find")?.getAttribute("aria-describedby") ?? "";
    expect(document.getElementById(findWhy)?.textContent).toBe(t("cmd.find.unavailable"));
    closeDialog();
    flushSync();

    // From a grid, find is available.
    const grid = surfaceChild("grid", { "app.find": () => true });
    grid.focus();
    field = await openPalette();
    type(field, "rechercher");
    expect(option("app.find")?.getAttribute("aria-disabled")).toBeNull();
    closeDialog();
    flushSync();

    // From a Markdown file, presenting is greyed; Enter and a click say why and run nothing.
    setLive({ fileType: "markdown", formats: formatsOf(MARKDOWN), canPresent: false });
    surfaceChild("editor").focus();
    field = await openPalette();
    type(field, "présenter");
    const row = option("typst.present") as HTMLElement;
    expect(row.getAttribute("aria-disabled")).toBe("true");
    const why = document.getElementById(row.getAttribute("aria-describedby") ?? "");
    expect(why?.textContent).toBe(t("cmd.present.unavailable"));
    expect(field.getAttribute("aria-activedescendant")).toBe(row.id);
    run.mockClear();
    key(field, { key: "Enter" });
    row.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    flushSync();
    expect(announced).toEqual([t("cmd.present.unavailable"), t("cmd.present.unavailable")]);
    expect(run).not.toHaveBeenCalled();
    expect(document.querySelector("dialog[open]")).not.toBeNull();
    // Said again each time: the status is emptied, then filled on the next tick.
    const status = document.querySelector("dialog [role=status]") as HTMLElement;
    key(field, { key: "Enter" });
    expect(status.textContent).toBe("");
    await tick();
    flushSync();
    expect(status.textContent).toBe(t("cmd.present.unavailable"));

    // Every built-in that can be refused has its reason.
    for (const c of registry.commands) {
      if (c.when !== undefined || c.entry === undefined)
        expect(c.unavailable, `${c.id} has a reason`).toBeDefined();
    }
    const back = registry.command("app.return");
    expect(evalWhen(back?.when, { ...emptyContext(), canReturn: false })).toBe(false);
    expect(evalWhen(back?.when, { ...emptyContext(), canReturn: true })).toBe(true);
    const vector = Object.values(
      import.meta.glob(
        "../../extensions/testdata/manifests/invalid/missing-key.command-when-unavailable.json",
        { query: "?raw", import: "default", eager: true },
      ),
    )[0] as string;
    expect(() => parseManifest(source(vector, "builtin"))).toThrow(ManifestError);
  });

  it("EX0b2 P-6 the palette is an APG combobox with its count announced and at most 50 rows", async () => {
    vi.useFakeTimers();
    const many = Array.from({ length: 120 }, (_, i) => ({
      id: `lab.c${i}`,
      label: `Zeta ${String(i).padStart(3, "0")}`,
    }));
    commands.useRegistry(withPlugins(plugin("lab", many)), { system: "linux" });
    const field = await openPalette(document.body);
    type(field, "zeta");
    const dialog = document.querySelector("dialog") as HTMLDialogElement;
    expect(field.getAttribute("aria-expanded")).toBe("true");
    expect(field.getAttribute("aria-autocomplete")).toBe("list");
    const list = document.getElementById(field.getAttribute("aria-controls") ?? "");
    expect(list?.getAttribute("role")).toBe("listbox");
    const selected = () => document.querySelector('[role="option"][aria-selected="true"]');
    const follows = () => expect(field.getAttribute("aria-activedescendant")).toBe(selected()?.id);
    expect(options().length).toBeLessThanOrEqual(50);
    follows();
    key(field, { key: "ArrowDown" });
    expect(selected()?.id).toBe("palette-lab.c1");
    follows();
    key(field, { key: "ArrowUp" });
    expect(selected()?.id).toBe("palette-lab.c0");
    key(field, { key: "PageDown" });
    expect(selected()?.id).toBe("palette-lab.c10");
    follows();
    key(field, { key: "PageUp" });
    expect(selected()?.id).toBe("palette-lab.c0");
    // The active row is scrolled into view as the keys move it.
    const scrolled: { id: string; arg: unknown }[] = [];
    Element.prototype.scrollIntoView = function scrollIntoView(arg?: unknown) {
      scrolled.push({ id: this.id, arg });
    };
    for (let i = 0; i < 15; i++) key(field, { key: "ArrowDown" });
    expect(scrolled.at(-1)).toEqual({ id: "palette-lab.c15", arg: { block: "nearest" } });
    key(field, { key: "PageDown" });
    expect(scrolled.at(-1)?.id).toBe("palette-lab.c25");
    for (let i = 0; i < 15; i++) key(field, { key: "PageDown" });
    expect(selected()?.id).toBe("palette-lab.c119");
    expect(scrolled.at(-1)?.id).toBe("palette-lab.c119");
    expect(option("lab.c119")).not.toBeNull();
    expect(options().length).toBeLessThanOrEqual(50);
    follows();

    // The count, inside the dialog, said once the typing stops.
    const status = dialog.querySelector('[role="status"]') as HTMLElement;
    expect(dialog.contains(status)).toBe(true);
    vi.advanceTimersByTime(300);
    flushSync();
    expect(status.textContent).toBe("120 commandes");
    let changes = 0;
    const seen = new MutationObserver(() => changes++);
    seen.observe(status, { childList: true, characterData: true, subtree: true });
    for (const q of ["z", "ze", "zeta", "zeta 00", "zeta 007"]) {
      type(field, q);
      vi.advanceTimersByTime(100);
    }
    flushSync();
    expect(status.textContent).toBe("120 commandes");
    vi.advanceTimersByTime(200);
    flushSync();
    await Promise.resolve();
    expect(status.textContent).toBe("1 commande");
    expect(changes).toBe(1);
    type(field, "zzz");
    vi.advanceTimersByTime(300);
    flushSync();
    expect(status.textContent).toBe("Aucune commande");
    expect(textIn("en", "palette.none")).toBe("No command");
    expect(textIn("en", "palette.count.one")).toBe("{count} command");
    expect(textIn("en", "palette.count.other")).toBe("{count} commands");
  });

  it("EX0b2 P-7 Enter gives the focus back to where it was, then runs there; Escape runs nothing", async () => {
    for (const c of registry.commands) await registry.loadCommand(c);
    const editor = surfaceChild("editor");
    editor.focus();
    let field = await openPalette();
    // The palette's own command is not listed.
    expect(option("app.palette")).toBeNull();
    expect(options().length).toBeGreaterThan(0);
    // A row keeps the focus in the field.
    const press = new MouseEvent("mousedown", { bubbles: true, cancelable: true });
    options()[0]?.dispatchEvent(press);
    expect(press.defaultPrevented).toBe(true);
    // A registry chord does nothing behind the dialog.
    run.mockClear();
    key(field, { key: "s", keyCode: 83, ctrlKey: true });
    key(field, { key: "b", keyCode: 66, ctrlKey: true });
    expect(run).not.toHaveBeenCalled();
    // Enter: the focus is back in the editor when the command runs there.
    const seen: { id: string; target: unknown; focus: Element | null }[] = [];
    run.mockImplementationOnce((id, target) => {
      seen.push({ id, target, focus: document.activeElement });
      return true;
    });
    type(field, "à propos");
    key(field, { key: "Enter" });
    expect(seen).toEqual([{ id: "app.about", target: editor, focus: editor }]);
    expect(dialogs.open).toBeUndefined();
    expect(readRecents(() => true)).toEqual(["app.about"]);

    // From a main-menu item, gone with its menu: the target is the last focused surface.
    editor.focus();
    const item = document.createElement("button");
    document.body.append(item);
    item.focus();
    item.remove();
    field = await openPalette(item);
    run.mockImplementationOnce((id, target) => {
      seen.push({ id, target, focus: document.activeElement });
      return true;
    });
    type(field, "à propos");
    key(field, { key: "Enter" });
    expect(seen.at(-1)).toEqual({ id: "app.about", target: editor, focus: editor });

    // Enter and Escape that end a composition belong to the input method, not to the palette.
    editor.focus();
    field = await openPalette();
    type(field, "à propos");
    run.mockClear();
    key(field, { key: "Enter", isComposing: true });
    key(field, { key: "Escape", keyCode: 229 });
    key(field, { key: "Enter", keyCode: 229 });
    expect(run).not.toHaveBeenCalled();
    expect(dialogs.open).toBe("palette");
    closeDialog();
    flushSync();

    // Escape gives the focus back and runs nothing.
    editor.focus();
    field = await openPalette();
    run.mockClear();
    key(field, { key: "Escape" });
    expect(run).not.toHaveBeenCalled();
    expect(document.activeElement).toBe(editor);
    expect(dialogs.open).toBeUndefined();

    // Its own command while open: the field again, its text selected; the other one replaces it.
    editor.focus();
    field = await openPalette();
    type(field, "abc");
    editor.focus();
    key(field, { key: "P", keyCode: 80, ctrlKey: true, shiftKey: true });
    expect(run).toHaveBeenLastCalledWith("app.palette");
    await vi.waitFor(() => expect(document.activeElement).toBe(field));
    expect([field.selectionStart, field.selectionEnd]).toEqual([0, 3]);
    key(field, { key: "?", keyCode: 191, ctrlKey: true, shiftKey: true });
    expect(run).toHaveBeenLastCalledWith("app.shortcuts");
    await vi.waitFor(() => {
      flushSync();
      expect(document.querySelector('[data-testid="shortcuts-window"]')).not.toBeNull();
    });
    expect(document.querySelector('[role="combobox"]')).toBeNull();
    expect(dialogs.target).toBe(editor);
  });

  it("EX0b2 P-8 ranking 1 000 commands stays within its counted bound", async () => {
    // 1 000 seeded labels, a tenth of them long runs of « a ».
    let seed = 11;
    const next = () => {
      seed = (Math.imul(seed, 1103515245) + 12345) >>> 0;
      return seed >>> 16;
    };
    const WORDS = ["panneau", "de", "la", "gauche", "page", "mise", "en", "forme", "texte", "a"];
    const items = Array.from({ length: 1000 }, (_, i) => ({
      label:
        i % 10 === 0
          ? "a".repeat(150 + (next() % 100))
          : Array.from({ length: 2 + (next() % 4) }, () => WORDS[next() % WORDS.length]).join(" "),
      keywords: [WORDS[next() % WORDS.length] as string],
    }));
    rankStats.folds = 0;
    const p = prepare(
      items,
      (x) => x.label,
      (x) => x.keywords,
      "fr",
    );
    expect(rankStats.folds).toBe(2000);
    const total = p.entries.reduce(
      (n, e) => n + e.label.s.length + e.keywords.reduce((k, w) => k + w.s.length, 0),
      0,
    );
    for (const text of ["a", "pan gau", "de la page", `${"a".repeat(63)}b`]) {
      Object.assign(rankStats, { comparisons: 0, selection: 0, folds: 0 });
      const q = query(text);
      const out = rank(p, q, { available: (x) => x.label.length % 3 > 0, recency: () => -1 });
      if (text.length < 64) expect(out.length, text).toBeGreaterThan(0);
      expect(rankStats.comparisons, text).toBeLessThanOrEqual(4 * q.words.length * total);
      expect(rankStats.selection, text).toBeLessThanOrEqual(12 * items.length);
      expect(rankStats.folds, text).toBe(0);
    }

    // In the palette, over the e2e build's 1 000 fixture commands: at most 50 rows, nothing folded.
    const r = buildRegistry([...builtinSources(), ...manySources()], globLoader());
    expect(r.commands.length).toBeGreaterThanOrEqual(1000);
    commands.useRegistry(r, { system: "linux" });
    const field = await openPalette(document.body);
    rankStats.folds = 0;
    for (const text of ["a", "pan gau", "de la page", `${"a".repeat(63)}b`]) {
      type(field, text);
      expect(options().length, text).toBeLessThanOrEqual(50);
    }
    expect(rankStats.folds).toBe(0);
  });

  it("PAL1 one opening over 1 000 commands builds the context once, not once per command", async () => {
    // Each context copies the live state; built per command, it made a warm opening O(commands x
    // keys) and failed PERF-PALETTE's 50 ms on GitHub's WebKit runner (run 45601671496).
    const r = buildRegistry([...builtinSources(), ...manySources()], globLoader());
    expect(r.commands.length).toBeGreaterThanOrEqual(1000);
    commands.useRegistry(r, { system: "linux" });
    const editor = surfaceChild("editor");
    editor.focus();
    for (let i = 0; i < 3; i++) {
      const before = commands.commandStats.contexts;
      await openPalette();
      expect(commands.commandStats.contexts - before, `opening ${i}`).toBe(1);
      closeDialog();
      flushSync();
      editor.focus();
    }
  });
});
