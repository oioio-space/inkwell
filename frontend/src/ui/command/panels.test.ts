import { flushSync, mount, unmount } from "svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import { registry } from "../../extensions";
import { panelsOf, useRegistry } from "../../extensions/commands";
import { setLive } from "../../extensions/live.svelte";
import { builtinSources, globLoader } from "../../extensions/loader";
import {
  ManifestError,
  type ManifestSource,
  parseManifest,
  SIDES,
} from "../../extensions/manifest";
import { builtinPanelEntry } from "../../extensions/panelEntries";
import { buildRegistry } from "../../extensions/registry";
import { emptyContext } from "../../extensions/when";
import { problems } from "../../language/problems.svelte";
import { history } from "../history/history.svelte";
import { notice } from "../notice.svelte";
import { cleanup, press, render } from "../testkit.svelte";
import PanelSide from "./PanelSide.svelte";
import { configurePanels, openPanel, PANEL, selectedOf, selection } from "./panelSelection.svelte";
import { bodyOf, loadSide, prefetchSide, sideReady } from "./panels.svelte";

const INVALID = import.meta.glob("../../extensions/testdata/manifests/invalid/*.json", {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;
const VALID = import.meta.glob("../../extensions/testdata/manifests/valid/*panel*.json", {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;
const REFUSED = import.meta.glob("../../extensions/testdata/manifests/refused/*panel*.json", {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;

/** A built-in manifest's source; its panels' entry is a file of its folder. */
const source = (text: string, origin: ManifestSource["origin"] = "builtin", folder = "sample") => ({
  origin,
  folder,
  text,
  files: new Set(["native/panels.ts"]),
});
function verdict(s: ManifestSource): string {
  try {
    parseManifest(s);
    return "accepted";
  } catch (error) {
    if (error instanceof ManifestError) return error.reason;
    throw error;
  }
}
/** An essential built-in with these panels. */
const withPanels = (id: string, panels: object[]) =>
  JSON.stringify({
    id,
    version: "1.0.0",
    essential: true,
    contributes: { panels },
  });
const panel = (id: string, side: string, order: number, extra: object = {}) => ({
  id,
  label: id,
  side,
  order,
  icon: "quote",
  entry: "native/panels.ts",
  capabilities: ["ui.native"],
  ...extra,
});
const name = (path: string) => path.slice(path.lastIndexOf("/") + 1);
const tabs = (host: ParentNode) => [...host.querySelectorAll<HTMLElement>('[role="tab"]')];

afterEach(() => {
  cleanup();
  useRegistry(registry, { system: "linux" });
  setLive({ ...emptyContext() });
  history.open = false;
  selection.end = PANEL.history;
  vi.restoreAllMocks();
});

describe("EX0c2 the panels point and its sides", () => {
  it("EX0c2 P-1 a malformed panel is refused with its reason", () => {
    const words = ["unknown-side", "plugin-panel-entry", "panel-no-label"];
    const vectors = Object.entries(INVALID).filter(([p]) => words.some((w) => p.includes(w)));
    expect(vectors.map(([p]) => words.find((w) => p.includes(w))).sort()).toEqual(
      [...words].sort(),
    );
    for (const [path, text] of vectors) {
      const n = name(path);
      const origin = n.includes("plugin-") ? "plugin" : "builtin";
      expect({ n, reason: verdict(source(text, origin)) }).toEqual({
        n,
        reason: n.slice(0, n.indexOf(".")),
      });
    }
    // A valid vector: both sides, a `when`, read unchanged.
    const [valid] = Object.values(VALID);
    const parsed = parseManifest(source(valid as string));
    expect(parsed.contributes.panels.map((p) => [p.id, p.side, p.order])).toEqual([
      ["sample.notes", "end", 50],
      ["sample.outline", "start", 20],
    ]);
    expect(Object.keys(SIDES)).toEqual(["start", "end"]);
    // A panel id is a claim: the same one twice refuses the manifest, named.
    const [dup] = Object.values(REFUSED);
    const r = buildRegistry([...builtinSources(), source(dup as string)], globLoader());
    expect(r.refused).toContainEqual({
      folder: "sample",
      reason: "conflict on panel sample.notes",
      with: ["sample", "sample"],
    });
    expect(r.panels("end").some((p) => p.extension === "sample")).toBe(false);
    // Another extension's panel id, or a panel whose capability is not the native one, is refused.
    expect(verdict(source(withPanels("x", [panel("y.p", "end", 1)])))).toBe("not-own");
    expect(verdict(source(withPanels("x", [panel("x.p", "end", 1, { capabilities: [] })])))).toBe(
      "missing-key",
    );
  });

  it("EX0c2 P-2 a side keeps the core's panels first, then order, extension and id; one panel draws no tab list", () => {
    const a = source(
      withPanels("zed", [panel("zed.b", "end", 25), panel("zed.a", "end", 25)]),
      "builtin",
      "zed",
    );
    const b = source(withPanels("ann", [panel("ann.go", "end", 25)]), "builtin", "ann");
    const one = buildRegistry([...builtinSources(), a, b], globLoader());
    const two = buildRegistry([b, a, ...builtinSources()], globLoader());
    for (const r of [one, two])
      expect(r.panels("end").map((p) => p.id)).toEqual([
        "app.problems",
        "app.history",
        "ann.go",
        "zed.a",
        "zed.b",
        "app.comments",
        "app.activity",
      ]);
    expect(one.panels("start").map((p) => p.id)).toEqual(["app.files"]);
    // The left side has one panel: its body alone, in an aside named by the side, no tab list.
    const host = render(PanelSide, { side: "start" });
    const aside = host.querySelector("aside");
    expect(aside?.getAttribute("aria-label")).toBe("Fichiers du projet");
    expect(host.querySelector('[role="tablist"]')).toBeNull();
    expect(host.querySelector("nav")?.getAttribute("aria-label")).toBe("Liste des fichiers");
  });

  it("EX0c2 P-3 a side's tab list follows the WAI-ARIA tabs pattern", async () => {
    await loadSide("end");
    history.open = true;
    const host = render(PanelSide, { side: "end" });
    const list = host.querySelector('[role="tablist"]') as HTMLElement;
    expect(list.getAttribute("aria-label")).toBe("Relire");
    expect(tabs(host).map((t) => t.textContent?.trim())).toEqual([
      "Problèmes",
      "Historique",
      "Commentaires",
      "Activité",
    ]);
    // Every tab controls a mounted panel labelled by it; only the selected one shows.
    for (const tab of tabs(host)) {
      const panelEl = document.getElementById(tab.getAttribute("aria-controls") ?? "");
      expect(panelEl?.getAttribute("role")).toBe("tabpanel");
      expect(panelEl?.getAttribute("aria-labelledby")).toBe(tab.id);
      expect(panelEl?.hidden).toBe(tab.getAttribute("aria-selected") !== "true");
    }
    const selected = () => tabs(host).find((t) => t.getAttribute("aria-selected") === "true");
    expect(selected()?.textContent?.trim()).toBe("Historique");
    // Roving focus: the selected tab alone is in the tab sequence.
    expect(tabs(host).map((t) => t.tabIndex)).toEqual([-1, 0, -1, -1]);
    // Arrows move and select at once (automatic activation), wrapping at the ends; Home, End.
    const at = (i: number) => tabs(host)[i] as HTMLElement;
    press(at(1), "ArrowLeft");
    expect(selected()).toBe(at(0));
    expect(document.activeElement).toBe(at(0));
    press(at(0), "ArrowLeft");
    expect(selected()).toBe(at(3));
    press(at(3), "ArrowRight");
    expect(selected()).toBe(at(0));
    press(at(0), "End");
    expect(selected()).toBe(at(3));
    press(at(3), "Home");
    expect(selected()).toBe(at(0));
    expect(tabs(host).map((t) => t.tabIndex)).toEqual([0, -1, -1, -1]);
    // A panel whose content takes no focus is itself in the tab sequence (comments, activity).
    press(at(0), "End");
    const comments = document.getElementById(at(2).getAttribute("aria-controls") ?? "");
    expect(comments?.getAttribute("tabindex")).toBe("0");
  });

  it("EX0c2 P-4 panel bodies load with their side: the start side at start, the end side at idle after the first paint, a panel shown only once loaded", async () => {
    // The left side's bodies are part of the start; the drawer's are not.
    expect(builtinPanelEntry("app", "native/panels.ts")).toBeDefined();
    expect(builtinPanelEntry("app", "native/drawer.ts")).toBeUndefined();
    let loads = 0;
    const loader = (folder: string, entry: string) => {
      loads++;
      return globLoader()(folder, entry);
    };
    const r = buildRegistry(builtinSources(), loader);
    useRegistry(r, { system: "linux" });
    const files = r.panels("start")[0];
    expect(files && bodyOf(files)).toBeDefined();
    render(PanelSide, { side: "start" });
    expect(loads).toBe(0);
    // The drawer's code comes at idle (a timer where the browser has no idle callback), all at once.
    const shownEditor = document.createElement("div");
    shownEditor.className = "cm-content";
    document.body.append(shownEditor);
    const fake = {
      document,
      requestIdleCallback: undefined,
      setTimeout: vi.fn((f: () => void) => f()),
    };
    const { prefetchSide } = await import("./panels.svelte");
    prefetchSide("end", fake as unknown as Window);
    expect(fake.setTimeout).toHaveBeenCalledOnce();
    await loadSide("end");
    expect(sideReady("end")).toBe(true);
    // A side is drawn only once its bodies are in: no empty panel shows.
    for (const p of r.panels("end")) expect(bodyOf(p), p.id).toBeDefined();
    // A tab's `when` hides it, and a hidden chosen panel falls back to the first one shown.
    const s = source(
      withPanels("smp", [panel("smp.a", "end", 1, { when: "text" })]),
      "builtin",
      "smp",
    );
    const w = buildRegistry([...builtinSources(), s], globLoader());
    useRegistry(w, { system: "linux" });
    setLive({ text: true });
    expect(panelsOf("end").map((p) => p.id)).toContain("smp.a");
    selection.end = "smp.a";
    expect(selectedOf("end", panelsOf("end"))?.id).toBe("smp.a");
    setLive({ text: false });
    expect(panelsOf("end").map((p) => p.id)).not.toContain("smp.a");
    expect(selectedOf("end", panelsOf("end"))?.id).toBe("app.problems");
  });

  it("EX0c2 P-5 opening a side keeps the focus, closing it gives the focus back, openPanel selects", async () => {
    await loadSide("end");
    const editor = document.createElement("div");
    editor.className = "cm-content";
    editor.tabIndex = 0;
    document.body.append(editor);
    editor.focus();
    const target = document.createElement("div");
    document.body.append(target);
    history.open = true;
    const side = mount(PanelSide, { target, props: { side: "end" } });
    flushSync();
    // Opening leaves the focus where it was.
    expect(document.activeElement).toBe(editor);
    // Closing while a tab holds the focus gives it back, never to the page.
    const tab = target.querySelector<HTMLElement>('[role="tab"]');
    tab?.focus();
    tab?.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
    expect(document.activeElement).toBe(tab);
    unmount(side);
    flushSync();
    expect(document.activeElement).toBe(editor);
    // The core opens a side by a panel's name: the counter's Shift+click, a passage's history.
    const opened: string[] = [];
    configurePanels({ end: () => opened.push("end") });
    expect(registry.panels("end").map((p) => p.id)).toEqual(
      expect.arrayContaining(Object.values(PANEL)),
    );
    problems.show();
    expect(selection.end).toBe(PANEL.problems);
    expect(opened).toEqual(["end"]);
    openPanel(PANEL.history);
    expect(selection.end).toBe(PANEL.history);
    configurePanels({ end: () => (history.open = true) });
  });

  it("EX0c2 P-11 panels cost nothing at start: indexed once at build, a closed side imports nothing", () => {
    let loads = 0;
    const r = buildRegistry(builtinSources(), (folder, entry) => {
      loads++;
      return globLoader()(folder, entry);
    });
    expect(loads).toBe(0);
    for (const side of ["start", "end"] as const) {
      expect(r.panels(side)).toBe(r.panels(side));
      expect(Object.isFrozen(r.panels(side))).toBe(true);
    }
    useRegistry(r, { system: "linux" });
    // Asking what a side shows, and drawing the open left side, load no entry.
    expect(panelsOf("end")).toHaveLength(4);
    render(PanelSide, { side: "start" });
    expect(loads).toBe(0);
    // The panels' code: one start entry (eager) and one drawer entry, both ext/app's.
    const entries = new Set(
      [...r.panels("start"), ...r.panels("end")].map((p) => `${p.folder}/${p.entry}`),
    );
    expect([...entries].sort()).toEqual(["app/native/drawer.ts", "app/native/panels.ts"]);
  });

  it("EX0c2 review: a panel that fails to load counts as done, says so, and is tried again at the next opening", async () => {
    const shown: string[] = [];
    vi.spyOn(notice, "show").mockImplementation((text) => void shown.push(text));
    let fails = true;
    let tries = 0;
    const s = source(
      withPanels("smp", [panel("smp.broken", "end", 99, { label: "history.tab.comments" })]),
      "builtin",
      "smp",
    );
    const r = buildRegistry([...builtinSources(), s], (folder, entry) => {
      if (folder !== "smp") return globLoader()(folder, entry);
      tries++;
      return fails ? Promise.reject(new Error("gone")) : Promise.resolve({ panels: {} });
    });
    useRegistry(r, { system: "linux" });
    await loadSide("end");
    // The side opens: the failed panel is counted as done and says it, its tab still there.
    expect(sideReady("end")).toBe(true);
    expect(shown).toEqual([expect.stringContaining("smp.broken")]);
    history.open = true;
    const host = render(PanelSide, { side: "end" });
    selection.end = "smp.broken";
    flushSync();
    expect(host.querySelector('[role="tabpanel"]:not([hidden])')?.textContent).toMatch(/./);
    // Its entry is not kept: the next opening tries it again.
    fails = false;
    await loadSide("end");
    expect(tries).toBe(2);
  });

  it("EX0c2 review: the drawer's code is fetched only once the editor shows (after PERF-START's mark)", () => {
    const fake = {
      document,
      requestIdleCallback: undefined,
      setTimeout: vi.fn((f: () => void) => f()),
    };
    document.querySelector(".cm-content")?.remove();
    prefetchSide("end", fake as unknown as Window);
    expect(fake.setTimeout).not.toHaveBeenCalled();
    const content = document.createElement("div");
    content.className = "cm-content";
    document.body.append(content);
    return new Promise<void>((done) =>
      queueMicrotask(() => {
        expect(fake.setTimeout).toHaveBeenCalledOnce();
        done();
      }),
    );
  });
});
