import { mkdirSync, readFileSync, rmSync } from "node:fs";
import { join } from "node:path";
import type { Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { clear, docText, editor, open, pair, typeAtEnd } from "./helpers";
import { ALICE_PORT, ALICE_URL, BOB_PORT, BOB_URL, exportDir } from "./ports";

// The preview (task C2): the project is compiled by the embedded Typst, offline, and the pages in
// view stream to the webview; problems show in the status bar; the document exports to a PDF.
// Typst starts at the first compilation of an instance, hence the long first waits.
const SLOW = { timeout: 45_000 };

const pageImages = (p: Page) => p.locator("[data-page] img");
const indicator = (p: Page) => p.getByTestId("page-indicator");
const problems = (p: Page) => p.getByRole("button", { name: /Problèmes du document/ });
const position = (p: Page) => p.getByTestId("position");

async function write(p: Page, text: string) {
  await clear(p);
  await editor(p).click();
  await p.keyboard.type(text);
}

async function exportPdf(p: Page, port: number): Promise<{ path: string; bytes: Buffer }> {
  const dir = exportDir(port);
  rmSync(dir, { recursive: true, force: true });
  mkdirSync(dir, { recursive: true });
  await p.getByRole("button", { name: "Menu principal" }).click();
  await p.getByRole("menuitem", { name: /Exporter en PDF/ }).click();
  await expect(p.getByRole("status").filter({ hasText: "PDF enregistré" })).toBeVisible(SLOW);
  const path = join(dir, "Sans titre.pdf");
  return { path, bytes: readFileSync(path) };
}

test("PRV-100 typing updates the preview, and 'Page X sur Y' follows the document", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await write(page, "= Bonjour\nUn premier paragraphe.");
  await expect(pageImages(page).first()).toBeVisible(SLOW);
  await expect(indicator(page)).toHaveText("Page 1 sur 1");
  const before = await pageImages(page).first().getAttribute("src");
  expect(before).toMatch(/^data:image\/svg\+xml;base64,/);

  await typeAtEnd(page, " Et la suite.\n#pagebreak()\n= Deux");
  await expect(indicator(page)).toHaveText("Page 1 sur 2", SLOW);
  await expect(async () => {
    expect(await pageImages(page).first().getAttribute("src")).not.toBe(before);
  }).toPass(SLOW);
  await page.screenshot({ path: "test-results/preview-typing.png" });
  await clear(page);
});

test("PRV-101 an error shows in the status bar and jumps to its line; the preview keeps the last good page", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await write(page, "= Titre\nUne ligne correcte.");
  await expect(problems(page)).toContainText("Aucun problème", SLOW);
  await expect(pageImages(page).first()).toBeVisible(SLOW);

  await typeAtEnd(page, "\n#fonctioninconnue()");
  await expect(problems(page)).toHaveAttribute("aria-label", /1 erreur/, SLOW);
  await expect(page.locator(".notice")).toContainText("dernière version qui fonctionnait");
  await expect(pageImages(page).first()).toBeVisible(); // still the last good version

  // Away from the error, then to it through the list.
  await editor(page).click();
  await page.keyboard.press("ControlOrMeta+Home");
  await expect(position(page)).toHaveText("Ligne 1, colonne 1");
  await problems(page).click();
  const entry = page.getByRole("menuitem", { name: /unknown variable/ });
  await expect(entry).toContainText("main.typ, ligne 3");
  await page.screenshot({ path: "test-results/preview-problems.png" });
  await entry.click();
  await expect(position(page)).toHaveText("Ligne 3, colonne 2");
  await expect(editor(page)).toBeFocused();

  // Fixing it clears the count.
  await page.keyboard.press("Shift+Home");
  await page.keyboard.press("Delete");
  await expect(problems(page)).toContainText("Aucun problème", SLOW);
  await expect(page.locator(".notice")).toHaveCount(0);
  await clear(page);
});

