// Block EX0b2 (docs/plans/2026-11-25-ex0b2-palette.md): the command palette and the shortcuts window in
// the real application. E1: the palette from the text and from the main menu; E2: PERF-PALETTE over the
// e2e build's 1 000 fixture commands (architecture §15.4); E3: the shortcuts window.
import { rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import type { Browser, Page } from "@playwright/test";
import { docsRoot, mainTypIn } from "./disk";
import { expect, test } from "./fixtures";
import { clear, docText, editor, judgeBudget, open, typeAtEnd } from "./helpers";
import { ALICE_PORT, ALICE_URL } from "./ports";

test.beforeEach(() => test.setTimeout(120_000));

const palette = (p: Page) => p.getByRole("combobox", { name: "Rechercher une commande" });
const firstOption = (p: Page) =>
  p.getByRole("listbox", { name: "Commandes" }).getByRole("option").first();
const openPalette = (p: Page) => p.keyboard.press("Control+Shift+P");

/** The durations of a performance measure, in order. */
const measures = (p: Page, name: string) =>
  p.evaluate((n) => performance.getEntriesByName(n).map((e) => e.duration), name);

test("EX0b2-E1 the palette runs a command where the person was, from the text and from the main menu", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await clear(page);
  await typeAtEnd(page, "un mot");
  await page.keyboard.press("Control+Shift+ArrowLeft");
  await openPalette(page);
  await expect(palette(page)).toBeFocused();
  await palette(page).fill("gras");
  await expect(firstOption(page)).toContainText("Gras");
  await page.keyboard.press("Enter");
  await expect(palette(page)).toBeHidden();
  await expect(editor(page)).toBeFocused();
  await expect.poll(() => docText(page)).toBe("un *mot*");

  // From the main menu: the target is the text it came from.
  await typeAtEnd(page, " deux");
  await page.keyboard.press("Control+Shift+ArrowLeft");
  await page.getByRole("button", { name: "Menu principal" }).click();
  await page.getByRole("menuitem", { name: "Rechercher une commande…" }).click();
  await palette(page).fill("gras");
  await page.keyboard.press("Enter");
  await expect(editor(page)).toBeFocused();
  await expect.poll(() => docText(page)).toBe("un *mot* *deux*");

  // Reopened, the last command run is first without a query; a synonym finds its command.
  await openPalette(page);
  await expect(firstOption(page)).toContainText("Gras");
  // The keys keep the active row in view.
  for (let i = 0; i < 15; i++) await page.keyboard.press("ArrowDown");
  await expect(page.locator('[role="option"][aria-selected="true"]')).toBeInViewport();
  await palette(page).fill("réglages");
  await expect(firstOption(page)).toContainText("Préférences");
  await page.keyboard.press("Escape");
  await expect(palette(page)).toBeHidden();
  await expect(editor(page)).toBeFocused();

  // In a Markdown file, presenting is greyed: Enter says why and the palette stays.
  await editor(page).fill("= EX0b2\n");
  await page.keyboard.press("ControlOrMeta+s");
  await expect.poll(() => mainTypIn(join(docsRoot, String(ALICE_PORT)))).toBeTruthy();
  const notes = join(dirname(mainTypIn(join(docsRoot, String(ALICE_PORT)))), "ex0b2-notes.md");
  writeFileSync(notes, "# Notes\n\nun texte\n");
  try {
    await page.reload();
    await page.locator('[data-path="ex0b2-notes.md"]').click();
    await expect(editor(page)).toContainText("un texte");
    await editor(page).click();
    await openPalette(page);
    await palette(page).fill("présenter");
    const present = firstOption(page);
    await expect(present).toContainText("Présenter");
    await expect(present).toHaveAttribute("aria-disabled", "true");
    await expect(present).toHaveAccessibleDescription("Seul un document Typst se présente");
    await page.keyboard.press("Enter");
    await expect(page.locator("dialog [role=status]")).toHaveText(
      "Seul un document Typst se présente",
    );
    await expect(palette(page)).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(palette(page)).toBeHidden();
    await expect(editor(page)).toBeFocused();
  } finally {
    rmSync(notes, { force: true });
  }
});

