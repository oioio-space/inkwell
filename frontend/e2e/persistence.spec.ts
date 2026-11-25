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
  // biome-ignore lint/performance/useDomNodeTextContent: the assertion is about the text as rendered (innerText), not the raw DOM text
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

/** What the page says once a fresher page took the store (RT1 rule C-2), as the catalogue words it. */
const openElsewhere: string = JSON.parse(
  readFileSync(join(import.meta.dirname, "..", "src", "i18n", "fr.json"), "utf8"),
)["store.openElsewhere"];

/** The root project.json records: the document the project is. */
const recordedRoot = (): string =>
  JSON.parse(readFileSync(join(mainTyp(), "..", ".inkwell", "project.json"), "utf8")).rootUrl;

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

test("RT1-E2E a page replaced by a fresher one stops and the saved document stays the project's root", async ({
  browser,
}) => {
  // Rules C-1 and C-2 end to end (plan 2026-11-25-rt1-root-kept.md, E-1): the first page refreshes its file list
  // every 4 s; once a fresher page took the store, it is told so and never opens the store stream again, so the
  // fresher page's start is never cut and the saved document stays the project's root.
  const text = `= Racine gardée ${Date.now()}`;
  const first = await (await browser.newContext({ locale: "fr-FR" })).newPage();
  await first.goto(`http://127.0.0.1:${PORT}/`);
  await expect(editor(first)).toBeVisible();
  await type(first, text);
  await expect.poll(onDisk, { timeout: 10_000 }).toBe(text);
  const root = recordedRoot();
  expect(root).toMatch(/^automerge:/);

  // Each store stream a page opens is one « open » frame (kind 1) naming it (@wailsio/runtime, stream.js).
  let opens = 0;
  first.on("request", (r) => {
    const h = r.headers();
    if (
      r.url().endsWith("/wails/stream/send") &&
      h["x-wails-stream-name"] === "store" &&
      h["x-wails-stream-kind"] === "1"
    ) {
      opens++;
    }
  });

  const fresh = await (await browser.newContext({ locale: "fr-FR" })).newPage();
  await fresh.goto(`http://127.0.0.1:${PORT}/`);
  await expect(editor(fresh)).toBeVisible();
  await expect.poll(() => docText(fresh), { timeout: 10_000 }).toBe(text);
  await expect(first.locator(".toast")).toHaveText(openElsewhere);
  const seen = opens;

  // Two refreshes of the first page later: it opened no store stream, and nothing replaced the root.
  await first.waitForTimeout(9_000);
  expect(opens - seen, "C-2 the replaced page never reconnects").toBe(0);
  expect(opens, "C-2 the replaced page opened no store stream after it was replaced").toBe(0);
  expect(recordedRoot()).toBe(root);
  await expect(first.locator(".toast")).toHaveText(openElsewhere);
  // The fresher page still owns the store: what it types is saved.
  const more = `${text} suite`;
  await type(fresh, more);
  await expect.poll(onDisk, { timeout: 10_000 }).toBe(more);
  expect(recordedRoot()).toBe(root);
});
