import type { Page } from "@playwright/test";
import { expect, test } from "../fixtures";
import { editor, open } from "../helpers";
import { ALICE_URL } from "../ports";

const _SHOT = "../gates/L3-setup/ux";

/** The person can see and click the editor: what sits at its centre is the editor itself. */
const editorReachable = (p: Page) =>
  editor(p).evaluate((el) => {
    const r = el.getBoundingClientRect();
    if (r.width < 50 || r.height < 20) return false;
    const hit = document.elementFromPoint(r.left + Math.min(r.width / 2, 60), r.top + 10);
    return !!hit && !!hit.closest(".cm-editor");
  });

/** Status-bar items that overlap each other: the text becomes unreadable. */
const statusOverlaps = (p: Page) =>
  p.locator("footer.bot").evaluate((bar) => {
    const boxes = [...bar.children]
      .map((c) => {
        // The text's own box: a shrunk item can let its text spill over the next one.
        const range = document.createRange();
        range.selectNodeContents(c);
        return { name: (c.textContent ?? "").trim(), r: range.getBoundingClientRect() };
      })
      .filter((b) => b.r.width > 0);
    const out: string[] = [];
    for (let i = 0; i < boxes.length; i++)
      for (let j = i + 1; j < boxes.length; j++) {
        const a = boxes[i]?.r,
          b = boxes[j]?.r;
        if (a && b && a.left < b.right - 1 && b.left < a.right - 1)
          out.push(`${boxes[i]?.name} / ${boxes[j]?.name}`);
      }
    const last = boxes.at(-1)?.r;
    if (last && last.right > bar.getBoundingClientRect().right + 1)
      out.push("cut off at the right");
    return out;
  });

async function withError(page: Page) {
  await editor(page).fill('= Accueil\n\nBonjour.\n\n#titreInconnu("x")\n');
  await page.keyboard.press("ControlOrMeta+s");
  await expect(page.getByTestId("problems")).toHaveAttribute("data-errors", "1", {
    timeout: 30_000,
  });
}

test("UX-L3-1: clicking a problem in a narrow window shows the editor, not the preview", async ({
  browser,
}, info) => {
  const page = await open(browser, ALICE_URL);
  try {
    await withError(page);
    await page.getByTestId("problems").click({ modifiers: ["Shift"] });
    const panel = page.getByRole("region", { name: "Problèmes" });
    await panel.getByRole("button").first().click();
    await expect.poll(() => editorReachable(page)).toBe(true);
    await page.screenshot({ path: info.outputPath("l3-1.png") });
  } finally {
    await editor(page).fill("");
  }
});

test("UX-L3-2: the status bar never overlaps itself with the problems panel open", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  try {
    await editor(page).fill("= Accueil\n\nBonjour.\n");
    await expect(page.getByTestId("page-indicator")).toBeVisible({ timeout: 30_000 });
    await editor(page).press("ControlOrMeta+End");
    await page.keyboard.type("#titreInconnu");
    await page.keyboard.press("ControlOrMeta+s");
    await expect(page.getByTestId("problems")).toHaveAttribute("data-errors", "1", {
      timeout: 30_000,
    });
    await page.getByTestId("problems").click({ modifiers: ["Shift"] });
    await expect.poll(() => statusOverlaps(page)).toEqual([]);
  } finally {
    await editor(page).fill("");
  }
});

const ready = (p: Page) =>
  expect
    .poll(() =>
      p.evaluate(() =>
        (
          globalThis as typeof globalThis & { __inkwellLanguageReady?: () => boolean }
        ).__inkwellLanguageReady?.(),
      ),
    )
    .toBe(true);

test("UX-L3-3: signature help reads as plain text and fits the window", async ({
  browser,
}, info) => {
  const page = await open(browser, ALICE_URL);
  try {
    await page.setViewportSize({ width: 800, height: 600 });
    await editor(page).fill("Bonjour\n#image");
    await ready(page);
    await page.keyboard.press("ControlOrMeta+End");
    await page.keyboard.type("(");
    await page.keyboard.press("ControlOrMeta+Shift+Space");
    const help = page.locator(".writing-help").last();
    await expect(help).toContainText("source");
    await page.screenshot({ path: info.outputPath("l3-3.png") });
    const text = await help.innerText();
    expect(text).not.toContain("```");
    expect(text).not.toMatch(/\]\(https?:/);
    expect(text).not.toMatch(/^---$/m);
    const box = await help.boundingBox();
    expect(box && box.y + box.height).toBeLessThanOrEqual(600 - 36);
  } finally {
    await page.keyboard.press("Escape");
    await editor(page).fill("");
  }
});

