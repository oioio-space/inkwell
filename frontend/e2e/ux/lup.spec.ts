import { writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import type { Locator, Page } from "@playwright/test";
import { docsRoot, mainTypIn } from "../disk";
import { expect, test } from "../fixtures";
import { clear, open, typeAtEnd } from "../helpers";
import { ALICE_PORT, ALICE_URL } from "../ports";

// Defects of the dependency-upgrade walkthrough (Claire), found by looking at the screen. Both
// predate the upgrade: the same build of main shows them.

/** Pairs of visible, non-nested elements among `items` whose boxes overlap by more than 1px. */
async function overlaps(items: Locator): Promise<string[]> {
  return items.evaluateAll((els) => {
    const boxes = els
      .map((e) => ({
        name: (e.getAttribute("aria-label") ?? e.textContent ?? "").trim(),
        r: e.getBoundingClientRect(),
        e,
      }))
      .filter((b) => b.r.width > 0 && b.r.height > 0);
    const out: string[] = [];
    for (const [i, a] of boxes.entries())
      for (const b of boxes.slice(i + 1)) {
        if (a.e.contains(b.e) || b.e.contains(a.e)) continue;
        const x = Math.min(a.r.right, b.r.right) - Math.max(a.r.left, b.r.left);
        const y = Math.min(a.r.bottom, b.r.bottom) - Math.max(a.r.top, b.r.top);
        if (x > 1 && y > 1) out.push(`« ${a.name} » / « ${b.name} »`);
      }
    return out;
  });
}

const headerItems = (p: Page) =>
  p.getByRole("banner").locator("h1, button, [role=status]").filter({ visible: true });

/** Pairs of words drawn over each other in the status bar (its items are partly bare text). */
const statusClashes = (p: Page): Promise<string[]> =>
  p.getByRole("contentinfo", { name: "Barre d'état" }).evaluate((bar) => {
    const words: { text: string; r: DOMRect }[] = [];
    const walk = document.createTreeWalker(bar, NodeFilter.SHOW_TEXT);
    for (let n = walk.nextNode(); n; n = walk.nextNode()) {
      if (!n.textContent?.trim()) continue;
      const range = document.createRange();
      range.selectNodeContents(n);
      for (const r of range.getClientRects())
        if (r.width > 0) words.push({ text: n.textContent.trim(), r });
    }
    const out: string[] = [];
    for (const [i, a] of words.entries())
      for (const b of words.slice(i + 1)) {
        const x = Math.min(a.r.right, b.r.right) - Math.max(a.r.left, b.r.left);
        const y = Math.min(a.r.bottom, b.r.bottom) - Math.max(a.r.top, b.r.top);
        if (x > 1 && y > 1) out.push(`« ${a.text} » / « ${b.text} »`);
      }
    return out;
  });

// UX-LUP-1 is the known narrow-window problem S13 (on main too, not caused by the upgrade):
// below 1300px the History drawer pushes the layout instead of covering the preview. It is
// fixed by its own lot, S13 (docs/objectif-en-cours.md, "Lots suivants"); that lot turns these
// back into `test`.
for (const width of [1100, 950]) {
  test.fixme(`UX-LUP-1 at ${width}px, opening the history covers nothing in the header and the status bar`, async ({
    browser,
  }) => {
    const page = await open(browser, ALICE_URL);
    await page.setViewportSize({ width, height: 640 });
    await page.getByRole("button", { name: "Afficher l'historique" }).click();
    await expect(page.getByRole("complementary", { name: "Historique" })).toBeVisible();
    // Spec §3.4: from 900 to 1199px the drawer covers the preview; the title, the header
    // buttons and the status bar stay readable and apart.
    await expect.poll(() => statusClashes(page)).toEqual([]);
    await expect.poll(() => overlaps(headerItems(page))).toEqual([]);
  });
}

test("UX-LUP-2 back on main.typ from another file, the caret is where it was", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await clear(page);
  await typeAtEnd(page, "= Rapport\nLe comité a décidé.\nÀ suivre");
  const bar = page.getByRole("contentinfo", { name: "Barre d'état" });
  await expect(bar).toContainText("Ligne 3, colonne 9");
  await page.keyboard.press("ControlOrMeta+s");
  const dir = dirname(mainTypIn(join(docsRoot, String(ALICE_PORT))));
  writeFileSync(join(dir, "budget.csv"), "Poste;Montant\nSalle;120\n");
  await page.locator('[data-path="budget.csv"]').click();
  await expect(page.getByRole("grid")).toBeVisible();
  await page.locator('[data-path="main.typ"]').click();
  // Spec §4.25: a file comes back "avec sa position"; typing goes on where it stopped.
  await expect(bar).toContainText("Ligne 3, colonne 9");
});
