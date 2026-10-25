import { type ChildProcess, spawn } from "node:child_process";
import { existsSync, mkdtempSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { expect, type Page, test } from "@playwright/test";
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

  // The projection follows the text, 300 ms after the last keystroke.
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
    restarted.kill("SIGTERM");
  }
});

test("an edit of main.typ outside Inkwell is offered for import, or ignored", async ({
  browser,
}) => {
  const mine = `= Mon texte ${Date.now()}`;
  const page = await (await browser.newContext({ locale: "fr-FR" })).newPage();
  await page.goto(`http://127.0.0.1:${PORT}/`);
  await expect(editor(page)).toBeVisible();
  await type(page, mine);
  await expect.poll(onDisk, { timeout: 10_000 }).toBe(mine);

  // Import: the outside text becomes the document.
  const theirs = `= Édité ailleurs ${Date.now()}`;
  writeFileSync(mainTyp(), theirs);
  const toast = page.getByRole("alert");
  await expect(toast).toContainText("main.typ a été modifié en dehors d'Inkwell");
  await toast.getByRole("button", { name: "Importer" }).click();
  await expect(toast).toBeHidden();
  await expect.poll(() => docText(page)).toBe(theirs);
  expect(onDisk()).toBe(theirs);

  // Ignore: the document wins and the file is written from it again.
  writeFileSync(mainTyp(), "= Autre modification");
  await expect(toast).toBeVisible();
  await toast.getByRole("button", { name: "Ignorer" }).click();
  await expect(toast).toBeHidden();
  await expect.poll(onDisk, { timeout: 10_000 }).toBe(theirs);
  expect(await docText(page)).toBe(theirs);
});

/** Starts another inkwell-server over `docs` and waits until it answers. */
async function startInstance(port: number, docs: string): Promise<ChildProcess> {
  const child = spawn(resolve(import.meta.dirname, "../../bin/inkwell-server"), [], {
    env: {
      ...process.env,
      INKWELL_SERVER_PORT: String(port),
      INKWELL_HOME: mkdtempSync(join(tmpdir(), "inkwell-e2e-home-")),
      INKWELL_DOCS: docs,
    },
    stdio: "ignore",
  });
  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    try {
      if ((await fetch(`http://127.0.0.1:${port}/health`)).ok) return child;
    } catch {
      // not listening yet
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  child.kill("SIGKILL");
  throw new Error(`instance on port ${port} did not start`);
}
