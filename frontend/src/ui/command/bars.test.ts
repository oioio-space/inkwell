import { tick } from "svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import { registry } from "../../extensions";
import {
  configure,
  menuOf,
  registerSurface,
  type Shell,
  useRegistry,
} from "../../extensions/commands";
import type { LiveContext } from "../../extensions/live.svelte";
import { setLive } from "../../extensions/live.svelte";
import { builtinSources, globLoader } from "../../extensions/loader";
import { buildRegistry, type Viewer } from "../../extensions/registry";
import { emptyContext } from "../../extensions/when";
import { gridHandlers } from "../sheet/commands";
import { cleanup, click, fakeWidth, press, render } from "../testkit.svelte";
import LocationBar from "./LocationBar.svelte";
import LocationMenuAt from "./LocationMenuAt.svelte";
import LocationToolbar from "./LocationToolbar.svelte";
import ViewBar from "./ViewBar.svelte";

const cmds = (host: ParentNode, sel = "[data-cmd]") =>
  [...host.querySelectorAll<HTMLElement>(sel)].map((b) => b.dataset.cmd);
const btn = (host: ParentNode, id: string) =>
  host.querySelector<HTMLElement>(`[data-cmd="${id}"]`) as HTMLElement;

/** A grid surface that says `state` of itself, and records what its commands did. */
function grid(state: Partial<LiveContext>) {
  const el = document.createElement("section");
  document.body.append(el);
  const did: string[] = [];
  const ctrl = new Proxy({ active: { r: 0, c: 0 } } as Record<string, unknown>, {
    get: (o, name) => (name === "active" ? o.active : () => did.push(String(name))),
  });
  registerSurface(
    el,
    "grid",
    gridHandlers({
      ctrl: () => ctrl as never,
      find: () => {},
      step: () => undefined,
      filter: () => did.push("filter"),
      copy: () => did.push("copy"),
      paste: () => did.push("paste"),
      autofit: () => did.push("autofit"),
    }),
    { viewer: registry.viewers.find((v) => v.id === "tables.grid") as Viewer, state: () => state },
  );
  return { el, did };
}

const bar = (el: Element, after?: (id: string) => void) =>
  render(LocationToolbar, {
    location: "grid/toolbar",
    label: "Outils du tableur",
    target: el,
    after,
  });

afterEach(() => {
  cleanup();
  useRegistry(registry, { system: "linux" });
  setLive({ ...emptyContext() });
  vi.restoreAllMocks();
});

