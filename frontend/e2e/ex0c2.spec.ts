// Block EX0c2 (docs/plans/2026-11-25-ex0c2-panels-grid.md): the window's two sides drawn from the
// `panels` point, the grid's bar and context menu, the view bar, the history strip and the
// difference bar drawn from their locations, in the real application. E1: every control of those
// regions is its location's or point's resolved item, in order, or a fixed widget of a closed list,
// so a hard-coded control fails, named « stray » (the gate's MV13 adds one). E2: the drawer's tabs
// and the grid's menu by keyboard, the focus given back, the drawer's code fetched at idle, no
// empty panel shown, its first opening within 100 ms.
import { unlinkSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import type { Locator, Page } from "@playwright/test";
import { docsRoot, mainTypIn } from "./disk";
import { expect, test } from "./fixtures";
import { clear, docText, editor, open, pair, typeAtEnd } from "./helpers";
import { ALICE_PORT, ALICE_URL, BOB_URL } from "./ports";
import { cell, grid, openSheet, pick, setText } from "./sheetHelpers";

test.beforeEach(() => test.setTimeout(150_000));

/** The fixed widgets a region may hold besides its items: a closed list (plan §4). */
const FIXED = [
  "show-text",
  "default-viewer",
  "offer-table",
  "since",
  "legend",
  "colour",
  "diff-mode",
] as const;

/** What the registry resolves now: a location's items (on a surface), or a side's panels. */
const resolved = (page: Page, location: string, target?: string) =>
  page.evaluate(
    ([loc, sel]) =>
      (
        globalThis as typeof globalThis & {
          __inkwellLocation: (l: string, t?: string) => string[];
        }
      ).__inkwellLocation(loc as string, sel),
    [location, target] as const,
  );
const panels = (page: Page, side: "start" | "end") =>
  page.evaluate(
    (s) =>
      (
        globalThis as typeof globalThis & { __inkwellPanels: (x: string) => string[] }
      ).__inkwellPanels(s),
    side,
  );

/**
 * Each control of a region: « cmd:<id> » for a command, « fixed:<name> » for a fixed widget of the
 * closed list (itself or inside one), « tab:<panel> » for a side's tab; anything else is
 * « stray:<its name> ».
 */
function controls(region: Locator): Promise<string[]> {
  return region.locator('button, [role^="menuitem"], [role="tab"]').evaluateAll(
    (els, fixed) =>
      els.map((el) => {
        const own = (el as HTMLElement).closest<HTMLElement>("[data-fixed]")?.dataset.fixed;
        if (own && fixed.includes(own)) return `fixed:${own}`;
        const h = el as HTMLElement;
        if (h.dataset.cmd) return `cmd:${h.dataset.cmd}`;
        if (h.getAttribute("role") === "tab" && h.dataset.panel) return `tab:${h.dataset.panel}`;
        return `stray:${el.getAttribute("aria-label") ?? el.textContent?.trim()}`;
      }),
    [...FIXED] as string[],
  );
}
const of = (list: string[], kind: string) =>
  list.filter((x) => x.startsWith(`${kind}:`)).map((x) => x.slice(kind.length + 1));
const stray = (list: string[]) => of(list, "stray");

const drawer = (p: Page) => p.getByTestId("history-drawer");
const toggle = (p: Page) => p.locator('header [data-cmd="app.review"]');

test("EX0c2-E1 the sides, the grid's bar and menu, the view bar, the history strip and the difference bar are their resolved items, or fixed widgets", async ({
  browser,
}) => {
  // The grid: its bar and its context menu, by the grid's own state.
  const page = await openSheet(browser, ALICE_URL);
  await setText(page, "n;v\na;1\nb;2\n");
  const bar = page.getByRole("toolbar", { name: "Outils du tableur" });
  const onBar = await controls(bar);
  expect(stray(onBar), "stray controls in the grid's bar").toEqual([]);
  expect(of(onBar, "cmd")).toEqual(await resolved(page, "grid/toolbar", "section.sheet"));
  expect(of(onBar, "fixed")).toEqual(["show-text"]);
  expect(onBar.at(-1)).toBe("fixed:show-text");
  await cell(page, 1, 1).click({ button: "right" });
  const menu = page.getByRole("menu", { name: "Actions sur la sélection" });
  const inMenu = await controls(menu);
  expect(stray(inMenu), "stray items in the grid's menu").toEqual([]);
  expect(of(inMenu, "cmd")).toEqual(await resolved(page, "grid/context", "section.sheet"));
  expect(of(inMenu, "cmd")).toContain("tables.cut");
  await page.keyboard.press("Escape");
  // The view bar: under the text of a table, the default viewer only.
  await page.getByRole("button", { name: "Voir le texte" }).click();
  const viewbar = page.locator(".viewbar");
  await expect(viewbar).toHaveCount(1);
  const onView = await controls(viewbar);
  expect(stray(onView), "stray controls in the view bar").toEqual([]);
  expect(onView).toEqual([
    "fixed:default-viewer",
    ...(await resolved(page, "view")).map((c) => `cmd:${c}`),
  ]);
  await page.getByRole("button", { name: "Voir le tableau" }).click();
  await expect(grid(page)).toBeVisible();
  await expect(viewbar).toHaveCount(0);
  await page.context().close();

  // A table read from the disk is read-only: the bar greys its editing items, the menu hides them.
  const alice = await open(browser, ALICE_URL);
  await clear(alice);
  await editor(alice).fill("= Rapport\n");
  await alice.keyboard.press("ControlOrMeta+s");
  await expect.poll(() => mainTypIn(join(docsRoot, String(ALICE_PORT)))).toBeTruthy();
  const dir = dirname(mainTypIn(join(docsRoot, String(ALICE_PORT))));
  const csv = join(dir, "c2-stock.csv");
  writeFileSync(csv, "n,v\na,1\nb,2\n");
  try {
    await expect(alice.locator('[data-path="c2-stock.csv"]')).toBeVisible({ timeout: 15_000 });
    await alice.locator('[data-path="c2-stock.csv"]').click();
    await expect(grid(alice)).toBeVisible({ timeout: 20_000 });
    const roBar = alice.getByRole("toolbar", { name: "Outils du tableur" });
    expect(of(await controls(roBar), "cmd")).toEqual(
      await resolved(alice, "grid/toolbar", "section.sheet"),
    );
    for (const name of ["Annuler", "Insérer une ligne au-dessus", "Supprimer les lignes"])
      await expect(roBar.getByRole("button", { name, exact: true })).toBeDisabled();
    await expect(roBar.getByRole("button", { name: "Trier de A à Z" })).toBeEnabled();
    await cell(alice, 0, 0).click({ button: "right" });
    const roMenu = await controls(alice.getByRole("menu", { name: "Actions sur la sélection" }));
    expect(of(roMenu, "cmd")).toEqual(await resolved(alice, "grid/context", "section.sheet"));
    expect(of(roMenu, "cmd")).not.toContain("tables.cut");
    expect(of(roMenu, "cmd")).toContain("tables.copy");
    await alice.keyboard.press("Escape");
  } finally {
    unlinkSync(csv);
  }

  // The sides: the files alone (no tab list), the drawer's tabs in the point's order.
  await alice.locator('[data-path="main.typ"]').click();
  const start = alice.getByRole("complementary", { name: "Fichiers du projet" });
  const onStart = await controls(start);
  expect(of(onStart, "tab")).toEqual([]);
  expect(await panels(alice, "start")).toEqual(["app.files"]);
  await toggle(alice).click();
  await expect(drawer(alice)).toBeVisible();
  const tabs = await controls(drawer(alice).getByRole("tablist"));
  expect(stray(tabs), "stray controls in the drawer's tab list").toEqual([]);
  expect(of(tabs, "tab")).toEqual(await panels(alice, "end"));

  // The difference bar: the mode pick, then diff/toolbar's items.
  const bob = await open(browser, BOB_URL, true);
  await clear(bob);
  await pair(alice, bob);
  await typeAtEnd(bob, "Ajout de Bob.\n");
  await expect.poll(() => docText(alice), { timeout: 20_000 }).toContain("Ajout de Bob");
  // The history strip: what Bob added since Alice's last visit.
  const strip = alice.getByTestId("change-strip");
  await expect(strip).toBeVisible({ timeout: 20_000 });
  const onStrip = await controls(strip);
  expect(stray(onStrip), "stray controls in the history strip").toEqual([]);
  expect(of(onStrip, "cmd")).toEqual(await resolved(alice, "history/toolbar"));
  expect(of(onStrip, "cmd")).toEqual(["app.mark-seen"]);
  await expect(strip.getByRole("button", { name: "Marquer tout comme vu" })).toBeVisible();
  await drawer(alice).getByRole("tab", { name: "Historique" }).click();
  await alice
    .getByRole("listbox", { name: "Moments de l'historique" })
    .getByRole("option", { name: /^Bob/ })
    .first()
    .click();
  await alice.getByRole("button", { name: "Voir les changements" }).click();
  const view = alice.getByTestId("diff-view");
  await expect(view).toBeVisible();
  const onDiff = await controls(view.locator(".bar"));
  expect(stray(onDiff), "stray controls in the difference bar").toEqual([]);
  expect(of(onDiff, "cmd")).toEqual(
    await resolved(alice, "diff/toolbar", '[data-testid="diff-view"]'),
  );
  expect(of(onDiff, "cmd")).toEqual(["app.previous-change", "app.next-change"]);
  await expect(view.getByRole("button", { name: "Différence suivante" })).toBeEnabled();
  await alice.keyboard.press("Escape");
  await alice.context().close();
  await bob.context().close();
});

test("EX0c2-E2 the drawer's tabs and the grid's menu by keyboard, the focus given back, the drawer ready at once", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  const scripts: string[] = [];
  page.on("request", (r) => {
    if (r.resourceType() === "script") scripts.push(new URL(r.url()).pathname);
  });
  await page.reload();
  await expect(editor(page)).toBeVisible();
  // The drawer's code comes at idle after the first paint, before anyone opens it.
  await expect
    .poll(() => scripts.some((s) => /\/assets\/drawer-[\w-]+\.js$/.test(s)), { timeout: 10_000 })
    .toBe(true);
  // Its first opening: within 100 ms of the key, with no frame showing an empty panel.
  await editor(page).click();
  // The watch starts in the page, at the key's own event (capture), and ends at the drawer's frame.
  await page.evaluate(() => {
    const w = globalThis as typeof globalThis & {
      __c2Open?: Promise<{ ms: number; empty: number }>;
    };
    w.__c2Open = new Promise((done) => {
      let t0 = 0;
      let empty = 0;
      const seen = new MutationObserver(() => {
        const side = document.querySelector('[data-testid="history-drawer"]');
        const shown = side?.querySelector('[role="tabpanel"]:not([hidden])');
        if (side && (!shown || shown.childElementCount === 0)) empty++;
      });
      seen.observe(document.body, { childList: true, subtree: true });
      // The toggle's own click, from the page (a click that moves no focus, as a key would).
      t0 = performance.now();
      document.querySelector<HTMLElement>('header [data-cmd="app.review"]')?.click();
      const frame = () => {
        if (t0 && document.querySelector('[data-testid="history-drawer"]')) {
          seen.disconnect();
          done({ ms: performance.now() - t0, empty });
        } else requestAnimationFrame(frame);
      };
      requestAnimationFrame(frame);
    });
  });
  const opened = await page.evaluate(
    () =>
      (globalThis as typeof globalThis & { __c2Open: Promise<{ ms: number; empty: number }> })
        .__c2Open,
  );
  expect(opened.empty).toBe(0);
  expect(opened.ms).toBeLessThan(100);
  // Opening left the focus in the text.
  await expect(page.locator(".cm-content")).toBeFocused();
  // The tabs: roving focus, arrows wrap, Home and End, a tab selected as it takes the focus.
  const list = drawer(page).getByRole("tablist", { name: "Relire" });
  const tab = (name: string) => list.getByRole("tab", { name });
  await tab("Historique").focus();
  await page.keyboard.press("ArrowRight");
  await expect(tab("Commentaires")).toBeFocused();
  await expect(tab("Commentaires")).toHaveAttribute("aria-selected", "true");
  await page.keyboard.press("End");
  await expect(tab("Activité")).toBeFocused();
  await page.keyboard.press("ArrowRight");
  await expect(tab("Problèmes")).toBeFocused();
  await expect(tab("Problèmes")).toHaveAttribute("aria-selected", "true");
  await page.keyboard.press("ArrowLeft");
  await expect(tab("Activité")).toBeFocused();
  await page.keyboard.press("Home");
  await expect(tab("Problèmes")).toBeFocused();
  // Tab goes from the tab list into its panel's content.
  const panelId = await tab("Problèmes").getAttribute("aria-controls");
  await expect(page.locator(`#${panelId}`)).toBeVisible();
  // Closing the drawer while a tab has the focus: the focus goes back to the text, not to the page.
  await page.evaluate(() =>
    document.querySelector<HTMLElement>('header [data-cmd="app.review"]')?.click(),
  );
  await expect(drawer(page)).toHaveCount(0);
  await expect(page.locator(".cm-content")).toBeFocused();
  // Its chord opens and closes it from the text (Playwright types « h » with Shift held, as Caps Lock
  // does: the Shift chord, never Ctrl+H's Replace).
  await page.keyboard.press("ControlOrMeta+Shift+h");
  await expect(drawer(page)).toBeVisible();
  await expect(page.locator(".cm-content")).toBeFocused();
  await page.keyboard.press("ControlOrMeta+Shift+h");
  await expect(drawer(page)).toHaveCount(0);
  await page.context().close();

  // The grid's menu by keyboard: Shift+F10 opens it on the first item; Up wraps; End; Escape back.
  const sheet = await openSheet(browser, ALICE_URL);
  await setText(sheet, "n;v\na;1\nb;2\n");
  await pick(sheet, 1, 1);
  await sheet.keyboard.press("Shift+F10");
  const menu = sheet.getByRole("menu", { name: "Actions sur la sélection" });
  await expect(menu).toBeVisible();
  const items = menu.locator(":scope > [role^=menuitem]");
  await expect(items.first()).toBeFocused();
  await sheet.keyboard.press("ArrowUp");
  await expect(items.last()).toBeFocused();
  await sheet.keyboard.press("Home");
  await expect(items.first()).toBeFocused();
  await sheet.keyboard.press("Escape");
  await expect(menu).toHaveCount(0);
  await expect(grid(sheet)).toBeFocused();
  // Enter runs the item: freezing from the menu, then the bar offers to unfreeze, pressed.
  await sheet.keyboard.press("Shift+F10");
  await menu.getByRole("menuitem", { name: "Figer jusqu'à la sélection" }).focus();
  await sheet.keyboard.press("Enter");
  await expect(menu).toHaveCount(0);
  await expect(sheet.getByRole("button", { name: "Libérer les volets" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(grid(sheet)).toBeFocused();
  await sheet.getByRole("button", { name: "Libérer les volets" }).click();
  await expect(grid(sheet)).toBeFocused();
  await setText(sheet, "");
  await sheet.context().close();
});
