// Block EX0c1 (docs/plans/2026-11-25-ex0c-locations.md): the header bar, the formatting bar, the
// preview's tools and the status bar drawn from the registry's locations, in the real application.
// E1: every button and menu item of those regions is its location's item, in order, or a named
// fixed widget, so a hard-coded button fails. E2: the status bar's items for main.typ, a note and a
// picture, its layout at 640 px, and the zoom menu's steps.
import { unlinkSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import type { Locator, Page } from "@playwright/test";
import { docsRoot, mainTypIn } from "./disk";
import { expect, test } from "./fixtures";
import { clear, editor, open } from "./helpers";
import { ALICE_PORT, ALICE_URL } from "./ports";

test.beforeEach(() => test.setTimeout(120_000));

const PNG = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=",
  "base64",
);
const FILES: Record<string, string | Buffer> = {
  "c1-notes.md": "# Notes\n",
  "c1-figures.csv": "Mois,Ventes\nJanvier,1200\n",
  "c1-logo.png": PNG,
};

/** Alice's project with a note, a table and a picture beside main.typ. */
async function withFiles(page: Page, body: () => Promise<void>) {
  await clear(page);
  await editor(page).fill("= Rapport\nBonjour\n");
  await page.keyboard.press("ControlOrMeta+s");
  await expect.poll(() => mainTypIn(join(docsRoot, String(ALICE_PORT)))).toBeTruthy();
  const dir = dirname(mainTypIn(join(docsRoot, String(ALICE_PORT))));
  try {
    for (const [name, data] of Object.entries(FILES)) writeFileSync(join(dir, name), data);
    await expect(page.locator('[data-path="c1-logo.png"]')).toBeVisible();
    await body();
  } finally {
    for (const name of Object.keys(FILES)) {
      try {
        unlinkSync(join(dir, name));
      } catch {}
    }
  }
}

/** The location's items whose `when` holds now, as the registry resolves them. */
const resolved = (page: Page, location: string) =>
  page.evaluate(
    (loc) =>
      (
        globalThis as typeof globalThis & { __inkwellLocation: (l: string) => string[] }
      ).__inkwellLocation(loc),
    location,
  );

/**
 * Each control of a region, as « cmd:<id> » when it runs a command or « fixed:<name> » when it is
 * one of the region's named fixed widgets; anything else is « unknown:<its name> ».
 */
async function controls(region: Locator, fixed: Record<string, string>): Promise<string[]> {
  return region.locator('button, [role^="menuitem"]').evaluateAll(
    (els, fixedSel) =>
      els.map((el) => {
        for (const [name, sel] of Object.entries(fixedSel))
          if (el.matches(sel)) return `fixed:${name}`;
        const cmd = (el as HTMLElement).dataset.cmd;
        return cmd ? `cmd:${cmd}` : `unknown:${el.getAttribute("aria-label") ?? el.textContent}`;
      }),
    fixed,
  );
}
const commandsOf = (list: string[]) =>
  list.filter((x) => x.startsWith("cmd:")).map((x) => x.slice(4));

test("EX0c1-E1 the header bar, the formatting bar and the preview's tools are their locations' items, or named fixed widgets", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await expect(editor(page)).toBeVisible();

  // The header bar: header/start, then header/end around the people and the main menu.
  const fixedEnd = {
    people: '[data-testid="people-open"]',
    menu: 'button[aria-label="Menu principal"]',
  };
  const header = page.locator("header.hb");
  const start = await controls(header.locator(".start"), {});
  expect(start).toEqual((await resolved(page, "header/start")).map((id) => `cmd:${id}`));
  const end = await controls(header.locator(".end"), fixedEnd);
  expect(end.filter((x) => !x.startsWith("cmd:"))).toEqual(["fixed:people", "fixed:menu"]);
  expect(end.at(-1)).toBe("fixed:menu");
  expect(commandsOf(end)).toEqual(await resolved(page, "header/end"));
  expect(commandsOf(end)).toContain("typst.present");

  // The formatting bar: the heading menu and Insert around the format location's items.
  const bar = page.getByRole("toolbar", { name: /mise en forme/i });
  const fixedBar = {
    heading: "button.heading",
    // Insert's trigger is named by its text (« Insérer »), the heading menu's by its class.
    insert: 'button[aria-haspopup="menu"]:not(.heading):not([aria-label])',
    more: 'button[aria-label="Plus d\'actions"]',
  };
  const wide = await controls(bar, fixedBar);
  expect(wide.filter((x) => x.startsWith("unknown:"))).toEqual([]);
  expect(wide[0]).toBe("fixed:heading");
  expect(wide.at(-1)).toBe("fixed:insert");
  expect(commandsOf(wide)).toEqual(await resolved(page, "format"));
  // 800 px: what does not fit is in ⋯, in the same order, and nothing is lost.
  await page.setViewportSize({ width: 800, height: 640 });
  await expect(bar.getByRole("button", { name: "Plus d'actions" })).toBeVisible();
  const narrow = commandsOf(await controls(bar, fixedBar));
  await bar.getByRole("button", { name: "Plus d'actions" }).click();
  const menu = page.getByRole("menu", { name: "Plus d'actions" });
  const overflow = commandsOf(await controls(menu, {}));
  expect(overflow.length).toBeGreaterThan(0);
  expect([...narrow, ...overflow]).toEqual(await resolved(page, "format"));
  await page.keyboard.press("Escape");
  await page.setViewportSize({ width: 1100, height: 640 });

  // The preview's tools: preview/toolbar around the zoom level.
  const tools = page.getByRole("group", { name: /aperçu/i });
  const previewTools = await controls(tools, { fit: '[data-el="preview.fit"]' });
  expect(previewTools.filter((x) => x.startsWith("unknown:"))).toEqual([]);
  expect(previewTools[1]).toBe("fixed:fit");
  expect(commandsOf(previewTools)).toEqual(await resolved(page, "preview/toolbar"));
  await expect(tools.getByRole("button", { name: "Réduire l'aperçu" })).toBeVisible();
  await expect(tools.getByRole("button", { name: "Agrandir l'aperçu" })).toBeVisible();

  // A note and a table have no formatting bar; the header keeps its items, Present is Typst's.
  await withFiles(page, async () => {
    for (const name of ["c1-notes.md", "c1-figures.csv"]) {
      await page.locator(`[data-path="${name}"]`).click();
      await expect(page.locator(".ttl small")).toHaveText(name);
      await expect(page.getByRole("toolbar", { name: /mise en forme/i })).toHaveCount(0);
      const items = commandsOf(await controls(header.locator(".end"), fixedEnd));
      expect(items).toEqual(await resolved(page, "header/end"));
      expect(items).not.toContain("typst.present");
    }
  });
  await page.context().close();
});