describe("EX0c2 the grid's bar and menu, the view, history and difference bars", () => {
  it("EX0c2 P-6 the grid's bar is its location, greyed and pressed from the grid's own state, the focus kept on the grid, overflowing by width", () => {
    // The shell's context says read-only for a table (it is not the main document): the grid says otherwise.
    setLive({ readOnly: true });
    const { el, did } = grid({
      readOnly: false,
      gridLocked: false,
      gridFiltered: false,
      gridFrozen: false,
    });
    const after = vi.fn();
    const wide = fakeWidth(1200);
    let host = bar(el, after);
    expect(cmds(host)).toEqual([
      "app.undo",
      "app.redo",
      "tables.insert-row-above",
      "tables.delete-rows",
      "tables.insert-col-left",
      "tables.delete-cols",
      "tables.sort-asc",
      "tables.sort-desc",
      "tables.filter",
      "tables.freeze",
    ]);
    expect(host.querySelector('[role="toolbar"]')?.getAttribute("aria-label")).toBe(
      "Outils du tableur",
    );
    expect(btn(host, "tables.insert-row-above").getAttribute("aria-label")).toBe(
      "Insérer une ligne au-dessus",
    );
    for (const id of cmds(host))
      expect(btn(host, id as string).getAttribute("aria-disabled"), id).toBeNull();
    // A press keeps the grid's focus; after the click the host says which command ran.
    const down = new MouseEvent("mousedown", { bubbles: true, cancelable: true });
    btn(host, "tables.sort-asc").dispatchEvent(down);
    expect(down.defaultPrevented).toBe(true);
    click(btn(host, "tables.sort-asc"));
    expect(did).toEqual(["sortBy"]);
    expect(after).toHaveBeenCalledWith("tables.sort-asc");
    cleanup();
    wide();
    const wider = fakeWidth(1200);
    // Its own read-only state (the reader role, a lock, a file too big) greys every editing item.
    setLive({ readOnly: false });
    host = bar(grid({ readOnly: true }).el);
    for (const id of [
      "app.undo",
      "app.redo",
      "tables.insert-row-above",
      "tables.delete-rows",
      "tables.insert-col-left",
      "tables.delete-cols",
    ])
      expect(btn(host, id).getAttribute("aria-disabled"), id).toBe("true");
    for (const id of ["tables.sort-asc", "tables.sort-desc", "tables.filter", "tables.freeze"])
      expect(btn(host, id).getAttribute("aria-disabled"), id).toBeNull();
    cleanup();
    // A sort or a filter locks the rows (not the columns); a filter presses Filter; frozen panes swap the two.
    host = bar(
      grid({ readOnly: false, gridLocked: true, gridFiltered: true, gridFrozen: true }).el,
    );
    expect(btn(host, "tables.insert-row-above").getAttribute("aria-disabled")).toBe("true");
    expect(btn(host, "tables.insert-col-left").getAttribute("aria-disabled")).toBeNull();
    expect(btn(host, "tables.filter").getAttribute("aria-pressed")).toBe("true");
    expect(cmds(host).slice(-1)).toEqual(["tables.unfreeze"]);
    expect(btn(host, "tables.unfreeze").getAttribute("aria-label")).toBe("Libérer les volets");
    expect(btn(host, "tables.unfreeze").getAttribute("aria-pressed")).toBe("true");
    cleanup();
    wider();
    // Narrow: what does not fit is in ⋯, in order, and nothing is lost.
    const restore = fakeWidth(260);
    try {
      const g = grid({ readOnly: false });
      host = bar(g.el);
      const onBar = cmds(
        host.querySelector('[role="toolbar"]') as HTMLElement,
        ":scope > [data-cmd]",
      );
      expect(onBar.length).toBeLessThan(10);
      click(host.querySelector('button[aria-label="Plus d\'actions"]') as HTMLElement);
      const inMenu = cmds(document, '[role^="menuitem"][data-cmd]');
      expect([...onBar, ...inMenu]).toEqual(
        menuOf("grid/toolbar", {}, g.el).flatMap((x) => x.items.map((c) => c.id)),
      );
    } finally {
      restore();
    }
  });

  it("EX0c2 P-7 the grid's context menu is its location, its items hidden by their when, its keys those of the menu pattern", () => {
    const { el } = grid({ readOnly: false, gridFrozen: true });
    const onclose = vi.fn();
    let host = render(LocationMenuAt, {
      location: "grid/context",
      x: 10,
      y: 10,
      label: "Actions sur la sélection",
      target: el,
      onclose,
    });
    const menu = host.querySelector('[role="menu"]') as HTMLElement;
    expect(menu.getAttribute("aria-label")).toBe("Actions sur la sélection");
    const items = () => [...menu.querySelectorAll<HTMLElement>(":scope > [role^=menuitem]")];
    expect(items().map((i) => i.dataset.cmd)).toEqual(
      menuOf("grid/context", {}, el).flatMap((g) => g.items.map((c) => c.id)),
    );
    expect(items().map((i) => i.dataset.cmd)).toContain("tables.unfreeze");
    // A separator between groups, the precise names, a toggle as a checked box.
    expect(menu.querySelectorAll('[role="separator"]')).toHaveLength(3);
    expect(btn(menu, "tables.insert-row-above").textContent).toContain(
      "Insérer une ligne au-dessus",
    );
    expect(btn(menu, "tables.unfreeze").getAttribute("role")).toBe("menuitemcheckbox");
    expect(btn(menu, "tables.unfreeze").getAttribute("aria-checked")).toBe("true");
    // Native buttons: Enter and Space run the item.
    expect(items().every((i) => i.tagName === "BUTTON")).toBe(true);
    // The keys: the first item has the focus, Up and Down wrap, Home and End, Escape and Tab close.
    const first = items()[0] as HTMLElement;
    const last = items().at(-1) as HTMLElement;
    first.focus();
    press(first, "ArrowUp");
    expect(document.activeElement).toBe(last);
    press(last, "ArrowDown");
    expect(document.activeElement).toBe(first);
    press(first, "End");
    expect(document.activeElement).toBe(last);
    press(last, "Home");
    expect(document.activeElement).toBe(first);
    press(first, "Escape");
    press(first, "Tab");
    expect(onclose).toHaveBeenCalledTimes(2);
    cleanup();
    // Read-only (its own state): the editing items are hidden, copying and sorting stay.
    const ro = grid({ readOnly: true });
    host = render(LocationMenuAt, {
      location: "grid/context",
      x: 0,
      y: 0,
      label: "m",
      target: ro.el,
      onclose,
    });
    const shown = cmds(host);
    for (const id of [
      "tables.cut",
      "tables.paste",
      "tables.paste-values",
      "tables.insert-row-above",
      "tables.delete-rows",
      "tables.clear",
      "tables.duplicate-row",
      "tables.unfreeze",
    ])
      expect(shown, id).not.toContain(id);
    expect(shown).toEqual(
      expect.arrayContaining(["tables.copy", "tables.sort-asc", "tables.freeze", "tables.autofit"]),
    );
  });

  it("EX0c2 P-8 past 8 extension items a context menu groups them in an Extensions submenu", async () => {
    const plugin = (n: number) => ({
      origin: "plugin" as const,
      folder: "plug",
      text: JSON.stringify({
        id: "plug",
        version: "1.0.0",
        contributes: {
          commands: Array.from({ length: n }, (_, i) => ({
            id: `plug.c${i}`,
            label: `C${i}`,
            category: "edit",
          })),
          menus: Array.from({ length: n }, (_, i) => ({
            command: `plug.c${i}`,
            location: "grid/context",
            group: "extensions",
            order: n - i,
          })),
        },
      }),
      files: new Set<string>(),
    });
    const { el } = grid({ readOnly: false });
    // Eight: inline, after a separator.
    useRegistry(buildRegistry([...builtinSources(), plugin(8)], globLoader()), { system: "linux" });
    let host = render(LocationMenuAt, {
      location: "grid/context",
      x: 0,
      y: 0,
      label: "m",
      target: el,
      onclose: () => {},
    });
    expect(cmds(host).filter((c) => c?.startsWith("plug."))).toHaveLength(8);
    expect(host.querySelector("[aria-haspopup]")).toBeNull();
    cleanup();
    // Nine: one « Extensions » item; Right opens them in order with the focus on the first, Left closes.
    const g = grid({ readOnly: false });
    useRegistry(buildRegistry([...builtinSources(), plugin(9)], globLoader()), { system: "linux" });
    host = render(LocationMenuAt, {
      location: "grid/context",
      x: 0,
      y: 0,
      label: "m",
      target: g.el,
      onclose: () => {},
    });
    const menu = host.querySelector('[role="menu"]') as HTMLElement;
    expect(cmds(menu).filter((c) => c?.startsWith("plug."))).toHaveLength(0);
    const parent = menu.querySelector<HTMLElement>("[aria-haspopup]") as HTMLElement;
    expect(parent.textContent).toContain("Extensions");
    expect(parent.getAttribute("aria-expanded")).toBe("false");
    parent.focus();
    press(parent, "ArrowRight");
    await tick();
    const sub = menu.querySelector<HTMLElement>('[role="menu"]') as HTMLElement;
    expect(document.activeElement).toBe(sub.querySelector("[data-cmd]"));
    expect(parent.getAttribute("aria-expanded")).toBe("true");
    expect(cmds(sub)).toEqual(Array.from({ length: 9 }, (_, i) => `plug.c${8 - i}`));
    press(sub, "ArrowLeft");
    expect(menu.querySelector('[role="menu"]')).toBeNull();
    expect(document.activeElement).toBe(parent);
  });

  it("EX0c2 P-9 the view bar offers the type's default viewer only while another viewer is shown", () => {
    const gridViewer = registry.viewers.find((v) => v.id === "tables.grid") as Viewer;
    const text = registry.essentialViewer("text");
    const ondefault = vi.fn();
    // The default viewer shows: no view bar (a grid has none, the CSV-20 screenshots).
    let host = render(ViewBar, {
      offer: false,
      onoffer: () => {},
      defaultViewer: gridViewer,
      shown: gridViewer,
      ondefault,
    });
    expect(host.querySelector(".viewbar")).toBeNull();
    cleanup();
    // Another one shows: the default is offered, by its own label.
    host = render(ViewBar, {
      offer: false,
      onoffer: () => {},
      defaultViewer: gridViewer,
      shown: text,
      ondefault,
    });
    const back = host.querySelector<HTMLElement>('[data-fixed="default-viewer"]') as HTMLElement;
    expect(back.textContent?.trim()).toBe("Voir le tableau");
    expect(back.dataset.viewer).toBe("tables.grid");
    click(back);
    expect(ondefault).toHaveBeenCalledOnce();
    cleanup();
    // A text that reads as a table is offered as one.
    host = render(ViewBar, {
      offer: true,
      onoffer: () => {},
      defaultViewer: text,
      shown: text,
      ondefault,
    });
    expect(host.querySelector('[data-fixed="offer-table"]')?.textContent?.trim()).toBe(
      "Ouvrir comme tableau ?",
    );
    expect(host.querySelector('[data-fixed="default-viewer"]')).toBeNull();
    // Which viewer draws its own status bar is the viewer's (`ownStatus`), not a type test.
    expect(registry.viewers.filter((v) => v.ownStatus).map((v) => v.id)).toEqual(["tables.grid"]);
  });

  it("EX0c2 P-10 the history strip and the difference bar are their locations, one name per command", () => {
    const calls: string[] = [];
    configure({ shell: new Proxy({} as Shell, { get: (_, n) => () => calls.push(String(n)) }) });
    // The strip: « Marquer tout comme vu » while there are marks to see.
    setLive({ unseen: true });
    let host = render(LocationBar, { location: "history/toolbar", labelClass: "btn" });
    const seen = btn(host, "app.mark-seen");
    expect(seen.textContent?.trim()).toBe("Marquer tout comme vu");
    expect(seen.className).toContain("btn");
    expect(seen.getAttribute("aria-disabled")).toBeNull();
    cleanup();
    setLive({ unseen: false });
    host = render(LocationBar, { location: "history/toolbar", labelClass: "btn" });
    expect(btn(host, "app.mark-seen").getAttribute("aria-disabled")).toBe("true");
    cleanup();
    // The difference bar: its two commands, by the diff view's own `changes`.
    for (const changes of [true, false]) {
      const dialog = document.createElement("dialog");
      document.body.append(dialog);
      registerSurface(
        dialog,
        "diff",
        { "app.next-change": () => {}, "app.previous-change": () => {} },
        { state: () => ({ changes }) },
      );
      host = render(LocationBar, { location: "diff/toolbar", target: dialog });
      expect(cmds(host)).toEqual(["app.previous-change", "app.next-change"]);
      expect(btn(host, "app.next-change").textContent?.trim()).toBe("Différence suivante");
      expect(btn(host, "app.next-change").getAttribute("aria-disabled")).toBe(
        changes ? null : "true",
      );
      cleanup();
    }
  });

  it("EX0c2 review: an item run from the bar's ⋯ menu closes it, then gives the grid its focus back", async () => {
    const restore = fakeWidth(160);
    try {
      const g = grid({ readOnly: false, gridFrozen: true });
      const order: string[] = [];
      const host = bar(g.el, (id) =>
        order.push(
          `${id} ${document.querySelector('[role="menu"]') ? "menu open" : "menu closed"}`,
        ),
      );
      click(host.querySelector('button[aria-label="Plus d\'actions"]') as HTMLElement);
      click(
        document.querySelector('[role^="menuitem"][data-cmd="tables.unfreeze"]') as HTMLElement,
      );
      expect(g.did).toEqual(["unfreeze"]);
      await tick();
      expect(order).toEqual(["tables.unfreeze menu closed"]);
    } finally {
      restore();
    }
  });
});
