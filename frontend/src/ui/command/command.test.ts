import { flushSync } from "svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import { formatHandlers, formatsOf } from "../../editor/format";
import { registry } from "../../extensions";
import * as commands from "../../extensions/commands";
import { ariaKeys } from "../../extensions/keys";
import { setLive } from "../../extensions/live.svelte";
import { emptyContext } from "../../extensions/when";
import { workspace } from "../../workspace/workspace.svelte";
import ZoomMenu from "../popover/ZoomMenu.svelte";
import HeaderBar from "../shell/HeaderBar.svelte";
import { cleanup, click, fakeEditor, fakeWidth, render } from "../testkit.svelte";
import FormatBar from "../toolbar/FormatBar.svelte";

// The spy every control must reach: `run()` with its command's id, through no other path.
vi.mock("../../extensions/commands", async (original) => {
  const m = await original<typeof import("../../extensions/commands")>();
  return { ...m, run: vi.fn(m.run) };
});

afterEach(() => {
  cleanup();
  vi.mocked(commands.run).mockClear();
});

/** The identity a control bound to a command must carry, read from the registry itself. */
function checkIdentity(el: HTMLElement) {
  const id = el.dataset.cmd as string;
  const c = registry.command(id);
  expect(c, `data-cmd ${id}`).toBeDefined();
  if (!c) return;
  const label = commands.labelOf(c);
  const name = el.getAttribute("aria-label") ?? el.textContent?.trim() ?? "";
  expect(name.startsWith(label), `${id}: « ${name} » starts with « ${label} »`).toBe(true);
  const chord = commands.chordsOf(id)[0];
  const shown = commands.shortcutOf(id);
  const reason = commands.availability(id).reason;
  const tip = el.getAttribute("aria-describedby");
  if (tip && !reason) {
    // Its own tooltip says the label and this system's chord: no second one from `title`.
    expect(el.getAttribute("title"), id).toBeNull();
    const text = document.getElementById(tip)?.textContent ?? "";
    expect(text, id).toContain(label);
    if (shown) expect(text, id).toContain(shown);
  } else
    expect(el.getAttribute("title"), id).toBe(reason ?? (shown ? `${label} (${shown})` : label));
  expect(el.getAttribute("aria-keyshortcuts"), id).toBe(chord ? ariaKeys(chord) : null);
  vi.mocked(commands.run).mockClear();
  click(el);
  expect(
    vi.mocked(commands.run).mock.calls.map((call) => call[0]),
    id,
  ).toEqual([id]);
}

describe("EX0b1 controls bound to commands", () => {
  it("EX0b1 C-11 every control bound to a command carries its id, the registry's identity and runs through run()", () => {
    commands.useRegistry(registry, { system: "linux" });
    setLive({
      ...emptyContext(),
      canPresent: true,
      hasToolbar: true,
      toolbarVisible: true,
      previewOpen: true,
      sidebarOpen: true,
    });

    // The header bar: six buttons, and the main menu drawn from the registry.
    const header = render(HeaderBar, {
      title: "Inkwell",
      subtitle: "main.typ",
      status: "alone",
      me: { id: "me", name: "Moi" },
      peers: [],
    });
    const runs = [...header.querySelectorAll<HTMLButtonElement>("button:not([aria-haspopup])")];
    expect(runs.map((b) => b.dataset.cmd)).toEqual([
      "app.sidebar",
      "app.preview",
      "typst.present",
      "app.people",
      "app.share",
      "app.review",
    ]);
    for (const b of runs) checkIdentity(b);
    // A toggle keeps one label in both states; its state is aria-pressed.
    const sidebar = runs[0] as HTMLButtonElement;
    const before = sidebar.getAttribute("aria-label");
    expect(sidebar.getAttribute("aria-pressed")).toBe("true");
    setLive({ sidebarOpen: false });
    flushSync();
    expect(sidebar.getAttribute("aria-label")).toBe(before);
    expect(sidebar.getAttribute("aria-pressed")).toBe("false");
    click(header.querySelector('button[aria-haspopup="menu"]') as HTMLElement);
    const items = [...document.querySelectorAll<HTMLElement>('[role="menu"] [role^="menuitem"]')];
    expect(items.map((i) => i.dataset.cmd)).toEqual([
      "app.toolbar",
      "app.preview",
      "app.fullscreen",
      "app.palette",
      "typst.present",
      "typst.present-here",
      "typst.present-public",
      "typst.export",
      "app.preferences",
      "app.shortcuts",
      "app.about",
    ]);
    expect(items[0]?.getAttribute("role")).toBe("menuitemcheckbox");
    expect(items[0]?.getAttribute("aria-checked")).toBe("true");
    for (const i of items) {
      if (!i.isConnected)
        click(header.querySelector('button[aria-haspopup="menu"]') as HTMLElement);
      const again = document.querySelector<HTMLElement>(
        `[role="menu"] [data-cmd="${i.dataset.cmd}"]`,
      );
      checkIdentity(again as HTMLElement);
    }
    cleanup();

    // The formatting bar and its heading menu, on an editor surface.
    vi.spyOn(Object.getPrototypeOf(workspace), "rightsKnown", "get").mockReturnValue(true);
    const restore = fakeWidth(1400);
    const editor = fakeEditor("un mot‸");
    const target = document.createElement("div");
    document.body.append(target);
    commands.registerSurface(target, "editor", formatHandlers("typst", editor.run), {
      viewer: registry.essentialViewer("text"),
    });
    setLive({ formats: formatsOf("typst"), readOnly: false });
    const bar = render(FormatBar, {
      lang: "typst",
      editorState: editor.state,
      target,
      run: editor.run,
      files: [],
      currentFile: "main.typ",
      readFile: async () => undefined,
    });
    const barButtons = [...bar.querySelectorAll<HTMLElement>("button[data-bar]")];
    expect(barButtons.length).toBeGreaterThan(8);
    for (const b of barButtons) {
      expect(b.dataset.cmd).toBe(`format.${b.dataset.bar}`);
      checkIdentity(b);
    }
    click(bar.querySelector('button[aria-haspopup="menu"]') as HTMLElement); // the heading menu
    const levels = [...document.querySelectorAll<HTMLElement>('[role="menuitemradio"]')];
    expect(levels.map((l) => l.dataset.cmd)).toEqual([
      "format.paragraph",
      "format.h1",
      "format.h2",
      "format.h3",
      "format.h4",
    ]);
    for (const l of levels) {
      const open = document.querySelector<HTMLElement>(
        `[role="menuitemradio"][data-cmd="${l.dataset.cmd}"]`,
      );
      if (!open) click(bar.querySelector('button[aria-haspopup="menu"]') as HTMLElement);
      const el = document.querySelector<HTMLElement>(
        `[role="menuitemradio"][data-cmd="${l.dataset.cmd}"]`,
      ) as HTMLElement;
      // The level already chosen stays: activating it runs nothing (a paragraph would follow).
      if (el.getAttribute("aria-checked") === "true") continue;
      checkIdentity(el);
    }
    restore();
    cleanup();

    // The zoom menu of the status bar.
    const zoom = render(ZoomMenu, {});
    click(zoom.querySelector('button[aria-haspopup="menu"]') as HTMLElement);
    const steps = [...document.querySelectorAll<HTMLElement>('[role="menu"] [data-cmd]')];
    expect(steps.map((s) => s.dataset.cmd)).toEqual([
      "app.zoom-out",
      "app.zoom-in",
      "app.zoom-reset",
    ]);
    for (const s of steps.filter((x) => !x.hasAttribute("disabled"))) checkIdentity(s);
  });
});
