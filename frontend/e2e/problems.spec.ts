import { rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { docsRoot, mainTypIn } from "./disk";
import { expect, test } from "./fixtures";
import { editor, open } from "./helpers";
import { ALICE_PORT, ALICE_URL } from "./ports";

test("ER-1–4: one count, F8 and click open the file and select the diagnostic range", async ({
  browser,
}, info) => {
  const page = await open(browser, ALICE_URL);
  await editor(page).fill('#l3UnknownName("OK")\n');
  await page.keyboard.press("ControlOrMeta+s");
  const counter = page.getByTestId("problems");
  await expect(counter).toHaveAttribute("data-errors", "1");
  // The sole status counter navigates on a plain click; F8 does the same after moving away.
  await expect(
    page.getByRole("button", { name: /Problèmes du document.*Problème suivant/ }),
  ).toHaveCount(1);
  await expect(page.getByRole("button", { name: "Tous les problèmes", exact: true })).toHaveCount(
    0,
  );
  await expect(page.locator(".writing-problem.error").first()).toBeVisible();
  await counter.click();
  await expect
    .poll(() =>
      page.evaluate(() =>
        (
          globalThis as typeof globalThis & { __inkwellSelection?: () => string }
        ).__inkwellSelection?.(),
      ),
    )
    .toBe("l3UnknownName");
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.press("F8");
  // Typing replaces the whole selected error name, proving a range rather than just a caret.
  await page.keyboard.type("text");
  await expect(editor(page)).toContainText("#text");
  await expect(editor(page)).not.toContainText("l3UnknownName");
  await page.keyboard.press("ControlOrMeta+s");
  await expect(counter).toHaveAttribute("data-errors", "0");
  const dir = dirname(mainTypIn(join(docsRoot, String(ALICE_PORT))));
  const path = join(dir, "l3-problem.typ");
  try {
    writeFileSync(path, "#l3OtherUnknown\n");
    await editor(page).fill('#include "l3-problem.typ"\n');
    await page.keyboard.press("ControlOrMeta+s");
    await expect(counter).toHaveAttribute("data-errors", "1");
    await counter.click({ modifiers: ["Shift"] });
    const panel = page.getByRole("region", { name: "Problèmes" });
    await expect(panel.getByRole("heading", { name: "l3-problem.typ" })).toBeVisible();
    await panel.getByRole("button").first().click();
    await expect(page.locator(".ttl small")).toHaveText("l3-problem.typ");
    await expect(editor(page)).toContainText("l3OtherUnknown");
    await expect
      .poll(() =>
        page.evaluate(() =>
          (
            globalThis as typeof globalThis & { __inkwellSelection?: () => string }
          ).__inkwellSelection?.(),
        ),
      )
      .toBe("l3OtherUnknown");
    for (const colorScheme of ["light", "dark"] as const) {
      await page.emulateMedia({ colorScheme });
      await page.screenshot({ path: info.outputPath(`problems-${colorScheme}.png`) });
    }
    await page.keyboard.press("Shift+F8");
    await expect(page.locator(".ttl small")).toHaveText("l3-problem.typ");
  } finally {
    await page.locator('[data-path="main.typ"]').click();
    await editor(page).fill("");
    await page.keyboard.press("ControlOrMeta+s");
    rmSync(path, { force: true });
    await page.context().close();
  }
});

test("N29: Typst completion uses the bundled language service", async ({ browser }) => {
  const page = await open(browser, ALICE_URL);
  await editor(page).fill("#tex");
  await expect
    .poll(() =>
      page.evaluate(() =>
        (
          globalThis as typeof globalThis & { __inkwellLanguageReady?: () => boolean }
        ).__inkwellLanguageReady?.(),
      ),
    )
    .toBe(true);
  await page.keyboard.press("ControlOrMeta+Space");
  await expect(page.locator(".cm-tooltip-autocomplete")).toContainText("text");
  await page.keyboard.press("Escape");
  await editor(page).fill("");
  await page.context().close();
});

test("ER-4 LSP-4: shared documentation stays inert plain text in signature help", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await editor(page).fill(
    '/// <img src=x onerror="window.l3Attack=1">\n#let l3demo(value) = value\n#l3demo(',
  );
  await page.keyboard.press("ControlOrMeta+End");
  // Help is explicitly requested, as a person does while writing a call.
  await expect
    .poll(() =>
      page.evaluate(() =>
        (
          globalThis as typeof globalThis & { __inkwellLanguageReady?: () => boolean }
        ).__inkwellLanguageReady?.(),
      ),
    )
    .toBe(true);
  await page.keyboard.press("ControlOrMeta+Shift+Space");
  const help = page.locator(".writing-help");
  await expect(help).toContainText("l3demo");
  await expect(help).toContainText("<img");
  await expect(help.locator("img,script")).toHaveCount(0);
  expect(
    await page.evaluate(() => (globalThis as typeof globalThis & { l3Attack?: number }).l3Attack),
  ).toBeUndefined();
  await page.keyboard.press("Escape");
  await editor(page).fill("");
  await page.context().close();
});