test("PRV-102 'Exporter en PDF' writes a valid PDF with its pdf.attach files", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await write(
    page,
    '= Rapport\nVoir la pièce jointe.\n#pdf.attach("notes.txt", bytes("bonjour pièce jointe"), description: "Mes notes")',
  );
  await expect(problems(page)).toContainText("Aucun problème", SLOW);

  const { bytes } = await exportPdf(page, ALICE_PORT);
  const text = bytes.toString("latin1");
  expect(text.startsWith("%PDF-")).toBe(true);
  expect(text.trimEnd().endsWith("%%EOF")).toBe(true);
  expect(text).toMatch(/\/Type\s*\/Catalog/);
  expect(text).toMatch(/\/Type\s*\/Filespec/);
  expect(text).toMatch(/\/Type\s*\/EmbeddedFile/);
  expect(text).toContain("notes.txt");
  await clear(page);
});

test("PRV-103 an export of a document with errors is refused, with the reason", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await write(page, "= Titre\n#inconnu()");
  await expect(problems(page)).toHaveAttribute("aria-label", /1 erreur/, SLOW);
  rmSync(exportDir(ALICE_PORT), { recursive: true, force: true });
  await page.getByRole("button", { name: "Menu principal" }).click();
  await page.getByRole("menuitem", { name: /Exporter en PDF/ }).click();
  await expect(
    page.getByRole("alert").filter({ hasText: "corrigez-les avant d'exporter" }),
  ).toBeVisible();
  await clear(page);
});

test("PRV-104 offline: a bundled package compiles, a missing one says why, and the About dialog tells how", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await write(page, '#import "@preview/cmarker:0.1.10"\n#cmarker.render("# Bonjour *hors ligne*")');
  await expect(pageImages(page).first()).toBeVisible(SLOW);
  await expect(problems(page)).toContainText("Aucun problème");

  await write(page, '#import "@preview/paquet-absent:1.2.3": *');
  const empty = page.getByTestId("preview-empty").or(page.locator(".notice"));
  await expect(empty).toContainText("@preview/paquet-absent:1.2.3", SLOW);
  await expect(empty).toContainText("ne télécharge jamais");
  await page.screenshot({ path: "test-results/preview-package.png" });

  await page.getByRole("button", { name: "Menu principal" }).click();
  await page.getByRole("menuitem", { name: "À propos" }).click();
  const dialog = page.getByRole("dialog", { name: "À propos" });
  const mode = dialog.getByTestId("network-mode");
  await expect(mode).toHaveAttribute("data-mode", /^(namespace|proxy only)$/, SLOW);
  await expect(mode).toContainText(/espace isolé|aucun accès au réseau/);
  await page.screenshot({ path: "test-results/preview-about.png" });
  await dialog.getByRole("button", { name: "Fermer" }).click();
  await clear(page);
});

test("PRV-105 jumps both ways between the source and the preview", async ({ browser }) => {
  const page = await open(browser, ALICE_URL);
  await write(
    page,
    "= Premier\nTexte du premier chapitre.\n#pagebreak()\n= Deuxième\nTexte du deuxième chapitre.\n#pagebreak()\n= Troisième\nTexte du troisième chapitre.",
  );
  await expect(indicator(page)).toHaveText("Page 1 sur 3", SLOW);

  // Caret in the third chapter, then "go to the cursor": the preview scrolls there and marks it.
  await editor(page).click();
  await page.keyboard.press("ControlOrMeta+End");
  await page.getByRole("button", { name: "Aller à l'endroit du curseur" }).click();
  await expect(page.locator("[data-page='3'] .mark")).toBeVisible(SLOW);
  await expect(indicator(page)).toHaveText("Page 3 sur 3");

  // A click on page 1 puts the caret in the first chapter.
  await page.getByTestId("pages").evaluate((el) => el.scrollTo({ top: 0 }));
  await expect(indicator(page)).toHaveText("Page 1 sur 3");
  const first = page.locator("[data-page='1'] button");
  const box = await first.boundingBox();
  if (!box) throw new Error("page 1 has no box");
  await page.mouse.click(box.x + box.width / 3, box.y + 140); // in the paragraph under the title
  await expect(position(page)).toHaveText(/^Ligne 2, colonne 1$/, SLOW);

  // The same from the keyboard: Ctrl+Alt+J.
  await page.keyboard.press("ControlOrMeta+End");
  await page.keyboard.press("ControlOrMeta+Alt+j");
  await expect(page.locator("[data-page='3'] .mark")).toBeVisible(SLOW);
  await clear(page);
});

