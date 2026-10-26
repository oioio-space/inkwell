import type { Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { clear, docText, editor, open, pair, typeAtEnd } from "./helpers";
import { ALICE_URL, BOB_URL } from "./ports";

// B2: the margin strip, the history drawer, readable differences, variants and the text zoom,
// with two real instances.

const SHOTS =
  "/tmp/claude-1000/-home-mathieu-GolandProjects-inkwell/9e09cb9d-09c5-4b33-8b2b-0d87e3efdfbf/scratchpad/shots";
const shot = (page: Page, name: string, dark: boolean) =>
  page.screenshot({ path: `${SHOTS}/b2-${name}-${dark ? "dark" : "light"}.png` });

test.beforeEach(() => test.setTimeout(90_000));

const drawer = (p: Page) => p.getByTestId("history-drawer");
const toggle = (p: Page) => p.getByTestId("history-toggle");

async function openDrawer(p: Page) {
  if (!(await drawer(p).isVisible())) await toggle(p).click();
  await expect(drawer(p)).toBeVisible();
}

test("B2 gutter, versions, who wrote what, readable differences, going back (HIS-02..07, DIF-01, DIF-05)", async ({
  browser,
}) => {
  const alice = await open(browser, ALICE_URL);
  const bob = await open(browser, BOB_URL, true);
  await clear(alice);
  await clear(bob);
  await pair(alice, bob);
  await typeAtEnd(alice, "= Plan de continuité\nUn premier paragraphe.\n");
  await expect.poll(() => docText(bob), { timeout: 20_000 }).toContain("premier paragraphe");

  // HIS-02/03: what Bob adds is marked on Alice's side; HIS-04: what Alice types is not.
  await typeAtEnd(bob, "Ajout de Bob.\n");
  await expect.poll(() => docText(alice)).toContain("Ajout de Bob");
  await expect(alice.locator(".cm-changed").first()).toBeVisible({
    timeout: 15_000,
  });
  const before = await alice.locator(".cm-changed").count();
  await typeAtEnd(alice, "Mot de Alice.\n");
  await expect.poll(() => docText(bob)).toContain("Mot de Alice");
  await alice.waitForTimeout(1200);
  expect(await alice.locator(".cm-changed").count()).toBe(before);
  await expect(alice.getByTestId("change-strip")).toContainText("Depuis votre dernière visite");
  await expect(alice.locator(".cm-minimap-tick").first()).toBeAttached();
  await alice.locator(".cm-changed").first().hover();
  await shot(alice, "gutter", false);
  await alice.getByRole("radio", { name: "Par personne" }).click();
  await shot(alice, "gutter-person", false);

  // HIS-06: a named version shows up on the other side with its name and author.
  await openDrawer(alice);
  await openDrawer(bob);
  await alice
    .getByRole("listbox", { name: "Moments de l'historique" })
    .getByRole("option", { name: /Maintenant/ })
    .click();
  await alice.getByRole("button", { name: "Nommer cette version" }).click();
  await alice.getByLabel("Nom de la version").fill("Relecture DSI");
  await alice.getByRole("button", { name: "Enregistrer" }).click();
  const version = bob
    .getByRole("listbox", { name: "Moments de l'historique" })
    .getByRole("option", { name: /Version « Relecture DSI »/ });
  await expect(version).toBeVisible({ timeout: 15_000 });
  await expect(version).toContainText("nommée par Alice");
  await shot(alice, "drawer", false);
  await shot(bob, "drawer", true);

  // DIF-01: the entry of Bob, in plain words.
  await alice
    .getByRole("listbox", { name: "Moments de l'historique" })
    .getByRole("option", { name: /^Bob/ })
    .first()
    .click();
  await alice.getByRole("button", { name: "Voir les changements" }).click();
  const view = alice.getByTestId("diff-view");
  await expect(view).toBeVisible();
  await expect(alice.getByTestId("diff-summary")).toContainText(/Bob a ajouté 1 phrase/);
  await expect(view.locator("ins.add").first()).toContainText("Ajout de Bob");
  await shot(alice, "diff", false);
  await alice.keyboard.press("F7");
  await alice.getByRole("radio", { name: "Côte à côte" }).click();
  await expect(view.locator(".cm-mergeView")).toBeVisible();
  await shot(alice, "diff-side", false);
  await alice.keyboard.press("Escape");
  await expect(view).toBeHidden();

  // DIF-05: who wrote what, with the hover.
  await alice.getByLabel("Qui a écrit quoi").check();
  await expect(alice.locator(".cm-who").first()).toBeVisible({ timeout: 15_000 });
  await expect(alice.locator('.cm-who[title*="Bob"]').first()).toBeAttached();
  await shot(alice, "blame", false);
  await alice.getByLabel("Qui a écrit quoi").uncheck();

  // HIS-07: going back to the version: preview first, then the text of the version, on both sides.
  const full = await docText(alice);
  await typeAtEnd(alice, "Trop tard.\n");
  await alice
    .getByRole("listbox", { name: "Moments de l'historique" })
    .getByRole("option", { name: /Version « Relecture DSI »/ })
    .click();
  await alice.getByRole("button", { name: "Revenir à cette version" }).click();
  const dialog = alice.getByTestId("apply-dialog");
  await expect(dialog).toBeVisible();
  await expect(alice.getByTestId("apply-summary")).toContainText(/supprimé/);
  await shot(alice, "revert-preview", false);
  await dialog.getByRole("button", { name: "Revenir à ce moment" }).click();
  await expect.poll(() => docText(alice)).toBe(full);
  await expect.poll(() => docText(bob)).toBe(full);
  // Ctrl+Z takes it back.
  await editor(alice).click();
  await alice.keyboard.press("ControlOrMeta+z");
  await expect.poll(() => docText(alice)).toContain("Trop tard");
});

test("B2 variants: branch, edit apart, merge (HIS-10, HIS-11, HIS-16)", async ({ browser }) => {
  const alice = await open(browser, ALICE_URL);
  const bob = await open(browser, BOB_URL, true);
  await clear(alice);
  await clear(bob);
  await pair(alice, bob);
  await typeAtEnd(alice, "= Texte commun\n");
  await expect.poll(() => docText(bob)).toContain("Texte commun");
  await openDrawer(alice);
  await openDrawer(bob);
  await alice
    .getByRole("listbox", { name: "Moments de l'historique" })
    .getByRole("option", { name: /Maintenant/ })
    .click();
  await alice.getByRole("button", { name: "Créer une variante ici" }).click();
  await alice.getByLabel("Nom de la variante").fill("procédure courte");
  await alice.getByRole("button", { name: "Enregistrer" }).click();
  const row = (p: Page) =>
    p
      .getByRole("listbox", { name: "Moments de l'historique" })
      .getByRole("option", { name: /Variante « procédure courte »/ });
  await expect(row(alice)).toBeVisible();
  await expect(row(bob)).toBeVisible({ timeout: 15_000 });

  // Edited apart: the common text does not change.
  await row(alice).click();
  await alice.getByRole("button", { name: "Modifier la variante" }).click();
  await expect(alice.getByText("Vous modifiez la variante")).toBeVisible();
  await typeAtEnd(alice, "Dans la variante.\n");
  await shot(alice, "variant-editing", false);
  await alice.getByRole("button", { name: "Revenir au texte principal" }).click();
  await expect.poll(() => docText(alice)).not.toContain("Dans la variante");
  await typeAtEnd(bob, "Pendant ce temps.\n");

  // Merge: the union, on both sides.
  await row(alice).click();
  await alice.getByRole("button", { name: "Fusionner" }).click();
  await expect.poll(() => docText(alice)).toContain("Dans la variante");
  await expect.poll(() => docText(alice)).toContain("Pendant ce temps");
  await expect.poll(() => docText(bob)).toContain("Dans la variante");
  await expect(
    alice
      .getByRole("listbox", { name: "Moments de l'historique" })
      .getByRole("option", { name: /Variante « procédure courte » fusionnée/ }),
  ).toBeVisible();
  await shot(alice, "variant-merged", false);
});

test("B2 text zoom: keys, indicator, kept per project, no sideways overflow (ZOOM-01..03)", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await clear(page);
  await typeAtEnd(page, `${"un mot très long ".repeat(40)}\nfin`);
  const size = () =>
    page.locator(".cm-content").evaluate((el) => Number.parseFloat(getComputedStyle(el).fontSize));
  const indicator = page.getByTestId("text-zoom");
  await expect(indicator).toHaveText("100 %");
  const base = await size();
  await page.keyboard.press("ControlOrMeta+=");
  await expect(indicator).toHaveText("110 %");
  expect(await size()).toBeGreaterThan(base);
  await page.keyboard.press("ControlOrMeta+-");
  await page.keyboard.press("ControlOrMeta+-");
  await expect(indicator).toHaveText("90 %");
  await page.keyboard.press("ControlOrMeta+0");
  await expect(indicator).toHaveText("100 %");
  // The wheel, with Ctrl.
  await page.locator(".cm-content").hover();
  await page.keyboard.down("Control");
  await page.mouse.wheel(0, -100);
  await page.keyboard.up("Control");
  await expect(indicator).toHaveText("110 %");
  // The caret stays where it was.
  const caret = await page.getByTestId("position").textContent();
  await page.keyboard.press("ControlOrMeta+=");
  expect(await page.getByTestId("position").textContent()).toBe(caret);
  // 300 % in a narrow window: lines wrap, nothing sideways.
  for (let i = 0; i < 25; i++) await page.keyboard.press("ControlOrMeta+=");
  await expect(indicator).toHaveText("300 %");
  await page.setViewportSize({ width: 480, height: 640 });
  const overflow = await page
    .locator(".cm-scroller")
    .evaluate((el) => el.scrollWidth - el.clientWidth);
  expect(overflow).toBeLessThanOrEqual(1);
  await shot(page, "zoom-300", false);
  // ZOOM-02: kept when the project is reopened.
  await page.reload();
  await expect(page.getByTestId("text-zoom")).toHaveText("300 %");
  await page.keyboard.press("ControlOrMeta+0");
});
