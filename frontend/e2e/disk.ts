import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import type { Browser, Page } from "@playwright/test";
import { expect } from "./fixtures";
import { editor } from "./helpers";

/** Where the instances keep their projects (see playwright.config.ts). */
export const docsRoot = process.env.INKWELL_E2E_DOCS ?? "";

/**
 * The `main.typ` of the project an instance has open: among its project folders, the one whose
 * file was written last (an instance that joined another's project has both).
 */
export function mainTypIn(instanceDocs: string): string {
  let best = "";
  let bestTime = -1;
  for (const dir of existsSync(instanceDocs) ? readdirSync(instanceDocs) : []) {
    const file = join(instanceDocs, dir, "main.typ");
    if (!existsSync(file)) continue;
    const time = statSync(file).mtimeMs;
    if (time > bestTime) [best, bestTime] = [file, time];
  }
  expect(best, `a project with a main.typ in ${instanceDocs}`).not.toBe("");
  return best;
}

export const readDisk = (file: string): string | undefined =>
  existsSync(file) ? readFileSync(file, "utf8") : undefined;

/** Opens an instance with the saving settings already chosen, as Preferences would store them. */
export async function openWith(
  browser: Browser,
  url: string,
  prefs: {
    saveMode?: "auto" | "manual";
    saveDelay?: number;
    externalPolicy?: "ask" | "import" | "keep";
  },
): Promise<Page> {
  const context = await browser.newContext({
    locale: "fr-FR",
    viewport: { width: 1100, height: 640 },
  });
  await context.addInitScript((value) => {
    localStorage.setItem("inkwell.settings", JSON.stringify(value));
  }, prefs);
  const page = await context.newPage();
  await page.goto(url);
  await expect(editor(page)).toBeVisible();
  return page;
}

/**
 * Checks `condition` over a window of `ms` and fails as soon as it is false: for "nothing happens"
 * assertions, which no single condition can wait for.
 */
export async function holds(what: string, condition: () => boolean, ms: number): Promise<void> {
  const end = Date.now() + ms;
  do {
    expect(condition(), what).toBe(true);
    await new Promise((r) => setTimeout(r, 50));
  } while (Date.now() < end);
}

/**
 * Whether the document history Inkwell keeps in a project folder (`.inkwell/repo`) was written
 * since `since` (ms since the epoch). Not the rest of `.inkwell` (backups, project.json…): they
 * are written on their own schedule and would stand for a save that has not happened. The files are compressed once a document is long, so what was typed
 * cannot be searched for in them.
 */
export function historyWrittenSince(projectDir: string, since: number): boolean {
  const walk = (dir: string): boolean =>
    existsSync(dir) &&
    readdirSync(dir, { withFileTypes: true }).some((entry) => {
      const path = join(dir, entry.name);
      return entry.isDirectory() ? walk(path) : statSync(path).mtimeMs >= since;
    });
  return walk(join(projectDir, ".inkwell", "repo"));
}
