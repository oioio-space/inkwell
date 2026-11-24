// Block EX0 (docs/plans/2026-11-24-ex0-contribution-registry.md, rule R-11): the entries of the
// built-in extensions load on demand, and a lazily loaded table opens within its budget.
import { rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import type { Page } from "@playwright/test";
import { docsRoot, mainTypIn } from "./disk";
import { expect, test } from "./fixtures";
import { editor, open } from "./helpers";
import { ALICE_PORT, ALICE_URL } from "./ports";

/** The chunks of the lazy entries, by their entry's name (Vite names a dynamic chunk after it). */
const LAZY = { grid: /\/assets\/grid-[\w-]+\.js$/, markdown: /\/assets\/markdown-[\w-]+\.js$/ };
const OTHER_LAZY = [/\/assets\/image-[\w-]+\.js$/, /\/assets\/card-[\w-]+\.js$/];

/** The project folder of the instance, made by a first save of main.typ. */
async function projectDir(page: Page): Promise<string> {
  await editor(page).fill("= EX0\n");
  await page.keyboard.press("ControlOrMeta+s");
  await expect.poll(() => mainTypIn(join(docsRoot, String(ALICE_PORT)))).toBeTruthy();
  return dirname(mainTypIn(join(docsRoot, String(ALICE_PORT))));
}

/** The editor of main.typ takes keys. */
async function mainEditable(page: Page): Promise<void> {
  await expect(page.locator(".cm-content")).toBeVisible({ timeout: 20_000 });
  await expect(page.locator(".cm-content")).toHaveAttribute("contenteditable", "true", {
    timeout: 20_000,
  });
}

/** A fixed pseudo-random table of 10 columns and exactly `bytes` bytes (seeded, so every run reads the same). */
function table(bytes: number): string {
  let seed = 20261124;
  const next = () => {
    seed = (Math.imul(seed, 1103515245) + 12345) & 0x7fffffff;
    return seed;
  };
  const header = "id,name,city,qty,price,date,code,flag,score,note\n";
  const rows: string[] = [header];
  let size = header.length;
  for (let i = 1; size < bytes; i++) {
    const n = next();
    const row = `${i},item${n % 997},city${n % 89},${n % 500},${(n % 10000) / 100},2026-${1 + (n % 12)}-${1 + (n % 28)},C${n % 7919},${n % 2 ? "yes" : "no"},${n % 101},note ${n % 31}\n`;
    rows.push(row);
    size += row.length;
  }
  const text = rows.join("");
  // Trimmed to the exact size on a row boundary, padded in the last note.
  const cut = text.lastIndexOf("\n", bytes - 1) + 1;
  const tail = bytes - cut;
  return text.slice(0, cut) + (tail > 1 ? `${"x".repeat(tail - 1)}\n` : tail === 1 ? "\n" : "");
}

test("EX0-E1 entries load on demand: main.typ loads none, a CSV its grid, a Markdown file its grammar", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  const scripts: string[] = [];
  page.on("request", (r) => {
    if (r.resourceType() === "script") scripts.push(new URL(r.url()).pathname);
  });
  const dir = await projectDir(page);
  const files = {
    "ex0-list.csv": "nom,qte\nPomme,3\nPoire,5\n",
    "ex0-notes.md": "# Notes\n\n**gras**\n",
    "ex0-stock.txt": "nom\tqte\nPomme\t3\nPoire\t5\nKiwi\t7\n",
  };
  try {
    for (const [name, text] of Object.entries(files)) writeFileSync(join(dir, name), text);
    await page.reload();
    await mainEditable(page);
    const loaded = (re: RegExp) => scripts.some((s) => re.test(s));
    expect(loaded(LAZY.grid), "grid at start").toBe(false);
    expect(loaded(LAZY.markdown), "markdown at start").toBe(false);
    for (const re of OTHER_LAZY) expect(loaded(re), `${re} at start`).toBe(false);

    await page.locator('[data-path="ex0-list.csv"]').click();
    await expect(page.getByRole("grid")).toContainText("Pomme");
    expect(loaded(LAZY.grid), "grid after the CSV").toBe(true);
    expect(loaded(LAZY.markdown), "markdown after the CSV").toBe(false);

    // W79: the table offers its text in the view bar, and the text its table, from the manifests.
    await page.getByRole("button", { name: "Voir le texte" }).first().click();
    await expect(editor(page)).toContainText("Pomme,3");
    await page.getByRole("button", { name: "Voir le tableau" }).click();
    await expect(page.getByRole("grid")).toContainText("Pomme");

    // A text that reads as a table is offered as one, and opens as one on request.
    await page.locator('[data-path="ex0-stock.txt"]').click();
    await expect(editor(page)).toContainText("Kiwi");
    await page.getByRole("button", { name: "Ouvrir comme tableau ?" }).click();
    await expect(page.getByRole("grid")).toContainText("Kiwi");

    await page.locator('[data-path="ex0-notes.md"]').click();
    await expect(editor(page)).toContainText("Notes");
    await expect
      .poll(() => loaded(LAZY.markdown), { message: "markdown after the .md" })
      .toBe(true);
    // The grammar colours the file once loaded: the heading gets its class.
    await expect(
      page.locator(".cm-content .cm-line").first().locator("span").first(),
    ).toHaveAttribute("class", /ͼ/);
  } finally {
    for (const name of Object.keys(files)) rmSync(join(dir, name), { force: true });
    await page.context().close();
  }
});

test("EX0-E2 PERF-EXT-VIEWER-CSV opens a 1 MiB table within budget", async ({ browser }) => {
  test.setTimeout(180_000);
  const page = await open(browser, ALICE_URL);
  const dir = await projectDir(page);
  const file = join(dir, "table-1mib.csv");
  const values: number[] = [];
  try {
    const text = table(1_048_576);
    expect(Buffer.byteLength(text)).toBe(1_048_576);
    writeFileSync(file, text);
    for (let run = 0; run <= 10; run++) {
      // An app already running, the grid's chunk not loaded: the page is reloaded before each run.
      await page.reload();
      await mainEditable(page);
      await page.locator('[data-path="table-1mib.csv"]').click();
      await expect(page.getByRole("grid")).toBeVisible({ timeout: 20_000 });
      const ms = await page.waitForFunction(
        () => performance.getEntriesByName("ex0:open-render")[0]?.duration,
        undefined,
        { timeout: 20_000 },
      );
      const value = (await ms.jsonValue()) as number;
      if (run > 0) values.push(value); // the first run is the warm-up
    }
    const sorted = [...values].sort((a, b) => a - b);
    // Nearest-rank p95 of 10 values: the 10th, the slowest.
    const p95 = sorted[Math.ceil(0.95 * sorted.length) - 1] ?? Number.POSITIVE_INFINITY;
    console.log(
      `PERF-EXT-VIEWER-CSV ms: ${sorted.map((v) => v.toFixed(1)).join(" ")}; p95 ${p95.toFixed(1)}`,
    );
    expect(values).toHaveLength(10);
    expect(p95).toBeLessThanOrEqual(200);
  } finally {
    rmSync(file, { force: true });
    await page.context().close();
  }
});
