import { flushSync } from "svelte";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { formatsOf } from "../../editor/format";
import { MARKDOWN, PLAIN, type TestLang, TYPST } from "../../editor/format/testkit";
import Zoom from "../../ext/app/native/status/Zoom.svelte";
import { registry } from "../../extensions";
import {
  availability,
  configure,
  locationShows,
  menuOf,
  type Shell,
  statusOf,
  useRegistry,
} from "../../extensions/commands";
import { setLive } from "../../extensions/live.svelte";
import { builtinSources, globLoader } from "../../extensions/loader";
import {
  LOCATIONS,
  ManifestError,
  type ManifestSource,
  type Origin,
  parseManifest,
  STATUS,
} from "../../extensions/manifest";
import { buildRegistry, type Registry, type StatusItem } from "../../extensions/registry";
import { emptyContext } from "../../extensions/when";
import { type MessageKey, textIn } from "../../i18n";
import Preview from "../../preview/Preview.svelte";
import { preview } from "../../preview/preview.svelte";
import { result } from "../../preview/testkit";
import { previewZoom } from "../../preview/zoom.svelte";
import { workspace } from "../../workspace/workspace.svelte";
import HeaderBar from "../shell/HeaderBar.svelte";
import StatusBar from "../shell/StatusBar.svelte";
import { cleanup, click, fakeEditor, fakeWidth, render } from "../testkit.svelte";
import { visibleCount } from "../toolbar/layout";
import FormatBar from "./FormatBar.svelte";
import { EXTENSION_ITEMS, type StatusBarProps, statusLayout } from "./status.svelte";