test("UX-L3-4: file completion hides the internal folder and shows no empty card", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  try {
    await editor(page).fill("#image");
    await ready(page);
    await page.keyboard.press("ControlOrMeta+End");
    await page.keyboard.type('("');
    await page.keyboard.press("ControlOrMeta+Space");
    const list = page.locator(".cm-tooltip-autocomplete");
    await expect(list).toBeVisible();
    await expect(list).not.toContainText(".inkwell");
    await expect(
      page.locator(".cm-completionInfo:empty, .cm-completionInfo .writing-help:empty"),
    ).toHaveCount(0);
    for (const card of await page.locator(".cm-completionInfo").all())
      expect((await card.innerText()).trim()).not.toBe("");
  } finally {
    await page.keyboard.press("Escape");
    await editor(page).fill("");
  }
});

test("UX-L3-5: an error found by the compiler alone stays clickable after saving", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  try {
    await page.setViewportSize({ width: 1440, height: 900 });
    await editor(page).fill('#image("absent.png")\n');
    await page.keyboard.press("ControlOrMeta+s");
    await expect(page.getByTestId("problems")).toHaveAttribute("data-errors", "1", {
      timeout: 30_000,
    });
    await page.getByTestId("problems").click({ modifiers: ["Shift"] });
    const panel = page.getByRole("region", { name: "Problèmes" });
    // Saving writes the same text again: it must not make the verified problem "to check again".
    await expect(panel.getByRole("button").first()).toBeEnabled({ timeout: 10_000 });
    // The disk echo of the save comes a few seconds later; the problem must survive it.
    await page.waitForTimeout(8_000);
    await expect(panel.getByRole("button").first()).toBeEnabled();
    await expect(panel).not.toContainText("vérifié à nouveau");
    await panel.getByRole("button").first().click();
    await expect
      .poll(() =>
        page.evaluate(() =>
          (
            globalThis as typeof globalThis & { __inkwellSelection?: () => string }
          ).__inkwellSelection?.(),
        ),
      )
      .not.toBe("");
  } finally {
    await editor(page).fill("");
  }
});

test("UX-L3-6: common compiler messages are in French in the problems panel", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  try {
    await page.setViewportSize({ width: 1440, height: 900 });
    for (const source of ["#let f(a b) = a\n", '#set text(font: "PoliceInexistante")\nBonjour\n']) {
      await editor(page).fill(source);
      await page.keyboard.press("ControlOrMeta+s");
      await expect(page.getByTestId("problems")).toHaveAttribute("data-errors", /./, {
        timeout: 30_000,
      });
      await page.getByTestId("problems").click({ modifiers: ["Shift"] });
      const panel = page.getByRole("region", { name: "Problèmes" });
      await expect(panel.getByRole("button").first()).toBeVisible();
      // The English original stays in "Détails techniques" (closed), never as the summary.
      const shown = await panel.evaluate((el) => {
        const copy = el.cloneNode(true) as HTMLElement;
        for (const d of copy.querySelectorAll("details")) d.remove();
        return copy.textContent ?? "";
      });
      expect(shown).not.toMatch(/expected comma|unknown font|policeinexistante/i);
      expect(shown).not.toContain("Consultez les détails techniques");
    }
  } finally {
    await editor(page).fill("");
  }
});

test("UX-L3-7: the problem counter tells how to reach the next problem and the list", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  try {
    await withError(page);
    const hint = await page
      .getByRole("button", { name: /Problèmes du document/ })
      .getAttribute("title");
    expect(hint).toContain("F8");
    expect(hint).toMatch(/Maj/);
  } finally {
    await editor(page).fill("");
  }
});
