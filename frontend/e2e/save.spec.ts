import { existsSync, statSync, unlinkSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import type { Page } from "@playwright/test";
import { docsRoot, historyWrittenSince, holds, mainTypIn, openWith, readDisk } from "./disk";
import { expect, test } from "./fixtures";
import { clear, docText, editor, open, pair, typeAtEnd } from "./helpers";
import { ALICE_PORT, ALICE_URL, BOB_PORT, BOB_URL } from "./ports";

// B1: when the project file is written (SAV-*) and what happens when another program changes it
// (EXT-*, X-16). The instances are real, and so is the disk: the specs read and write the
// project's main.typ in the instance's folder.

const aliceFile = () => mainTypIn(join(docsRoot, String(ALICE_PORT)));
const bobFile = () => mainTypIn(join(docsRoot, String(BOB_PORT)));

/** An instance, saving automatically after one second, taking outside changes in. */
const auto = (browser: Parameters<typeof openWith>[0]) =>
  openWith(browser, ALICE_URL, { saveMode: "auto", saveDelay: 1, externalPolicy: "import" });

/** Alice's document holds `text` and the file says so. */
async function start(page: Page, text: string) {
  await clear(page);
  await typeAtEnd(page, text);
  await expect.poll(() => readDisk(aliceFile()), { timeout: 10_000 }).toBe(text);
}

/** The file has caught up with the document: a spec before this one may still be writing its last text. */
async function fileCaughtUp(page: Page): Promise<string> {
  await expect
    .poll(async () => (readDisk(aliceFile()) ?? "").replace(/\n/g, "") === (await docText(page)), {
      timeout: 10_000,
    })
    .toBe(true);
  return readDisk(aliceFile()) ?? "";
}

const toast = (page: Page, text: string | RegExp) =>
  page.getByRole("status").filter({ hasText: text });

test("SAV-01 automatic saving writes once after the pause, never while typing", async ({
  browser,
}) => {
  const page = await auto(browser);
  await clear(page);
  await expect.poll(() => readDisk(aliceFile()), { timeout: 10_000 }).toBe("");

  const seen: (string | undefined)[] = [];
  const watcher = setInterval(() => {
    const now = readDisk(aliceFile());
    if (seen.at(-1) !== now) seen.push(now);
  }, 25);
  try {
    await editor(page).click();
    // Sixteen keys, 120 ms apart: two seconds of typing, no pause of a second anywhere.
    await page.keyboard.type("abcdefghijklmnop", { delay: 120 });
    await expect.poll(() => readDisk(aliceFile()), { timeout: 5_000 }).toBe("abcdefghijklmnop");
    await holds("no second write", () => seen.length === 2, 1_500);
  } finally {
    clearInterval(watcher);
  }
  expect(seen).toEqual(["", "abcdefghijklmnop"]);
});

test("SAV-02 SAV-04 saving with Ctrl+S: the file waits, the page says so, and switching to automatic writes at once", async ({
  browser,
}) => {
  const page = await openWith(browser, ALICE_URL, { saveMode: "manual" });
  const before = readDisk(aliceFile()) ?? "";
  await typeAtEnd(page, " manuel");
  // For longer than any automatic pause, the file keeps what it had.
  await holds("file untouched", () => readDisk(aliceFile()) === before, 2_500);
  await expect(page.getByTestId("unsaved")).toHaveText("Non enregistré");
  await expect(page.getByTestId("unsaved-dot")).toBeVisible();
  await expect(page).toHaveTitle(/^● /);

  await page.keyboard.press("ControlOrMeta+s");
  await expect.poll(() => readDisk(aliceFile())).toBe(`${before} manuel`);
  await expect(page.getByTestId("unsaved")).toBeHidden();
  await expect(page.getByTestId("unsaved-dot")).toBeHidden();
  await expect(page).not.toHaveTitle(/^● /);

  // Unsaved again; choosing "Automatiquement" in the preferences writes it without waiting.
  await typeAtEnd(page, " auto");
  await holds("file untouched", () => readDisk(aliceFile()) === `${before} manuel`, 1_500);
  await page.getByRole("button", { name: "Menu principal" }).click();
  await page.getByRole("menuitem", { name: "Préférences" }).click();
  await page.getByRole("radio", { name: "Automatiquement" }).click();
  await expect.poll(() => readDisk(aliceFile()), { timeout: 3_000 }).toBe(`${before} manuel auto`);
  await expect(page.getByTestId("unsaved")).toBeHidden();
});

test("SAV-03 closing with unwritten changes loses nothing, asks nothing, and reopens still unsaved", async ({
  browser,
}) => {
  const page = await openWith(browser, ALICE_URL, { saveMode: "manual" });
  const before = await fileCaughtUp(page);
  const typedAt = Date.now();
  await typeAtEnd(page, " pas écrit");
  await expect(page.getByTestId("unsaved")).toBeVisible();
  let asked = false;
  page.on("dialog", (d) => {
    asked = true;
    void d.dismiss();
  });
  // The store keeps the history a moment after each keystroke (automerge-repo's save debounce).
  await expect
    .poll(() => historyWrittenSince(dirname(aliceFile()), typedAt), { timeout: 5_000 })
    .toBe(true);
  await page.reload();
  await expect(editor(page)).toBeVisible();
  await expect.poll(() => docText(page)).toBe(`${before} pas écrit`.replace(/\n/g, ""));
  await expect(page.getByTestId("unsaved")).toBeVisible();
  expect(readDisk(aliceFile())).toBe(before);
  expect(asked).toBe(false);
});

test("EXT-01 EXT-12 a file changed outside is taken in, its lines are marked, Ctrl+Z and Annuler take it back", async ({
  browser,
}) => {
  const page = await auto(browser);
  await start(page, "base");

  writeFileSync(aliceFile(), "base\nvenu de dehors\n");
  await expect(toast(page, "Modifié hors d'Inkwell")).toContainText("1 ligne intégrée");
  await expect.poll(() => docText(page)).toBe("basevenu de dehors");
  await expect(page.locator(".cm-external-gutter").first()).toBeVisible();

  // Ctrl+Z takes the whole integration back in one step, and the file follows.
  await editor(page).click();
  await page.keyboard.press("ControlOrMeta+z");
  await expect.poll(() => docText(page)).toBe("base");
  await expect.poll(() => readDisk(aliceFile()), { timeout: 5_000 }).toBe("base");

  // "Annuler" after typing elsewhere removes only the integration.
  writeFileSync(aliceFile(), "base\nencore\n");
  await expect(toast(page, "Modifié hors d'Inkwell")).toContainText("intégrée");
  await expect.poll(() => docText(page)).toBe("baseencore");
  await editor(page).click();
  await page.keyboard.press("ControlOrMeta+Home");
  await page.keyboard.type("moi ");
  await toast(page, "Modifié hors d'Inkwell").getByRole("button", { name: "Annuler" }).click();
  await expect.poll(() => docText(page)).toBe("moi base");
  await expect.poll(() => readDisk(aliceFile()), { timeout: 5_000 }).toBe("moi base");
});

test("EXT-02 a change outside while the document is edited keeps both", async ({ browser }) => {
  const page = await auto(browser);
  await start(page, "un deux");
  // Typing and the outside write land together: the page types at the start while the file
  // gets a new end.
  await editor(page).click();
  await page.keyboard.press("ControlOrMeta+Home");
  writeFileSync(aliceFile(), "un deux trois\n");
  await page.keyboard.type("moi ");
  await expect.poll(() => docText(page)).toBe("moi un deux trois");
  await expect.poll(() => readDisk(aliceFile()), { timeout: 5_000 }).toBe("moi un deux trois\n");
});

test("EXT-08 a file deleted outside: the text stays and comes back only on Recréer", async ({
  browser,
}) => {
  const page = await auto(browser);
  await start(page, "je reste");
  const file = aliceFile();
  unlinkSync(file);
  const banner = page.getByRole("alert").filter({ hasText: "supprimé du disque" });
  await expect(banner).toContainText("Son texte reste dans Inkwell");
  expect(await docText(page)).toBe("je reste");

  await typeAtEnd(page, "!");
  await expect(page.getByTestId("unsaved")).toBeVisible();
  // Typing does not bring it back, however long the automatic pause.
  await holds("file still gone", () => !existsSync(file), 2_000);
  await expect(page.getByTestId("unsaved")).toBeVisible();

  await banner.getByRole("button", { name: "Recréer" }).click();
  await expect.poll(() => readDisk(file)).toBe("je reste!");
  await expect(banner).toBeHidden();
});

test("EXT-11 a file rewritten as binary is not taken in; Inkwell's version can be kept", async ({
  browser,
}) => {
  const page = await auto(browser);
  await start(page, "du texte");
  writeFileSync(aliceFile(), Buffer.from([0, 1, 2, 255, 0]));
  const banner = page.getByRole("alert").filter({ hasText: "n'est plus un texte" });
  await expect(banner).toBeVisible();
  expect(await docText(page)).toBe("du texte");
  await banner.getByRole("button", { name: "Garder la version d'Inkwell" }).click();
  await expect.poll(() => readDisk(aliceFile())).toBe("du texte");
  await expect(banner).toBeHidden();
});

test("EXT-13 with « Me demander », nothing happens until the person chooses", async ({
  browser,
}) => {
  const page = await openWith(browser, ALICE_URL, {
    saveMode: "auto",
    saveDelay: 1,
    externalPolicy: "ask",
  });
  await start(page, "ma version");

  writeFileSync(aliceFile(), "leur version\n");
  const ask = page.getByRole("alert").filter({ hasText: "modifié en dehors d'Inkwell" });
  await expect(ask).toBeVisible();
  expect(await docText(page)).toBe("ma version");
  await ask.getByRole("button", { name: "Garder la version d'Inkwell" }).click();
  await expect.poll(() => readDisk(aliceFile())).toBe("ma version");
  expect(await docText(page)).toBe("ma version");

  writeFileSync(aliceFile(), "ma version et la leur\n");
  await expect(ask).toBeVisible();
  await ask.getByRole("button", { name: "Intégrer" }).click();
  await expect.poll(() => docText(page)).toBe("ma version et la leur");
});

test("X-16 the same file changed outside on both machines converges, with no write loop", async ({
  browser,
}) => {
  const alice = await open(browser, ALICE_URL);
  const bob = await open(browser, BOB_URL);
  // The pages show what they have at once and load the stored history a moment later.
  for (const [page, file] of [
    [alice, aliceFile],
    [bob, bobFile],
  ] as const) {
    await expect
      .poll(async () => (await docText(page)) === (readDisk(file()) ?? "").replace(/\n/g, ""))
      .toBe(true);
  }
  await clear(alice);
  await clear(bob);
  await pair(alice, bob);
  await typeAtEnd(alice, "l1 l2 l3");
  await expect.poll(() => docText(bob)).toBe("l1 l2 l3");
  await expect.poll(() => readDisk(aliceFile()), { timeout: 10_000 }).toBe("l1 l2 l3");
  await expect.poll(() => readDisk(bobFile()), { timeout: 10_000 }).toBe("l1 l2 l3");

  // A program rewrites the file on each machine, a different word each.
  writeFileSync(aliceFile(), "L1 l2 l3");
  writeFileSync(bobFile(), "l1 l2 L3");
  await expect.poll(() => docText(alice), { timeout: 15_000 }).toBe("L1 l2 L3");
  await expect.poll(() => docText(bob), { timeout: 15_000 }).toBe("L1 l2 L3");
  await expect.poll(() => readDisk(aliceFile()), { timeout: 10_000 }).toBe("L1 l2 L3");
  await expect.poll(() => readDisk(bobFile()), { timeout: 10_000 }).toBe("L1 l2 L3");

  // Quiet: neither disk is written again.
  const stamp = () => [statSync(aliceFile()).mtimeMs, statSync(bobFile()).mtimeMs];
  const settled = stamp();
  await holds("no disk written", () => stamp().every((t, i) => t === settled[i]), 3_500);
  expect(readDisk(aliceFile())).toBe("L1 l2 L3");
  expect(readDisk(bobFile())).toBe("L1 l2 L3");
});
