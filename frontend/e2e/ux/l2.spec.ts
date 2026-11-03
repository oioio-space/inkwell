import { unlinkSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import type { Page } from "@playwright/test";
import { docsRoot, mainTypIn } from "../disk";
import { expect, test } from "../fixtures";
import { clear, editor, open, pair } from "../helpers";
import { ALICE_PORT, ALICE_URL, BOB_URL } from "../ports";

// Defects of the L2 walkthrough (opening a file for real), found by looking at the screen.

const PNG = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=",
  "base64",
);
const FILES: Record<string, string | Buffer> = {
  "figures.csv": "Mois,Ventes\nJanvier,1200\n",
  "notes.md": "# Notes\n",
  "logo.png": PNG,
  "blob.bin": Buffer.from([0, 255, 0]),
};

/** Alice's project folder, with the L2 sample files written beside main.typ. */
async function withFiles(page: Page, body: (dir: string) => Promise<void>) {
  await clear(page);
  await editor(page).fill("= Rapport\nBonjour\n");
  await page.keyboard.press("ControlOrMeta+s");
  await expect.poll(() => mainTypIn(join(docsRoot, String(ALICE_PORT)))).toBeTruthy();
  const dir = dirname(mainTypIn(join(docsRoot, String(ALICE_PORT))));
  try {
    for (const [name, data] of Object.entries(FILES)) writeFileSync(join(dir, name), data);
    await expect(page.locator('[data-path="blob.bin"]')).toBeVisible();
    await body(dir);
  } finally {
    for (const name of Object.keys(FILES)) {
      try {
        unlinkSync(join(dir, name));
      } catch {}
    }
  }
}

const bar = (p: Page) => p.getByRole("contentinfo", { name: "Barre d'état" });

test("UX-L2-1 a picture or an info card does not wear main.typ's page, problems and word count", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await withFiles(page, async () => {
    for (const name of ["logo.png", "blob.bin", "notes.md"]) {
      await page.locator(`[data-path="${name}"]`).click();
      await expect(page.locator(".ttl small")).toHaveText(name);
      // The preview is main.typ's pages; the problems and the page count are main.typ's too.
      await expect(page.locator(".side-preview")).toHaveCount(0);
      await expect(page.getByTestId("problems")).toHaveCount(0);
      await expect(page.getByTestId("page-indicator")).toHaveCount(0);
    }
    for (const name of ["logo.png", "blob.bin"]) {
      await page.locator(`[data-path="${name}"]`).click();
      // No text, so no words, no caret position, no language to announce.
      await expect(page.getByTestId("words")).toHaveCount(0);
      await expect(page.getByTestId("position")).toHaveCount(0);
      await expect(page.getByTestId("language")).toHaveCount(0);
      await expect(bar(page)).toBeVisible();
    }
    await page.locator('[data-path="main.typ"]').click();
    await expect(page.locator(".side-preview")).toBeVisible();
    await expect(page.getByTestId("position")).toBeVisible();
  });
  await page.context().close();
});

test("UX-L2-2 a table opened read-only says so once, not twice", async ({ browser }) => {
  const page = await open(browser, ALICE_URL);
  await withFiles(page, async () => {
    await page.locator('[data-path="figures.csv"]').click();
    await expect(page.getByRole("grid")).toHaveAttribute("aria-readonly", "true");
    await expect(page.getByText(/lecture seule|pas le modifier/)).toHaveCount(1);
  });
  await page.context().close();
});

test("UX-L2-3 Space opens a file and leaves the focus on the tree row", async ({ browser }) => {
  const page = await open(browser, ALICE_URL);
  await withFiles(page, async () => {
    const row = page.locator('[data-path="notes.md"]');
    await row.focus();
    await page.keyboard.press("Space");
    await expect(page.locator(".ttl small")).toHaveText("notes.md");
    await expect(row).toBeFocused();
    // Enter, on the other hand, goes to the content.
    await page.keyboard.press("Enter");
    await expect(editor(page)).toBeFocused();
  });
  await page.context().close();
});

test("UX-L2-4 each person sees which file the other has open", async ({ browser }) => {
  test.setTimeout(120_000);
  const alice = await open(browser, ALICE_URL);
  const bob = await open(browser, BOB_URL);
  await clear(alice);
  await clear(bob);
  await pair(alice, bob);
  await withFiles(alice, async () => {
    await alice.locator('[data-path="figures.csv"]').click();
    await bob.getByRole("button", { name: "Voir les personnes" }).click();
    const panel = bob.getByRole("dialog", { name: "Personnes" });
    await expect(panel.getByRole("listitem").filter({ hasText: "Alice" }).first()).toContainText(
      "figures.csv",
      { timeout: 15_000 },
    );
    await bob.screenshot({ path: test.info().outputPath("bob-sees-alice-on-csv.png") });
    await alice.locator('[data-path="main.typ"]').click();
    await expect(panel.getByRole("listitem").filter({ hasText: "Alice" }).first()).toContainText(
      "main.typ",
      { timeout: 15_000 },
    );
  });
});