test("EX0c1-E2 the status bar shows its items for main.typ, a note and a picture, keeps its layout at 640 px, and its zoom menu steps", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  const footer = page.getByRole("contentinfo", { name: "Barre d'état" });
  const items = () =>
    footer
      .locator("[data-status-item]")
      .evaluateAll((els) => els.map((e) => (e as HTMLElement).dataset.statusItem));
  await withFiles(page, async () => {
    await page.locator('[data-path="main.typ"]').click();
    await expect(page.getByTestId("position")).toBeVisible();
    await expect(page.getByTestId("page-indicator")).toBeVisible();
    expect(await items()).toEqual(await resolved(page, "status"));
    expect(await items()).toEqual([
      "app.presence",
      "typst.problems",
      "typst.page",
      "app.unsaved",
      "app.position",
      "app.words",
      "app.zoom",
      "app.encoding",
      "app.language",
    ]);
    // A note: no problems and no page; the caret, the words and the language.
    await page.locator('[data-path="c1-notes.md"]').click();
    await expect(page.locator(".ttl small")).toHaveText("c1-notes.md");
    await expect.poll(items).not.toContain("typst.problems");
    expect(await items()).toEqual(await resolved(page, "status"));
    expect(await items()).not.toContain("typst.page");
    expect(await items()).toContain("app.position");
    // A picture: no caret, no words, no language.
    await page.locator('[data-path="c1-logo.png"]').click();
    await expect(page.locator(".ttl small")).toHaveText("c1-logo.png");
    await expect.poll(items).not.toContain("app.position");
    expect(await items()).toEqual(await resolved(page, "status"));
    expect(await items()).not.toContain("app.words");
    expect(await items()).not.toContain("app.language");
    await page.locator('[data-path="main.typ"]').click();
    await expect(page.getByTestId("position")).toBeVisible();
  });

  // 640 px: the page and the language leave a squeezed bar; the people, the caret and the zoom stay.
  await page.setViewportSize({ width: 640, height: 640 });
  await expect(page.getByTestId("language")).toBeHidden();
  await expect(page.getByTestId("page-indicator")).toBeHidden();
  await expect(page.getByTestId("position")).toBeVisible();
  await expect(page.getByTestId("text-zoom")).toBeVisible();
  await expect(footer.locator(".people")).toBeVisible();
  const box = await footer.boundingBox();
  expect(box?.height).toBe(36);
  await page.setViewportSize({ width: 1100, height: 640 });

  // The zoom menu is the status/zoom location: its steps, in order, act on the text.
  const zoom = page.getByTestId("text-zoom");
  await expect(zoom).toHaveText("100 %");
  await zoom.click();
  const steps = page.getByRole("menu", { name: "Taille du texte" }).locator("[data-cmd]");
  expect(await steps.evaluateAll((els) => els.map((e) => (e as HTMLElement).dataset.cmd))).toEqual(
    await resolved(page, "status/zoom"),
  );
  await page
    .getByRole("menu", { name: "Taille du texte" })
    .getByRole("menuitem", { name: /Agrandir/ })
    .click();
  await expect(zoom).toHaveText("110 %");
  await page.keyboard.press("Escape");
  await page.keyboard.press("ControlOrMeta+0");
  await expect(zoom).toHaveText("100 %");
  await page.context().close();
});
