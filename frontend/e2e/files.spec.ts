import { rmSync, unlinkSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { docsRoot, mainTypIn } from "./disk";
import { expect, test } from "./fixtures";
import { editor, open } from "./helpers";
import { ALICE_PORT, ALICE_URL } from "./ports";

test("FM-OPEN-1 FM-OPEN-2 FM-OPEN-3: sidebar opens each projection and names the active file", async ({
  browser,
}, info) => {
  const page = await open(browser, ALICE_URL);
  await editor(page).fill("= Main L2\n");
  await page.keyboard.press("ControlOrMeta+s");
  await expect.poll(() => mainTypIn(join(docsRoot, String(ALICE_PORT)))).toBeTruthy();
  const dir = dirname(mainTypIn(join(docsRoot, String(ALICE_PORT))));
  const files = {
    "l2.txt": "Plain L2",
    "l2.md": "# Markdown L2",
    "l2.typ": "= Typst L2",
    "l2.csv": "Name,Count\nApple,42\n",
    "l2.png": Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=",
      "base64",
    ),
    "l2.bin": Buffer.from([0, 255, 0]),
  };
  try {
    for (const [name, text] of Object.entries(files)) writeFileSync(join(dir, name), text);
    for (const [name, text] of Object.entries(files)) {
      await page.locator(`[data-path="${name}"]`).click();
      await expect(page.locator(".ttl small")).toHaveText(name);
      await expect(page.locator(`[data-path="${name}"]`)).toHaveAttribute("aria-current", "page");
      if (name.endsWith(".csv")) {
        await expect(page.getByRole("grid")).toHaveAttribute("aria-readonly", "true");
        await expect(page.getByRole("grid")).toContainText("Apple");
        for (const colorScheme of ["light", "dark"] as const) {
          await page.emulateMedia({ colorScheme });
          await page.screenshot({ path: info.outputPath(`files-readonly-${colorScheme}.png`) });
        }
        await page.emulateMedia({ colorScheme: "light" });
      } else if (typeof text === "string") {
        await expect(editor(page)).toContainText(text);
        await expect(editor(page)).toHaveAttribute("aria-readonly", "true");
        await editor(page).click();
        await page.keyboard.type("MUTATION");
        await expect(editor(page)).not.toContainText("MUTATION");
      } else if (name.endsWith(".png")) {
        await expect(page.getByRole("img", { name })).toBeVisible();
      } else {
        await expect(page.getByText("Aucun aperçu pour ce type de fichier.")).toBeVisible();
      }
      // Every text, the grid included, carries the wording of design §4.16 / spec §6.6.
      if (typeof text === "string")
        await expect(
          page.getByText("Dans cette version, ce fichier s'affiche en lecture seule", {
            exact: true,
          }),
        ).toBeVisible();
    }
    // Edited, then deleted outside Inkwell while open (design §4.16): the view follows the
    // disk, then a toast says the file is gone and the main document comes back.
    await page.locator('[data-path="l2.txt"]').click();
    await expect(editor(page)).toContainText("Plain L2");
    writeFileSync(join(dir, "l2.txt"), "Plain L2, edited outside");
    await expect(editor(page)).toContainText("edited outside");
    unlinkSync(join(dir, "l2.txt"));
    await expect(
      page.getByText("l2.txt n'est plus dans le projet", { exact: false }),
    ).toBeVisible();
    await expect(page.locator('[data-path="main.typ"]')).toHaveAttribute("aria-current", "page");
    await expect(page.locator('[data-path="l2.txt"]')).toHaveCount(0);
    await page.locator('[data-path="main.typ"]').click();
    await expect(editor(page)).toContainText("Main L2");
    await expect(editor(page)).not.toHaveAttribute("aria-readonly", "true");
    expect(new URL(page.url()).search).toBe("");
    await editor(page).fill("");
  } finally {
    for (const name of Object.keys(files)) rmSync(join(dir, name), { force: true });
    await page.context().close();
  }
});