test("EX0b2-E2 PERF-PALETTE opens and filters 1 000 commands within budget", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await editor(page).click();
  const opens: number[] = [];
  for (let i = 0; i < 11; i++) {
    const before = (await measures(page, "ex0b2:palette-open")).length;
    await openPalette(page);
    await expect(firstOption(page)).toBeVisible();
    await expect
      .poll(async () => (await measures(page, "ex0b2:palette-open")).length)
      .toBe(before + 1);
    opens.push((await measures(page, "ex0b2:palette-open")).at(-1) as number);
    if (i === 0) {
      // The e2e build's fixture is there: more than 1 000 commands to rank.
      const count = async () =>
        Number((await page.locator("dialog [role=status]").textContent())?.replace(/\D/g, ""));
      await expect.poll(count).toBeGreaterThan(1000);
    }
    await page.keyboard.press("Escape");
    await expect(palette(page)).toBeHidden();
  }
  await openPalette(page);
  const queries: number[] = [];
  for (const letter of "abcdefghij") {
    const before = (await measures(page, "ex0b2:palette-filter")).length;
    await palette(page).fill(letter);
    await expect
      .poll(async () => (await measures(page, "ex0b2:palette-filter")).length)
      .toBe(before + 1);
    queries.push((await measures(page, "ex0b2:palette-filter")).at(-1) as number);
  }
  await page.keyboard.press("Escape");
  console.log(`PERF-PALETTE opens (ms): ${opens.map((d) => d.toFixed(1)).join(" ")}`);
  console.log(`PERF-PALETTE queries (ms): ${queries.map((d) => d.toFixed(1)).join(" ")}`);
  // Judged by the gate's exclusive e2e and e2e-webkit criteria (gates/EX0b2.toml), reported on a
  // shared runner (`judgeBudget`, convention of b240aeb): nothing is weakened where it is judged.
  const warm = Math.max(...opens.slice(1));
  const slowest = Math.max(...queries);
  judgeBudget(`first ${opens[0]} ms, warm ${warm} ms, query ${slowest} ms in a shared slot`, () => {
    expect(opens[0], "first open").toBeLessThanOrEqual(200);
    // One warm-up, then the slowest of ten warm opens and of ten one-letter queries.
    expect(warm, "warm opens").toBeLessThanOrEqual(50);
    expect(slowest, "one-letter queries").toBeLessThanOrEqual(50);
  });
});

/** Opens Alice's page with a keybinding of hers stored on this device. */
async function withBinding(browser: Browser): Promise<Page> {
  const context = await browser.newContext({
    locale: "fr-FR",
    viewport: { width: 1100, height: 640 },
  });
  await context.addInitScript(() => {
    localStorage.setItem(
      "inkwell.keybindings",
      JSON.stringify([{ command: "app.about", key: "Ctrl+Shift+F1" }]),
    );
  });
  const page = await context.newPage();
  await page.goto(ALICE_URL);
  await expect(editor(page)).toBeVisible();
  return page;
}

test("EX0b2-E3 the shortcuts window lists the chords in force, hers marked, and filters", async ({
  browser,
}) => {
  const page = await withBinding(browser);
  await editor(page).click();
  await page.keyboard.press("Control+?");
  const win = page.getByRole("dialog", { name: "Raccourcis clavier" });
  await expect(win).toBeVisible();
  const row = (id: string) => win.locator(`[data-shortcut="${id}"]`);
  await expect(row("format.bold")).toContainText("Gras");
  await expect(row("format.bold")).toContainText("Ctrl+B");
  await expect(win.getByRole("heading", { name: "Écrire" })).toBeVisible();
  await expect(row("keys.select-all")).toContainText("géré par l'éditeur");
  await expect(row("app.about")).toContainText("Ctrl+Maj+F1");
  await expect(row("app.about")).toContainText("personnalisé");
  await win.getByRole("searchbox", { name: "Rechercher un raccourci" }).fill("zoom");
  await expect(win.locator("[data-shortcut]")).toHaveCount(3);
  await expect(row("app.zoom-in")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(win).toBeHidden();
  await expect(editor(page)).toBeFocused();
});
