import type { Page } from "@playwright/test";
import { expect, test } from "../fixtures";
import { clear, docText, editor, open } from "../helpers";
import { ALICE_URL } from "../ports";

// Defects of the S11 walkthrough (presenting), found by looking at the screen.
// See docs/ux/walkthrough-2026-10-26-present.md.

const DECK = "= Un\n\nBonjour\n\n#pagebreak()\n\n= Deux\n\nFin";
const SLOW = { timeout: 45_000 };

const view = (p: Page) => p.getByTestId("presenter-view");

async function startPresenting(p: Page) {
  await clear(p);
  await editor(p).click();
  await p.keyboard.insertText(DECK);
  await expect(p.getByTestId("page-indicator")).toHaveText(/sur 2/, SLOW);
  // Back in the editor, as a person who writes then presses F5: the caret is in the text.
  await editor(p).click();
  await p.keyboard.press("F5");
  await expect(view(p)).toBeVisible();
  await expect(view(p).locator("img").first()).toBeVisible(SLOW);
}

async function leave(p: Page) {
  if (await view(p).isVisible()) await p.keyboard.press("Escape");
  await expect(view(p)).toBeHidden();
}

test("UX-S11-1 keys typed during a presentation never reach the document behind it", async ({
  browser,
}) => {
  const p = await open(browser, ALICE_URL);
  await startPresenting(p);
  const before = await docText(p);
  // Letters that mean nothing to the slides, then Delete and a space-less typing burst.
  await p.keyboard.type("xyz");
  await p.keyboard.press("Delete");
  await p.keyboard.press("Control+a");
  await p.keyboard.press("Control+x");
  await p.waitForTimeout(600);
  await leave(p);
  expect(await docText(p)).toBe(before);
});

test("UX-S11-2 the keyboard stays in the presentation: Tab never lands on the screen behind it", async ({
  browser,
}) => {
  const p = await open(browser, ALICE_URL);
  await startPresenting(p);
  for (let i = 0; i < 20; i++) {
    await p.keyboard.press("Tab");
    const outside = await p.evaluate(() => {
      const e = document.activeElement;
      if (!e || e === document.body) return false;
      return e.closest("[data-testid=presenter-view]") === null;
    });
    expect(outside, `Tab ${i + 1} left the presentation`).toBe(false);
  }
  await leave(p);
});

test("UX-S11-3 the presenter's header does not overlap at 200 % zoom or in a narrow window", async ({
  browser,
}) => {
  const p = await open(browser, ALICE_URL);
  await startPresenting(p);
  for (const size of [
    { width: 550, height: 320 }, // 1100 x 640 at 200 %
    { width: 360, height: 640 },
  ]) {
    await p.setViewportSize(size);
    await p.waitForTimeout(300);
    const boxes = await view(p)
      .locator("header.hb")
      .evaluate((hb) => {
        const items = [...hb.querySelectorAll<HTMLElement>("button, [role=status], .time, .clock")];
        return items
          .filter((e) => e.getClientRects().length > 0)
          .map((e) => {
            const r = e.getBoundingClientRect();
            return {
              name: e.getAttribute("aria-label") || e.textContent?.trim() || e.className,
              l: r.left,
              r: r.right,
              t: r.top,
              b: r.bottom,
            };
          });
      });
    for (let i = 0; i < boxes.length; i++) {
      for (let j = i + 1; j < boxes.length; j++) {
        const a = boxes[i];
        const b = boxes[j];
        const overlap = a.l < b.r - 1 && b.l < a.r - 1 && a.t < b.b - 1 && b.t < a.b - 1;
        expect(overlap, `${size.width}px: "${a.name}" overlaps "${b.name}"`).toBe(false);
      }
    }
    expect(await p.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
  await p.setViewportSize({ width: 1100, height: 640 });
  await leave(p);
});
