import { existsSync, mkdirSync, mkdtempSync, readdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, test } from "../fixtures";
import { editor } from "../helpers";
import { launchAgain, startInstance, stopInstance } from "../instance";
import { OFFLINE_BOB_PORT, SPARE_PORT, SPARE_URL } from "../ports";

// Defects of the OI walkthrough (gates/OI/UX.md). Each test fails until its defect is fixed.

async function session() {
  const root = mkdtempSync(join(tmpdir(), "inkwell-ux-oi-"));
  const home = join(root, "home");
  const docs = join(root, "docs");
  const primary = await startInstance(SPARE_PORT, docs, { home });
  const project = readdirSync(docs).find((d) =>
    existsSync(join(docs, d, ".inkwell", "project.json")),
  );
  const dir = join(docs, project as string);
  const again = (args: string[], cwd = dir) =>
    launchAgain(OFFLINE_BOB_PORT, docs, { home, args, cwd, env: { LC_ALL: "fr_FR.UTF-8" } });
  return { primary, dir, again };
}

test("UX-OI-1 three loose files opened in a row are all reported, none is silently replaced", async ({
  browser,
}) => {
  const { primary, again } = await session();
  try {
    const page = await (await browser.newContext({ locale: "fr-FR" })).newPage();
    await page.goto(SPARE_URL);
    await expect(editor(page)).toBeVisible();
    const dir = mkdtempSync(join(tmpdir(), "ux-burst-"));
    const names = ["un.md", "deux.md", "trois.md"];
    for (const n of names) writeFileSync(join(dir, n), "x");
    await Promise.all(names.map((n) => again([join(dir, n)])));
    await page.waitForTimeout(1_500);
    const seen = (await page.locator(".toast").allTextContents()).join(" ");
    // Expected: one toast naming them all, or a count (« 3 fichiers … »); today only the last shows.
    const named = names.filter((n) => seen.includes(n)).length;
    expect(named === 3 || /\b3\b/.test(seen), `the toast shows: ${seen}`).toBe(true);
  } finally {
    await stopInstance(primary);
  }
});

test("UX-OI-2 several files: the toast says which one was shown and which others were not", async ({
  browser,
}) => {
  const { primary, dir, again } = await session();
  try {
    const page = await (await browser.newContext({ locale: "fr-FR" })).newPage();
    await page.goto(SPARE_URL);
    await expect(editor(page)).toBeVisible();
    writeFileSync(join(dir, "a.md"), "# A\n");
    const loose = join(mkdtempSync(join(tmpdir(), "ux-loose-")), "brouillon.md");
    writeFileSync(loose, "x");
    mkdirSync(dir, { recursive: true });
    await again(["a.md", "absent.md", loose]);
    await expect(page.locator(".toast")).toContainText("absent.md");
    // « Et 1 autre. » leaves the person guessing which file was left out. The failure toast comes
    // first (role alert); the toast naming the loose file follows it once it goes (7 s), like
    // AdwToastOverlay's queue.
    await expect(page.locator(".toast")).toContainText("brouillon.md", { timeout: 12_000 });
  } finally {
    await stopInstance(primary);
  }
});

test("UX-OI-3 a failure toast interrupts a screen reader (role alert)", async ({ browser }) => {
  const { primary, dir, again } = await session();
  try {
    const page = await (await browser.newContext({ locale: "fr-FR" })).newPage();
    await page.goto(SPARE_URL);
    await expect(editor(page)).toBeVisible();
    writeFileSync(join(dir, ".cache.md"), "x");
    await again([".cache.md"]);
    await expect(page.locator(".toast")).toContainText("n'a pas pu ouvrir");
    await expect(page.locator(".toast")).toHaveAttribute("role", "alert");
  } finally {
    await stopInstance(primary);
  }
});
