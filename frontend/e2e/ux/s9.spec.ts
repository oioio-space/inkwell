import { writeFileSync } from "node:fs";
import { join } from "node:path";
import type { Page } from "@playwright/test";
import { docsRoot, mainTypIn, openWith, readDisk } from "../disk";
import { expect, test } from "../fixtures";
import { clear, docText, typeAtEnd } from "../helpers";
import { ALICE_PORT, ALICE_URL } from "../ports";

// Defects of the S9 walkthrough (outside edits), found by looking at the screen.

const file = () => mainTypIn(join(docsRoot, String(ALICE_PORT)));

async function start(page: Page, text: string) {
  await clear(page);
  await typeAtEnd(page, text);
  await expect.poll(() => readDisk(file()), { timeout: 10_000 }).toBe(text);
}

test("UX-S9-1 only the lines that came from the disk are marked, not the old line they follow", async ({
  browser,
}) => {
  const page = await openWith(browser, ALICE_URL, { saveMode: "auto", saveDelay: 1 });
  await start(page, "= Titre");
  writeFileSync(file(), "= Titre\nUne ligne\nvenue d'ailleurs\n");
  await expect(page.getByRole("status").filter({ hasText: "2 lignes intégrées" })).toBeVisible();
  // Lines 2 and 3 are marked in the gutter; line 1 is what the person had.
  await expect(page.locator(".cm-external-gutter")).toHaveCount(2);
  await expect(page.locator(".cm-external-gutter").first()).toHaveText("2");
});

test("UX-S9-2 an outside edit that only removed lines is marked and can be undone", async ({
  browser,
}) => {
  const page = await openWith(browser, ALICE_URL, { saveMode: "auto", saveDelay: 1 });
  await start(page, "a b c");
  writeFileSync(file(), "a  c");
  const toast = page.getByRole("status").filter({ hasText: "Modifié hors d'Inkwell" });
  await expect(toast.getByRole("button", { name: "Voir" })).toBeVisible();
  await expect(page.locator(".cm-external-gutter")).toHaveCount(1);
  await toast.getByRole("button", { name: "Annuler" }).click();
  await expect.poll(() => docText(page)).toBe("a b c");
});
