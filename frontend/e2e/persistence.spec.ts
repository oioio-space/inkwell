import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import type { Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { startInstance, stopInstance } from "./instance";
import { ALICE_PORT, SPARE_PORT, SPARE_URL } from "./ports";

// Alice's instance keeps its projects in <INKWELL_E2E_DOCS>/<ALICE_PORT> (see playwright.config.ts).
const PORT = ALICE_PORT;
const docsRoot = process.env.INKWELL_E2E_DOCS ?? "";
const instanceDocs = join(docsRoot, String(PORT));

const editor = (p: Page) => p.locator(".cm-content");
const docText = (p: Page) =>
  editor(p).evaluate((el) => el.innerText.replace(/ /g, " ").replace(/[​⁠\n]/g, ""));

/** The project folder the instance created, and its main.typ. */
function mainTyp(): string {
  const dirs = readdirSync(instanceDocs).filter((d) =>
    existsSync(join(instanceDocs, d, ".inkwell")),
  );
  expect(dirs, "one project folder").toHaveLength(1);
  return join(instanceDocs, dirs[0] as string, "main.typ");
}

const onDisk = () => (existsSync(mainTyp()) ? readFileSync(mainTyp(), "utf8") : undefined);

async function type(p: Page, text: string) {
  await editor(p).click();
  await p.keyboard.press("ControlOrMeta+a");
  await p.keyboard.press("Delete");
  await p.keyboard.type(text);
  await expect.poll(() => docText(p)).toBe(text);
}

test("what is typed lands in main.typ and survives a fresh browser and a second process", async ({
  browser,
}) => {
  const text = `= Titre persistant ${Date.now()}`;
  const page = await (await browser.newContext({ locale: "fr-FR" })).newPage();
  await page.goto(`http://127.0.0.1:${PORT}/`);
  await expect(editor(page)).toBeVisible();
  await type(page, text);

  // The projection follows the text, one pause after the last keystroke.
  await expect.poll(onDisk, { timeout: 10_000 }).toBe(text);

  // A browser with no storage of its own gets the document back from Go, not from the browser.
  const fresh = await (await browser.newContext({ locale: "fr-FR" })).newPage();
  await fresh.goto(`http://127.0.0.1:${PORT}/`);
  await expect(editor(fresh)).toBeVisible();
  await expect.poll(() => docText(fresh), { timeout: 10_000 }).toBe(text);

  // A second process over the same project folder: what the first persisted is all it has.
  const restarted = await startInstance(SPARE_PORT, instanceDocs);
  try {
    const again = await (await browser.newContext({ locale: "fr-FR" })).newPage();
    await again.goto(SPARE_URL);
    await expect(editor(again)).toBeVisible();
    await expect.poll(() => docText(again), { timeout: 10_000 }).toBe(text);
  } finally {
    await stopInstance(restarted);
  }
});
