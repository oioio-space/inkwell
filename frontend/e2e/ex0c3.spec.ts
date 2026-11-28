// Block EX0c3 (docs/plans/2026-11-25-ex0c3-insert-format.md): the Insert menu is the `insert`
// location, the formatting language the `format` entry of a grammar, in the real application. E1:
// every item of the Insert menu is the location's resolved item, in order, or a fixed widget of a
// closed list, so a hard-coded item fails, named « stray » (the gate's MI8 adds one); each item is
// named by its label and described by its description and syntax; a note and a table have no bar;
// the palette lists the insert commands and opens a step item at its picker. E2: insertions in
// Typst, the picker's types, cite without a bibliography, the status bar's language and words.
// A note opens read-only (a projection), so Markdown's own menu (its frozen CSV copy, its note) is
// proved by the rule tests F-4 and F-5 (ui/command/ex0c3.test.ts).
import { unlinkSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import type { Locator, Page } from "@playwright/test";
import { docsRoot, mainTypIn } from "./disk";
import { expect, test } from "./fixtures";
import { clear, docText, editor, open } from "./helpers";
import { ALICE_PORT, ALICE_URL } from "./ports";

test.beforeEach(() => test.setTimeout(120_000));

const PNG = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=",
  "base64",
);
const FILES: Record<string, string | Buffer> = {
  "c3-notes.md": "# Notes\n\nun texte de note\n",
  "c3-data.csv": "Mois,Ventes\nJanvier,1200\n",
  "c3-pic.png": PNG,
};

/** The fixed widgets the Insert menu may hold besides its items: a closed list (plan §2). */
const FIXED = [
  "insert-back",
  "insert-file",
  "insert-key",
  "insert-keyform",
  "insert-note",
  "insert-nobib",
  "insert-add-bib",
] as const;