test("PRV-106 a long document draws only the pages near the view, and the rest as you scroll", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await write(page, "#for i in range(40) [Page #i #pagebreak(weak: true)]");
  await expect(indicator(page)).toHaveText(/^Page 1 sur (40|41)$/, SLOW);
  await expect(pageImages(page).first()).toBeVisible();
  const total = Number.parseInt(
    ((await indicator(page).textContent()) ?? "").split("sur")[1] ?? "0",
    10,
  );
  expect(await page.locator("[data-page]").count()).toBeLessThan(10);

  await page.getByTestId("pages").evaluate((el) => el.scrollTo({ top: el.scrollHeight }));
  await expect(indicator(page)).toHaveText(`Page ${total} sur ${total}`);
  await expect(page.locator(`[data-page='${total}'] img`)).toBeVisible(SLOW);
  expect(await page.locator("[data-page]").count()).toBeLessThan(10);

  // Zoom changes the size of the pages, not their number.
  const percent = page.getByTestId("zoom");
  const fit = await percent.textContent();
  await page.getByRole("button", { name: "Agrandir" }).click();
  expect(await percent.textContent()).not.toBe(fit);
  await page.getByRole("button", { name: "Ajuster à la largeur" }).click();
  await expect(percent).toHaveText(fit ?? "");
  await clear(page);
});

test("PRV-107 hiding the preview keeps the problems in the status bar, and the choice is remembered", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await write(page, "= Titre\n#inconnu()");
  await expect(problems(page)).toHaveAttribute("aria-label", /1 erreur/, SLOW);
  await page.getByRole("button", { name: "Masquer l'aperçu" }).click();
  await expect(page.getByRole("region", { name: "Aperçu" })).toHaveCount(0);
  await expect(indicator(page)).toHaveCount(0);
  await expect(problems(page)).toHaveAttribute("aria-label", /1 erreur/);
  await page.reload();
  await expect(editor(page)).toBeVisible();
  await expect(page.getByRole("region", { name: "Aperçu" })).toHaveCount(0);
  await page.getByRole("button", { name: "Afficher l'aperçu" }).click();
  await expect(page.getByRole("region", { name: "Aperçu" })).toBeVisible();
  await clear(page);
});

test("PRV-108 two people: Bob's edit updates Alice's preview, and both exports carry the same date", async ({
  browser,
}) => {
  const alice = await open(browser, ALICE_URL);
  const bob = await open(browser, BOB_URL);
  await clear(alice);
  await clear(bob);
  await pair(alice, bob);

  await typeAtEnd(alice, "= Titre\nTexte écrit par Alice.");
  await expect(indicator(alice)).toHaveText("Page 1 sur 1", SLOW);
  await expect(indicator(bob)).toHaveText("Page 1 sur 1", SLOW);

  await expect(pageImages(alice).first()).toBeVisible(SLOW);
  await typeAtEnd(bob, "\n#pagebreak()\n= Écrit par Bob");
  await expect.poll(() => docText(alice)).toContain("Écrit par Bob");
  await expect(indicator(alice)).toHaveText("Page 1 sur 2", SLOW);
  await expect(indicator(bob)).toHaveText("Page 1 sur 2", SLOW);
  await expect(alice.locator("[data-page='2']")).toBeVisible();
  await alice.screenshot({ path: "test-results/preview-alice.png" });
  await bob.screenshot({ path: "test-results/preview-bob.png" });

  // The date inside the document is the time of the last change, the same for everyone.
  const a = await exportPdf(alice, ALICE_PORT);
  const b = await exportPdf(bob, BOB_PORT);
  const date = (pdf: Buffer) => pdf.toString("latin1").match(/\/CreationDate\s*\(D:(\d{14})/)?.[1];
  expect(date(a.bytes)).toBeDefined();
  expect(date(a.bytes)).toBe(date(b.bytes));
  expect(date(a.bytes)).not.toBe("20000101000000"); // not the default date: it comes from the change

  await clear(alice);
  await expect.poll(() => docText(bob)).toBe("");
});
