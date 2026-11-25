// Block EX0b1 (docs/plans/2026-11-25-ex0b-commands.md): the commands, menus and keybindings of the
// registry, in the real application. E1: keys through the one listener (the last keystroke saved, a held
// key, a field, the grid, the diff view); E2: every control bound to a command names one and shows its label.
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import type { Page } from "@playwright/test";
import { docsRoot, mainTypIn, openWith, readDisk } from "./disk";
import { expect, test } from "./fixtures";
import { clear, docText, editor, open, pair, typeAtEnd } from "./helpers";
import { ALICE_PORT, ALICE_URL, BOB_URL } from "./ports";
import { grid, openSheet } from "./sheetHelpers";

/** The text of Alice's main.typ on disk; empty until its project folder exists. */
const onDisk = () => {
  try {
    return readDisk(mainTypIn(join(docsRoot, String(ALICE_PORT)))) ?? "";
  } catch {
    return "";
  }
};
const saves = (p: Page) =>
  p.evaluate(() =>
    (globalThis as typeof globalThis & { __inkwellSaves: () => number }).__inkwellSaves(),
  );

test.beforeEach(() => test.setTimeout(120_000));

test("EX0b1-E1 keys run their commands: the last letter saved, a held Ctrl+S once, Ctrl+B in the text only, F5, Ctrl+F in a grid, F7 held", async ({
  browser,
}) => {
  const page = await openWith(browser, ALICE_URL, { saveMode: "manual" });
  await clear(page);
  // A letter typed then Ctrl+S at once: the letter is in the file.
  await typeAtEnd(page, "= Titre\nun mot");
  await page.keyboard.type("z");
  await page.keyboard.press("ControlOrMeta+s");
  await expect.poll(() => onDisk()).toMatch(/un motz\s*$/);

  // Held for a second, Ctrl+S saves once: the repeats are taken and run nothing.
  await typeAtEnd(page, " encore");
  const before = await saves(page);
  await page.keyboard.down("Control");
  for (let i = 0; i < 20; i++) {
    await page.keyboard.down("s");
    await page.waitForTimeout(50);
  }
  await page.keyboard.up("s");
  await page.keyboard.up("Control");
  expect(await saves(page)).toBe(before + 1);
  await expect.poll(() => onDisk()).toMatch(/un motz encore\s*$/);

  // Ctrl+B bolds in the text, and does nothing in a text field.
  await page.keyboard.press("Shift+Home");
  await page.keyboard.press("ControlOrMeta+b");
  await expect(editor(page)).toContainText("*un motz encore*");
  // A text field (the name in « Partager »): Ctrl+B is the field's, the registry leaves it.
  await page.getByRole("button", { name: "Partager", exact: true }).click();
  const field = page.getByLabel(/Votre nom/);
  await field.fill("Alice");
  await field.focus();
  await page.keyboard.press("ControlOrMeta+b");
  await expect(field).toHaveValue("Alice");
  await page.keyboard.press("Escape");
  expect(await docText(page)).toContain("*un motz encore*");
  expect(await docText(page)).not.toContain("**");

  // F5 opens the presenter view.
  await editor(page).click();
  await page.keyboard.press("F5");
  await expect(page.getByTestId("presenter-view")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByTestId("presenter-view")).toBeHidden();
  await page.context().close();

  // Ctrl+F in a CSV grid opens its search bar.
  const sheet = await openSheet(browser, ALICE_URL);
  await grid(sheet).focus();
  await sheet.keyboard.press("ControlOrMeta+f");
  await expect(
    sheet.getByRole("search", { name: "Chercher et remplacer dans les cellules" }),
  ).toBeVisible();
  await sheet.context().close();

  // F7 held in a difference steps from change to change more than once.
  const alice = await open(browser, ALICE_URL);
  const bob = await open(browser, BOB_URL, true);
  await clear(alice);
  await clear(bob);
  await pair(alice, bob);
  const gap = "\n".repeat(14);
  await typeAtEnd(alice, `un${gap}deux${gap}trois\n`);
  await expect.poll(() => docText(bob), { timeout: 20_000 }).toContain("trois");
  await editor(bob).click();
  await bob.keyboard.press("ControlOrMeta+Home");
  await bob.keyboard.type("A ");
  // Three changes far apart: one at each word.
  for (let w = 0; w < 2; w++) {
    for (let i = 0; i < 15; i++) await bob.keyboard.press("ArrowDown");
    await bob.keyboard.press("End");
    await bob.keyboard.type(" B");
  }
  await expect.poll(() => docText(alice), { timeout: 20_000 }).toContain("trois B");
  const drawer = alice.getByTestId("history-drawer");
  if (!(await drawer.isVisible())) await alice.getByTestId("history-toggle").click();
  await alice
    .getByRole("listbox", { name: "Moments de l'historique" })
    .getByRole("option", { name: /^Bob/ })
    .first()
    .click({ timeout: 20_000 });
  await alice.getByRole("button", { name: "Voir les changements" }).click();
  const info = alice.getByTestId("diff-info");
  await expect(info).toContainText("Changement 1 sur 3");
  await alice.keyboard.down("F7");
  await alice.waitForTimeout(100);
  await alice.keyboard.down("F7");
  await alice.keyboard.up("F7");
  await expect(info).toContainText("Changement 3 sur 3");
});

test("EX0b1-E2 every control bound to a command names one of the built-ins and shows its label", async ({
  browser,
}) => {
  const root = join(process.cwd(), "src");
  const ids = new Map<string, string>();
  for (const dir of readdirSync(join(root, "ext"))) {
    const m = JSON.parse(readFileSync(join(root, "ext", dir, "inkwell-plugin.json"), "utf8")) as {
      contributes: { commands?: { id: string; label: string }[] };
    };
    for (const c of m.contributes.commands ?? []) ids.set(c.id, c.label);
  }
  const fr = JSON.parse(readFileSync(join(root, "i18n", "fr.json"), "utf8")) as Record<
    string,
    string
  >;
  const page = await open(browser, ALICE_URL);
  await expect(editor(page)).toBeVisible();
  // The six buttons of the header bar, and the main menu's items.
  const header = page.locator("header.hb");
  const buttons = header.locator("button:not([aria-haspopup])");
  await expect(buttons).toHaveCount(6);
  for (const b of await buttons.all()) await expect(b).toHaveAttribute("data-cmd", /\S/);
  await page.getByRole("button", { name: "Menu principal" }).click();
  const items = page.getByRole("menu", { name: "Menu principal" }).locator('[role^="menuitem"]');
  await expect(items.first()).toBeVisible();
  expect(await items.count()).toBeGreaterThanOrEqual(7);
  for (const i of await items.all()) await expect(i).toHaveAttribute("data-cmd", /\S/);
  // Every [data-cmd] of the page names a built-in command and shows its French label.
  const controls = await page.locator("[data-cmd]").all();
  expect(controls.length).toBeGreaterThanOrEqual(20);
  for (const c of controls) {
    const id = (await c.getAttribute("data-cmd")) as string;
    expect(ids.has(id), id).toBe(true);
    const label = fr[ids.get(id) as string] as string;
    const name = `${(await c.getAttribute("aria-label")) ?? ""} ${(await c.textContent()) ?? ""}`;
    expect(name, id).toContain(label);
  }
});