/** Alice's project with a note, a table and a picture beside main.typ. */
async function withFiles(page: Page, body: () => Promise<void>) {
  await clear(page);
  await editor(page).fill("= Rapport\nBonjour tout le monde\n");
  await page.keyboard.press("ControlOrMeta+s");
  await expect.poll(() => mainTypIn(join(docsRoot, String(ALICE_PORT)))).toBeTruthy();
  const dir = dirname(mainTypIn(join(docsRoot, String(ALICE_PORT))));
  try {
    for (const [name, data] of Object.entries(FILES)) writeFileSync(join(dir, name), data);
    await expect(page.locator('[data-path="c3-pic.png"]')).toBeVisible({ timeout: 15_000 });
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

/** Each control of the menu: « cmd:<id> », « fixed:<name> » of the closed list, else « stray:… ». */
function controls(region: Locator): Promise<string[]> {
  return region.locator('button, [role^="menuitem"]').evaluateAll(
    (els, fixed) =>
      els.map((el) => {
        const own = (el as HTMLElement).closest<HTMLElement>("[data-fixed]")?.dataset.fixed;
        if (own && fixed.includes(own)) return `fixed:${own}`;
        const cmd = (el as HTMLElement).dataset.cmd;
        if (cmd) return `cmd:${cmd}`;
        return `stray:${el.getAttribute("aria-label") ?? el.textContent?.trim()}`;
      }),
    [...FIXED] as string[],
  );
}
const of = (list: string[], kind: string) =>
  list.filter((x) => x.startsWith(`${kind}:`)).map((x) => x.slice(kind.length + 1));

const trigger = (p: Page) => p.getByRole("button", { name: "Insérer", exact: true });
const menu = (p: Page) =>
  p.getByRole("menu", { name: "Insérer un fichier, une référence, une note…" });
const bar = (p: Page) => p.locator('[role="toolbar"][aria-label^="Mise en forme"]');
const palette = (p: Page) => p.getByRole("combobox", { name: "Rechercher une commande" });
const options = (p: Page) => p.getByRole("listbox", { name: "Commandes" }).getByRole("option");
/** Runs a menu entry from the keyboard, as the menu's own keys do (a long menu scrolls under the status bar). */
async function choose(entry: Locator) {
  await entry.focus();
  await entry.press("Enter");
}
const picks = (p: Page) =>
  menu(p)
    .locator('[data-fixed="insert-file"]')
    .evaluateAll((els) => els.map((e) => e.textContent?.trim()));

test("EX0c3-E1 the Insert menu is the insert location's items in order, or fixed widgets; names, descriptions and the palette", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await withFiles(page, async () => {
    await page.locator('[data-path="main.typ"]').click();
    await expect(bar(page)).toBeVisible();
    await trigger(page).click();
    await expect(menu(page)).toBeVisible();
    const items = await controls(menu(page));
    expect(of(items, "stray"), "stray items in the Insert menu").toEqual([]);
    expect(of(items, "cmd")).toEqual(await resolved(page, "insert"));
    expect(of(items, "cmd")).toContain("format.insert-footnote");
    // Each item is named by its label, described by its description and the syntax it writes.
    const image = menu(page).locator('[data-cmd="format.insert-image"]');
    await expect(image).toHaveAccessibleName("Image ou page de PDF");
    await expect(image).toHaveAccessibleDescription("Affiche le fichier dans le document #image()");
    await expect(menu(page).locator('[data-cmd="format.insert-pagebreak"]')).toHaveAccessibleName(
      "Saut de page",
    );
    await page.keyboard.press("Escape");
    await expect(menu(page)).toBeHidden();

    // A note and a table have no bar, so no Insert menu.
    for (const name of ["c3-notes.md", "c3-data.csv"]) {
      await page.locator(`[data-path="${name}"]`).click();
      await expect(page.locator(".ttl small")).toHaveText(name);
      await expect(bar(page)).toHaveCount(0);
    }

    // The palette lists the insert commands; a step item opens the menu at its picker.
    await page.locator('[data-path="main.typ"]').click();
    await expect(bar(page)).toBeVisible();
    await editor(page).click();
    await page.keyboard.press("Control+Shift+P");
    await palette(page).fill("note de bas");
    await expect(options(page).first()).toContainText("Note de bas de page");
    await palette(page).fill("image ou page");
    await expect(options(page).first()).toContainText("Image ou page de PDF");
    await page.keyboard.press("Enter");
    await expect(palette(page)).toBeHidden();
    await expect(menu(page)).toBeVisible();
    await expect.poll(() => picks(page)).toEqual(["c3-pic.png"]);
    expect(of(await controls(menu(page)), "stray")).toEqual([]);
    await page.keyboard.press("Escape");
  });
  await page.context().close();
});

test("EX0c3-E2 insertions in Typst, the picker's types, cite without a bibliography, the status bar's language and words", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await withFiles(page, async () => {
    await page.locator('[data-path="main.typ"]').click();
    await expect(page.getByTestId("language")).toHaveText("Typst");
    await expect(page.getByTestId("words")).toContainText("5");
    await editor(page).click();
    await page.keyboard.press("ControlOrMeta+End");
    // Nothing to ask: written at once, one undo step.
    await trigger(page).click();
    await choose(menu(page).locator('[data-insert="footnote"]'));
    await expect.poll(() => docText(page)).toContain("#footnote[]");
    await trigger(page).click();
    await choose(menu(page).locator('[data-insert="pagebreak"]'));
    await expect.poll(() => docText(page)).toContain("#pagebreak()");
    await page.keyboard.press("ControlOrMeta+z");
    await expect.poll(() => docText(page)).not.toContain("#pagebreak()");
    expect(await docText(page)).toContain("#footnote[]");
    // An image: the picker lists the pictures only.
    await trigger(page).click();
    await choose(menu(page).locator('[data-insert="image"]'));
    await expect.poll(() => picks(page)).toEqual(["c3-pic.png"]);
    await choose(menu(page).locator('[data-fixed="insert-file"]').first());
    await expect.poll(() => docText(page)).toContain('#image("c3-pic.png")');
    // A citation with no bibliography: the step that offers one.
    await trigger(page).click();
    await choose(menu(page).locator('[data-insert="cite"]'));
    await expect(menu(page).locator('[data-fixed="insert-nobib"]')).toBeVisible();
    await choose(menu(page).locator('[data-fixed="insert-add-bib"]'));
    await expect(menu(page).locator('[data-fixed="insert-note"]')).toContainText(".bib");
    await page.keyboard.press("Escape");

    // A note: its language and its words, no Typst items.
    await page.locator('[data-path="c3-notes.md"]').click();
    await expect(page.locator(".ttl small")).toHaveText("c3-notes.md");
    await expect(page.getByTestId("language")).toHaveText("Markdown");
    await expect(page.getByTestId("words")).toContainText("5");
  });
  await page.context().close();
});
