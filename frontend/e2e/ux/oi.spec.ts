import { existsSync, mkdirSync, mkdtempSync, readdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { Browser, Page } from "@playwright/test";
import { expect, test } from "../fixtures";
import { editor } from "../helpers";
import { launchAgain, startInstance, stopInstance } from "../instance";
import { OFFLINE_BOB_PORT, SPARE_PORT, SPARE_URL } from "../ports";

// Walkthrough of block OI (single instance, opening files): screenshots in UX_OI_SHOTS, light and dark.
const shots = process.env.UX_OI_SHOTS ?? join(tmpdir(), "ux-oi");
mkdirSync(shots, { recursive: true });

const fresh = () => {
  const root = mkdtempSync(join(tmpdir(), "inkwell-ux-oi-"));
  return { home: join(root, "home"), docs: join(root, "docs") };
};
const projects = (docs: string) =>
  readdirSync(docs).filter((d) => existsSync(join(docs, d, ".inkwell", "project.json")));

async function open(browser: Browser, scheme: "light" | "dark"): Promise<Page> {
  const page = await (await browser.newContext({ locale: "fr-FR", colorScheme: scheme })).newPage();
  await page.goto(SPARE_URL);
  await expect(editor(page)).toBeVisible();
  return page;
}

for (const scheme of ["light", "dark"] as const) {
  test(`UX-OI walkthrough ${scheme}`, async ({ browser }) => {
    test.fixme(
      true,
      "R-E2E1-2 (docs/plans/2026-11-17-e2e1.md): the R19 step reads an earlier toast",
    );
    const { home, docs } = fresh();
    const primary = await startInstance(SPARE_PORT, docs, { home });
    try {
      const page = await open(browser, scheme);
      await page.screenshot({ path: join(shots, `00-start-${scheme}.png`) });
      const dir = join(docs, projects(docs)[0] as string);
      const second = (args: string[], cwd = dir) =>
        launchAgain(OFFLINE_BOB_PORT, docs, { home, args, cwd, env: { LC_ALL: "fr_FR.UTF-8" } });

      // R3: a file of the project.
      writeFileSync(join(dir, "notes.md"), "# Notes OI\n");
      const r3 = await second(["notes.md"]);
      expect(r3.code).toBe(0);
      await expect(page.locator(".ttl small")).toHaveText("notes.md");
      await page.screenshot({ path: join(shots, `01-R3-file-${scheme}.png`) });

      // R5/R6: a loose file.
      const loose = join(mkdtempSync(join(tmpdir(), "ux-loose-")), "brouillon.md");
      writeFileSync(loose, "x");
      await second([loose]);
      await expect(page.locator(".toast")).toContainText("n'est dans aucun projet");
      await page.screenshot({ path: join(shots, `02-R6-loose-${scheme}.png`) });

      // R5: a file of another project (under Docs).
      const other = join(docs, "Autre");
      mkdirSync(join(other, ".inkwell"), { recursive: true });
      writeFileSync(join(other, ".inkwell", "project.json"), "{}");
      writeFileSync(join(other, "autre.md"), "x");
      await second([join(other, "autre.md")]);
      await page.waitForTimeout(500);
      await page.screenshot({ path: join(shots, `03-R5-other-${scheme}.png`) });
      console.log(`R5 ${scheme}: ${await page.locator(".toast").allTextContents()}`);

      // R19: cannot show.
      writeFileSync(join(dir, ".cache.md"), "x");
      await second([".cache.md"]);
      await expect(page.locator(".toast")).toContainText("n'a pas pu ouvrir");
      await page.screenshot({ path: join(shots, `04-R19-failed-${scheme}.png`) });

      // R8: several args, one in the project, one missing, one loose.
      writeFileSync(join(dir, "a.md"), "# A\n");
      await second(["a.md", "absent.md", loose]);
      await page.waitForTimeout(500);
      await page.screenshot({ path: join(shots, `05-R8-several-${scheme}.png`) });
      console.log(`R8 ${scheme}: ${await page.locator(".toast").allTextContents()}`);

      // Burst of three loose files in a row (hurried user): how many toasts are seen?
      const looseDir = mkdtempSync(join(tmpdir(), "ux-burst-"));
      const names = ["un.md", "deux.md", "trois.md"];
      for (const n of names) writeFileSync(join(looseDir, n), "x");
      await Promise.all(names.map((n) => second([join(looseDir, n)])));
      await page.waitForTimeout(800);
      await page.screenshot({ path: join(shots, `06-burst-${scheme}.png`) });
      console.log(`BURST ${scheme}: ${await page.locator(".toast").allTextContents()}`);

      // Keyboard: the toast has a close button reachable by Tab.
      const close = page.locator(".toast button");
      console.log(
        `CLOSE ${scheme}: ${await close.count()} ${await close.first().getAttribute("aria-label")}`,
      );
      // Narrow window: the long toast must fit.
      await page.setViewportSize({ width: 360, height: 640 });
      await page.waitForTimeout(300);
      await page.screenshot({ path: join(shots, `07-narrow-${scheme}.png`) });
      const box = await page.locator(".toast").first().boundingBox();
      console.log(`NARROW ${scheme}: ${JSON.stringify(box)}`);
    } finally {
      await stopInstance(primary);
    }
  });
}
