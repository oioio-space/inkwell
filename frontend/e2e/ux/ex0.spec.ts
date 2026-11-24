import { rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import type { Page } from "@playwright/test";
import { docsRoot, mainTypIn } from "../disk";
import { expect, test } from "../fixtures";
import { editor, open } from "../helpers";
import { ALICE_PORT, ALICE_URL } from "../ports";

// Defects of the EX0 walkthrough (gates/EX0/UX.md), found by looking at the screen. Each test
// FAILS on the current tree: it states the expected behaviour of the defect it is named after.

async function projectDir(page: Page): Promise<string> {
  await editor(page).fill("= UX EX0\n");
  await page.keyboard.press("ControlOrMeta+s");
  await expect.poll(() => mainTypIn(join(docsRoot, String(ALICE_PORT)))).toBeTruthy();
  return dirname(mainTypIn(join(docsRoot, String(ALICE_PORT))));
}

test("UX-EX0-1 the caret keeps its cell when a table is shown as text", async ({ browser }) => {
  const page = await open(browser, ALICE_URL);
  const dir = await projectDir(page);
  const file = join(dir, "ux-ex0-cells.csv");
  try {
    writeFileSync(file, "nom;ville;montant\nAlice;Paris;12\nBruno;Lyon;30\nChloé;Nantes;45\n");
    await page.reload();
    await page.locator('[data-path="ux-ex0-cells.csv"]').click();
    await page.getByRole("gridcell", { name: "Lyon" }).click();
    await page.getByRole("button", { name: "Voir le texte" }).first().click();
    await expect(editor(page)).toContainText("Bruno;Lyon;30");
    // Lyon is on line 3 (header = line 1), after "Bruno;": column 7. The text view must open there,
    // as the table already does the other way round (text line 4 -> row 4).
    await expect(page.getByText(/Ligne 3, colonne \d+/)).toBeVisible();
  } finally {
    rmSync(file, { force: true });
    await page.context().close();
  }
});

test("UX-EX0-2 after a failed table load, « Ouvrir comme tableau ? » is not offered (a dead button)", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  const dir = await projectDir(page);
  const file = join(dir, "ux-ex0-stock.txt");
  try {
    writeFileSync(file, "nom\tqte\nPomme\t3\nPoire\t5\nKiwi\t7\n");
    // The grid's chunk cannot be fetched (bad network).
    await page.route(/\/assets\/grid-[\w-]+\.js$/, (route) => route.abort());
    await page.reload();
    await page.locator('[data-path="ux-ex0-stock.txt"]').click();
    await expect(editor(page)).toContainText("Kiwi");
    const offer = page.getByRole("button", { name: "Ouvrir comme tableau ?" });
    // Either the offer is gone, or pressing it explains why nothing happens: never silence.
    if (await offer.count()) {
      await offer.click();
      await expect(page.getByRole("status").filter({ hasText: "n'a pas pu s'ouvrir" })).toBeVisible(
        { timeout: 3_000 },
      );
    }
  } finally {
    rmSync(file, { force: true });
    await page.context().close();
  }
});

test("UX-EX0-4 after a switch of view, the keyboard is in the view just shown", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  const dir = await projectDir(page);
  const file = join(dir, "ux-ex0-focus.csv");
  const inside = (selector: string) =>
    page.evaluate((s) => document.activeElement?.closest(s) !== null, selector);
  try {
    writeFileSync(file, "nom;ville\nAlice;Paris\nBruno;Lyon\n");
    await page.reload();
    await page.locator('[data-path="ux-ex0-focus.csv"]').click();
    await expect(page.getByRole("grid")).toContainText("Lyon");
    await page.getByRole("button", { name: "Voir le texte" }).first().focus();
    await page.keyboard.press("Enter");
    await expect(editor(page)).toContainText("Bruno;Lyon");
    await expect.poll(() => inside(".cm-editor")).toBe(true);
    await page.getByRole("button", { name: "Voir le tableau" }).focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("grid")).toContainText("Lyon");
    await expect.poll(() => inside('[role="grid"]')).toBe(true);
  } finally {
    rmSync(file, { force: true });
    await page.context().close();
  }
});

test("UX-EX0-5 the view chosen for a file is remembered like its type, on this device", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  const dir = await projectDir(page);
  const file = join(dir, "ux-ex0-memory.csv");
  try {
    writeFileSync(file, "nom;ville\nAlice;Paris\nBruno;Lyon\n");
    await page.reload();
    await page.locator('[data-path="ux-ex0-memory.csv"]').click();
    await expect(page.getByRole("grid")).toContainText("Lyon");
    await page.getByRole("button", { name: "Voir le texte" }).first().click();
    await expect(editor(page)).toContainText("Bruno;Lyon");
    // Away and back, then after a reload: the text, as left.
    await page.locator('[data-path="main.typ"]').click();
    await page.locator('[data-path="ux-ex0-memory.csv"]').click();
    await expect(editor(page)).toContainText("Bruno;Lyon");
    await expect(page.getByRole("grid")).toHaveCount(0);
    await page.reload();
    await page.locator('[data-path="ux-ex0-memory.csv"]').click();
    await expect(editor(page)).toContainText("Bruno;Lyon");
    await expect(page.getByRole("grid")).toHaveCount(0);
    // Back to the table: remembered too.
    await page.getByRole("button", { name: "Voir le tableau" }).click();
    await expect(page.getByRole("grid")).toContainText("Lyon");
    await page.reload();
    await page.locator('[data-path="ux-ex0-memory.csv"]').click();
    await expect(page.getByRole("grid")).toContainText("Lyon");
  } finally {
    rmSync(file, { force: true });
    await page.context().close();
  }
});
