import { expect, test } from "../fixtures";
import { clear, open, pair } from "../helpers";
import { ALICE_URL, BOB_URL } from "../ports";

// Defects of the SI1b walkthrough (honest sync indicator), found by looking at the screen.
// Both are expected to FAIL until fixed (block SI1b ux-walkthrough, gates/SI1b/UX.md).

/** Records every change of the header's status text, with the page clock. */
const record = (page: import("@playwright/test").Page) =>
  page.evaluate(() => {
    const el = document.querySelector('[role="status"]') as HTMLElement;
    const log: [number, string][] = [[0, el.textContent?.trim() ?? ""]];
    const t0 = performance.now();
    new MutationObserver(() => {
      const s = el.textContent?.trim() ?? "";
      if (log[log.length - 1][1] !== s) log.push([Math.round(performance.now() - t0), s]);
    }).observe(el, { subtree: true, childList: true, characterData: true });
    (window as unknown as { __sl: typeof log }).__sl = log;
  });
const changes = (page: import("@playwright/test").Page) =>
  page.evaluate(() => (window as unknown as { __sl: [number, string][] }).__sl);

async function typeFor(page: import("@playwright/test").Page, text: string, ms: number) {
  await page.locator(".cm-content").click();
  await page.keyboard.press("ControlOrMeta+End");
  const t0 = Date.now();
  for (let i = 0; Date.now() - t0 < ms; i++) {
    await page.keyboard.type(text[i % text.length]);
    await page.waitForTimeout(100);
  }
}

test("UX-SI1b-1 while two people type, the header holds one wording, it does not blink every second", async ({
  browser,
}) => {
  test.setTimeout(90_000);
  const alice = await open(browser, ALICE_URL);
  const bob = await open(browser, BOB_URL, true);
  await clear(alice);
  await clear(bob);
  await pair(alice, bob);
  await record(alice);
  await Promise.all([typeFor(alice, "Bonjour. ", 8000), typeFor(bob, "Salut. ", 8000)]);
  const log = await changes(alice);
  // Measured: 8 changes in 8 s ("Envoi en cours (1)" / "Pas encore à jour" alternating each second).
  // Expected: at most synced -> one working wording -> synced.
  expect(log.length - 1, JSON.stringify(log)).toBeLessThanOrEqual(2);
});

test("UX-SI1b-2 the header's status wording never overlaps the title, down to 500 px", async ({
  browser,
}) => {
  test.setTimeout(90_000);
  const alice = await open(browser, ALICE_URL);
  const bob = await open(browser, BOB_URL, true);
  await clear(alice);
  await clear(bob);
  await pair(alice, bob);
  await typeFor(alice, "Bonjour. ", 2000);
  await expect(alice.getByRole("status")).toHaveText(/Envoi en cours|Pas encore à jour/);
  for (const width of [900, 700, 600, 500]) {
    await alice.setViewportSize({ width, height: 740 });
    const box = async (sel: string) => alice.locator(sel).first().boundingBox();
    const status = await box('[role="status"]');
    const title = await alice.getByText("Sans titre", { exact: true }).first().boundingBox();
    expect(status && title && status.x + status.width <= title.x, `${width}px`).toBeTruthy();
  }
});