const VECTORS = import.meta.glob("../../extensions/testdata/manifests/invalid/*.json", {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;

/** A manifest's source; a status item's entry is a file of its folder (`native/status.ts`). */
const source = (text: string, origin: Origin = "builtin", folder = "sample"): ManifestSource => ({
  origin,
  folder,
  text,
  files: new Set([
    ...(text.includes('"status"') ? ["native/status.ts"] : []),
    ...(text.includes('"panels"') ? ["native/panels.ts"] : []),
  ]),
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
/** A manifest of commands `<id>.a`, `<id>.b`… shown as `extra` says. */
const manifest = (
  id: string,
  extra: Record<string, unknown> = {},
  names = ["go"],
  essential = false,
) =>
  JSON.stringify({
    id,
    version: "1.0.0",
    essential,
    contributes: {
      commands: names.map((n) => ({ id: `${id}.${n}`, label: n, category: "edit" })),
      ...extra,
    },
  });
const item = (command: string, location: string, group: string, order: number, when?: string) => ({
  command,
  location,
  group,
  order,
  ...(when ? { when } : {}),
});
const ids = (r: Registry, loc: string) => r.menu(loc).map((x) => x.command);
const builtins = (extra: ManifestSource[] = []) =>
  buildRegistry([...builtinSources(), ...extra], globLoader());

afterEach(() => {
  cleanup();
  useRegistry(registry, { system: "linux" });
  setLive({ ...emptyContext() });
  vi.restoreAllMocks();
});

/** What the window says about main.typ, a note or a picture, as App.svelte sets it. */
const typstMain = {
  fileType: "typst",
  formats: formatsOf(TYPST),
  main: true,
  text: true,
  previewOpen: true,
  canPresent: true,
};

const bar = (over: Partial<StatusBarProps> = {}) => ({
  lang: TYPST,
  status: "alone" as const,
  peerCount: 0,
  editorState: fakeEditor("a‸").state,
  info: undefined,
  problemsNav: true,
  ...over,
});

describe("EX0c1 locations and the status point", () => {
  it("EX0c1 L-1 a malformed location, group or status item is refused with its reason", () => {
    const names = Object.keys(VECTORS);
    for (const word of [
      "unknown-location",
      "foreign-group",
      "protected-group",
      "plugin-status-entry",
      "reserved-location",
    ])
      expect(
        names.some((n) => n.includes(word)),
        word,
      ).toBe(true);
    for (const [path, text] of Object.entries(VECTORS)) {
      const name = path.slice(path.lastIndexOf("/") + 1);
      const origin: Origin = name.includes("plugin-") ? "plugin" : "builtin";
      expect({ name, reason: verdict(source(text, origin)) }).toEqual({
        name,
        reason: name.slice(0, name.indexOf(".")),
      });
    }
    // A group of another location, a side's group on the other side.
    expect(verdict(source(manifest("x", { menus: [item("x.go", "format", "panels", 1)] })))).toBe(
      "bad-value",
    );
    const status = (side: string, group: string, extra: Record<string, unknown> = {}) =>
      manifest(
        "x",
        {
          status: [
            {
              id: "x.s",
              label: "S",
              side,
              group,
              order: 1,
              entry: "native/status.ts",
              capabilities: ["ui.native"],
              ...extra,
            },
          ],
        },
        ["go"],
        true,
      );
    expect(verdict(source(status("end", "caret")))).toBe("accepted");
    expect(verdict(source(status("start", "caret")))).toBe("bad-value");
    expect(verdict(source(status("middle", "caret")))).toBe("bad-value");
    expect(verdict(source(status("end", "caret", { capabilities: [] })))).toBe("missing-key");
    // A built-in that is not essential cannot run code in the bar, nor hold a protected group.
    expect(
      verdict(source(status("end", "caret").replace('"essential":true', '"essential":false'))),
    ).toBe("native-refused");
    expect(
      verdict(source(status("end", "save").replace('"essential":true', '"essential":false'))),
    ).toBe("protected");
    // Another origin's item in a protected group or a reserved location is kept out and listed.
    const r = builtins([
      source(
        manifest("plug", {
          menus: [
            item("plug.go", "header/end", "view", 1),
            item("plug.go", "editor/context", "extensions", 1),
            item("plug.go", "format", "extensions", 1),
          ],
        }),
        "plugin",
        "plug",
      ),
    ]);
    expect(r.refused).toEqual([
      { folder: "plug", reason: "protected group view", with: ["plug.go"] },
      { folder: "plug", reason: "reserved location editor/context", with: ["plug.go"] },
    ]);
    expect(ids(r, "format")).toContain("plug.go");
    expect(ids(r, "header/end")).not.toContain("plug.go");
    // Every location of the plan is in the closed table, the reserved ones with their block; EX0c2
    // hosts the grid's, the view bar's, the history's and the differences'.
    expect(Object.keys(LOCATIONS)).toHaveLength(19);
    const reserved = (loc: string) =>
      (LOCATIONS as Record<string, { reserved?: string }>)[loc]?.reserved;
    for (const loc of [
      "grid/toolbar",
      "grid/context",
      "view",
      "history/toolbar",
      "diff/toolbar",
      "insert",
    ])
      expect(reserved(loc), loc).toBeUndefined();
    expect(reserved("editor/context")).toBe("UX1-D");
  });

  it("EX0c1 L-2 a location keeps the core's groups at their fixed places, then order, extension and command", () => {
    const m = (id: string, names: string[], items: object[]) =>
      source(manifest(id, { menus: items }, names), "builtin", id);
    // Built in either order: the same place for every item.
    const sources = [
      m(
        "zed",
        ["b", "a"],
        [item("zed.b", "format", "extensions", 5), item("zed.a", "format", "extensions", 5)],
      ),
      m("ann", ["go"], [item("ann.go", "format", "extensions", 5)]),
      m("bob", ["go"], [item("bob.go", "format", "marks", 999)]),
    ];
    const one = buildRegistry([...builtinSources(), ...sources], globLoader());
    const two = buildRegistry([...[...sources].reverse(), ...builtinSources()], globLoader());
    for (const r of [one, two]) {
      const items = ids(r, "format");
      expect(items.slice(-3)).toEqual(["ann.go", "zed.a", "zed.b"]);
      // A marks item stays in the marks group, after the core's marks, before the inline group.
      expect(items.indexOf("bob.go")).toBe(items.indexOf("format.strike") + 1);
      expect(items.indexOf("bob.go")).toBeLessThan(items.indexOf("format.link"));
    }
    expect(ids(one, "format")).toEqual(ids(two, "format"));
    // The main menu's groups keep their fixed order, extensions before the application's.
    const groups = LOCATIONS.main.groups.map((g) => g.id);
    expect(groups).toEqual([
      "view",
      "insert",
      "export",
      "project",
      "document",
      "extensions",
      "app",
    ]);
    expect(STATUS.end.groups.map((g) => g.id)).toEqual([
      "problems",
      "preview",
      "save",
      "caret",
      "file",
      "type",
      "extensions",
    ]);
  });

  it("EX0c1 L-3 an item's when hides it and its command's when greys it, both read from the live context", () => {
    useRegistry(registry, { system: "linux" });
    setLive({ ...typstMain });
    const shown = () => menuOf("header/end").flatMap((g) => g.items.map((c) => c.id));
    expect(shown()).toContain("typst.present");
    setLive({ fileType: "markdown" });
    expect(shown()).not.toContain("typst.present");
    setLive({ fileType: "typst", canPresent: false });
    expect(shown()).not.toContain("typst.present");
    // The command's own `when` greys it where it shows (the main menu), with its reason.
    expect(availability("typst.present")).toEqual({
      ok: false,
      reason: expect.stringMatching(/./),
    });
    // `text` and `main` decide the caret, the words, the language and the page.
    setLive({ ...typstMain });
    const status = () => statusOf().map((x) => x.id);
    expect(status()).toEqual(expect.arrayContaining(["app.position", "typst.page"]));
    setLive({ text: false, main: false });
    expect(status()).not.toContain("app.position");
    expect(status()).not.toContain("app.words");
    expect(status()).not.toContain("app.language");
    expect(status()).not.toContain("typst.page");
    expect(status()).toContain("app.zoom");
  });

  it("EX0c1 L-4 what does not fit goes to the overflow menu in the same order, and nothing is lost", () => {
    useRegistry(registry, { system: "linux" });
    const items = menuOf("format", { formats: formatsOf(TYPST) }).flatMap((g, group) =>
      g.items.map((c) => ({ id: c.id, group })),
    );
    for (let width = 0; width <= 1400; width += 20) {
      const n = visibleCount(
        items.map((i) => i.group),
        width,
      );
      expect(n).toBeGreaterThanOrEqual(0);
      expect(n).toBeLessThanOrEqual(items.length);
    }
    // Drawn at 536 px: the bar and its ⋯ menu hold every item, once, in order.
    vi.spyOn(Object.getPrototypeOf(workspace), "rightsKnown", "get").mockReturnValue(true);
    const restore = fakeWidth(536);
    try {
      const editor = fakeEditor("a‸");
      const host = render(FormatBar, {
        lang: TYPST,
        editorState: editor.state,
        target: undefined,
        run: editor.run,
        files: [],
        currentFile: "main.typ",
        readFile: async () => undefined,
      });
      const onBar = [...host.querySelectorAll<HTMLElement>("button[data-bar]")].map(
        (b) => b.dataset.cmd,
      );
      click(host.querySelector('button[aria-label="Plus d\'actions"]') as HTMLElement);
      const inMenu = [
        ...document.querySelectorAll<HTMLElement>('[role^="menuitem"][data-bar]'),
      ].map((b) => b.dataset.cmd);
      expect(inMenu.length).toBeGreaterThan(0);
      expect([...onBar, ...inMenu]).toEqual(items.map((i) => i.id));
    } finally {
      restore();
    }
    // The extensions' status items: 3 in the bar, the rest in ⋯; all of them in a narrow bar.
    const ext = (n: number): StatusItem =>
      ({ id: `x.s${n}`, side: "end", group: "extensions", order: n }) as StatusItem;
    const core = registry.status;
    const many = [...core, ...[1, 2, 3, 4, 5].map(ext)];
    const wide = statusLayout(many, false);
    expect(wide.more.map((x) => x.id)).toEqual(["x.s4", "x.s5"]);
    expect(wide.end.filter((x) => x.group === "extensions")).toHaveLength(EXTENSION_ITEMS);
    const narrow = statusLayout(many, true);
    expect(narrow.more.map((x) => x.id)).toEqual(["x.s1", "x.s2", "x.s3", "x.s4", "x.s5"]);
    // The core's items are never put away; their groups say what a narrow bar hides.
    expect([...narrow.start, ...narrow.end]).toEqual(core);
    const rule = Object.fromEntries(
      STATUS.end.groups.map((g) => [g.id, "narrow" in g ? g.narrow : undefined]),
    );
    expect(rule).toEqual({
      problems: "text",
      preview: "hide",
      save: undefined,
      caret: undefined,
      file: undefined,
      type: "hide",
      extensions: "more",
    });
  });

  it("EX0c1 L-5 the header bar is drawn from its two locations around the core's fixed widgets", () => {
    useRegistry(registry, { system: "linux" });
    setLive({ ...typstMain, sidebarOpen: true });
    const props = {
      title: "Inkwell",
      subtitle: "main.typ",
      status: "alone",
      me: { id: "me", name: "Moi" },
      peers: [],
    };
    const header = render(HeaderBar, props);
    const buttons = () =>
      [...header.querySelectorAll<HTMLButtonElement>("button:not([aria-haspopup])")].map(
        (b) => b.dataset.cmd,
      );
    expect(buttons()).toEqual([
      "app.sidebar",
      "app.preview",
      "typst.present",
      "app.people",
      "app.share",
      "app.review",
    ]);
    // The sync state sits after the start's buttons; the main menu closes the end.
    const start = header.querySelector(".start") as HTMLElement;
    expect(start.lastElementChild?.getAttribute("role")).toBe("status");
    const end = header.querySelector(".end") as HTMLElement;
    expect(
      end.querySelector(
        ":scope > :last-child [aria-haspopup='menu'], :scope > [aria-haspopup='menu']:last-child",
      ),
    ).not.toBeNull();
    // Each group's look: icons, and Share as the suggested action with its label.
    const share = header.querySelector('[data-cmd="app.share"]') as HTMLElement;
    expect(share.classList.contains("sug")).toBe(true);
    expect(share.textContent?.trim()).toBe("Partager");
    expect(header.querySelector('[data-cmd="app.review"] svg')).not.toBeNull();
    // Present is Typst's, on the main document.
    setLive({ fileType: "markdown" });
    flushSync();
    expect(buttons()).not.toContain("typst.present");
  });

  it("EX0c1 L-6 the formatting bar is the format location of the open file's formats, or no bar", () => {
    useRegistry(registry, { system: "linux" });
    expect(locationShows("format", { formats: formatsOf(TYPST) })).toBe(true);
    expect(locationShows("format", { formats: formatsOf(MARKDOWN) })).toBe(true);
    expect(locationShows("format", { formats: formatsOf(PLAIN) })).toBe(false);
    expect(locationShows("format", { formats: new Set() })).toBe(false);
    vi.spyOn(Object.getPrototypeOf(workspace), "rightsKnown", "get").mockReturnValue(true);
    const restore = fakeWidth(1400);
    try {
      const editor = fakeEditor("a‸");
      const mount = (lang: TestLang) =>
        render(FormatBar, {
          lang,
          editorState: editor.state,
          target: undefined,
          run: editor.run,
          files: [],
          currentFile: "main.typ",
          readFile: async () => undefined,
        });
      const md = mount(MARKDOWN);
      const mdBar = [...md.querySelectorAll<HTMLElement>("button[data-bar]")].map(
        (b) => b.dataset.bar,
      );
      expect(mdBar).not.toContain("underline");
      expect(mdBar).toContain("task");
      // The hint is the language's: Markdown's bold writes **texte**.
      expect(md.querySelector("#tip-bold")?.textContent).toContain("**texte**");
      cleanup();
      const typ = mount(TYPST);
      expect(typ.querySelector("#tip-bold")?.textContent).toContain("*texte*");
      expect(typ.querySelector("#tip-bold")?.textContent).not.toContain("**texte**");
      // The heading menu is the format/heading location, the level's syntax beside it.
      click(typ.querySelector('button[aria-haspopup="menu"]') as HTMLElement);
      const levels = [...document.querySelectorAll<HTMLElement>('[role="menuitemradio"]')];
      expect(levels.map((l) => l.dataset.cmd)).toEqual(
        ids(registry, "format/heading").filter((id) => id !== undefined),
      );
      expect(levels[2]?.textContent).toContain("==");
    } finally {
      restore();
    }
  });

  it("EX0c1 L-7 the status bar is drawn from the status point, by side and group, the core's first", () => {
    useRegistry(registry, { system: "linux" });
    setLive({ ...typstMain });
    preview.receive(result());
    const mount = () =>
      render(StatusBar, { ...bar(), ofMain: true, unsaved: true } as unknown as Record<
        string,
        unknown
      >);
    const host = mount();
    flushSync();
    const order = () =>
      [...host.querySelectorAll<HTMLElement>("footer > *")].map(
        (el) => el.dataset.statusItem ?? (el.classList.contains("sp") ? "|" : "?"),
      );
    expect(order()).toEqual([
      "app.presence",
      "|",
      "typst.problems",
      "typst.page",
      "app.unsaved",
      "app.position",
      "app.words",
      "app.zoom",
      "app.encoding",
      "app.language",
    ]);
    expect(host.querySelector('[data-testid="problems"]')).not.toBeNull();
    cleanup();
    // A note: no problems, no page.
    setLive({ ...typstMain, fileType: "markdown", main: false });
    const note = mount();
    flushSync();
    const items = [...note.querySelectorAll<HTMLElement>("[data-status-item]")].map(
      (el) => el.dataset.statusItem,
    );
    expect(items).not.toContain("typst.problems");
    expect(items).not.toContain("typst.page");
    expect(items[0]).toBe("app.presence");
  });

  it("EX0c1 L-8 the preview's tools and the zoom menu are locations, each zoom acting on its own surface", async () => {
    useRegistry(registry, { system: "linux" });
    for (const c of registry.commands) await registry.loadCommand(c);
    const calls: string[] = [];
    configure({
      shell: new Proxy({} as Shell, {
        get: (_, name) => (arg?: unknown) => calls.push(`${String(name)}(${arg ?? ""})`),
      }),
    });
    setLive({ ...typstMain });
    const host = render(Preview, { onSourceJump: () => {} });
    preview.receive(result());
    flushSync();
    const tools = [...host.querySelectorAll<HTMLElement>(".tools button")];
    expect(tools.map((b) => b.dataset.cmd ?? b.dataset.el)).toEqual([
      "typst.preview-zoom-out",
      "preview.fit",
      "typst.preview-zoom-in",
      "typst.jump-to-preview",
    ]);
    expect(tools.map((b) => b.getAttribute("aria-label"))).toEqual([
      "Réduire l'aperçu",
      expect.stringMatching(/^\d+ %, ajuster à la largeur$/),
      "Agrandir l'aperçu",
      "Montrer le curseur dans l'aperçu",
    ]);
    const before = previewZoom.current;
    click(tools[2] as HTMLElement);
    flushSync();
    expect(previewZoom.current).toBeGreaterThan(before);
    expect(calls).toEqual([]);
    click(tools[1] as HTMLElement);
    flushSync();
    expect(previewZoom.mode).toBe("fit");
    cleanup();
    // The status bar's zoom menu is the text's: the `status/zoom` location, through the shell.
    const zoomItem = registry.status.find((x) => x.id === "app.zoom");
    const zoom = render(Zoom, { item: zoomItem, bar: {} as StatusBarProps });
    click(zoom.querySelector('button[aria-haspopup="menu"]') as HTMLElement);
    const steps = [...document.querySelectorAll<HTMLElement>('[role="menu"] [data-cmd]')];
    expect(steps.map((s) => s.dataset.cmd)).toEqual(ids(registry, "status/zoom"));
    click(steps[1] as HTMLElement);
    expect(calls).toEqual(["zoom(in)"]);
  });

  it("EX0c1 L-9 locations cost nothing at start: indexed once at build, drawn without an extension call", () => {
    let loads = 0;
    const r = buildRegistry(builtinSources(), (folder, entry) => {
      loads++;
      return globLoader()(folder, entry);
    });
    expect(loads).toBe(0);
    for (const loc of Object.keys(LOCATIONS)) expect(r.menu(loc)).toBe(r.menu(loc));
    expect(r.menu("nowhere")).toBe(r.menu("elsewhere"));
    expect(Object.isFrozen(r.menu("format"))).toBe(true);
    // Drawing a location loads nothing: the header bar asks the registry only.
    useRegistry(r, { system: "linux" });
    setLive({ ...typstMain });
    render(HeaderBar, {
      title: "Inkwell",
      subtitle: "main.typ",
      status: "alone",
      me: { id: "me", name: "Moi" },
      peers: [],
    });
    expect(loads).toBe(0);
    // The status items' code: one entry per extension that has some (app, typst), counted.
    const entries = new Set(r.status.map((x) => `${x.folder}/${x.entry}`));
    expect([...entries].sort()).toEqual(["app/native/status.ts", "typst/native/status.ts"]);
  });
});

describe("EX0c1 review fixes", () => {
  it("no two commands the palette lists share a label, in either language", () => {
    for (const language of ["fr", "en"] as const) {
      const seen = new Map<string, string>();
      for (const c of registry.commands.filter((x) => x.id !== "app.palette")) {
        const label = textIn(language, c.label as MessageKey);
        expect(seen.get(label), `${language}: « ${label} » is ${c.id} and ${seen.get(label)}`).toBe(
          undefined,
        );
        seen.set(label, c.id);
      }
    }
  });

  it("the preview's zoom commands act only where the preview is open", () => {
    for (const id of ["typst.preview-zoom-out", "typst.preview-zoom-in", "typst.preview-fit"]) {
      useRegistry(registry, { system: "linux" });
      setLive({ ...typstMain, previewOpen: false });
      expect(availability(id).ok, id).toBe(false);
      setLive({ ...typstMain });
      expect(availability(id).ok, id).toBe(true);
    }
  });
});

describe("EX0c1 review fixes: status items the registry refuses", () => {
  const REFUSED = import.meta.glob("../../extensions/testdata/manifests/refused/*.json", {
    query: "?raw",
    import: "default",
    eager: true,
  }) as Record<string, string>;

  it("a duplicate status id or a status item running an unknown command is refused, named", () => {
    const names = Object.keys(REFUSED).map((p) => p.slice(p.lastIndexOf("/") + 1));
    expect(names.some((n) => n.includes("duplicate-status-id"))).toBe(true);
    expect(names.some((n) => n.includes("status-command"))).toBe(true);
    for (const [path, text] of Object.entries(REFUSED)) {
      const name = path.slice(path.lastIndexOf("/") + 1);
      const r = builtins([source(text)]);
      const reasons = r.refused.filter((x) => x.folder === "sample").map((x) => x.reason);
      expect(reasons, name).toHaveLength(1);
      expect(reasons[0]?.startsWith(name.slice(0, name.indexOf(".")).replace("-", " ")), name).toBe(
        true,
      );
      expect(
        r.status.filter((x) => x.extension === "sample"),
        name,
      ).toEqual([]);
    }
  });
});

describe("EX0c1 review fixes: the status bar at start", () => {
  it("the built-ins' status items draw with the first paint, no entry loaded", () => {
    let loads = 0;
    const r = buildRegistry(builtinSources(), (folder, entry) => {
      loads++;
      return globLoader()(folder, entry);
    });
    useRegistry(r, { system: "linux" });
    setLive({ ...typstMain });
    const host = render(StatusBar, { ...bar(), ofMain: true, unsaved: true } as unknown as Record<
      string,
      unknown
    >);
    expect(host.querySelector('[data-testid="unsaved"]')).not.toBeNull();
    expect(host.querySelector('[data-testid="position"]')).not.toBeNull();
    expect(host.querySelector(".people")).not.toBeNull();
    expect(loads).toBe(0);
  });
});
