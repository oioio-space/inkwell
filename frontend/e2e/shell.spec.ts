import AxeBuilder from "@axe-core/playwright";
import { expect, type Page, test } from "@playwright/test";
import { clear, editor, open } from "./helpers";
import { ALICE_URL } from "./ports";

// The Adwaita shell (task A4): keyboard, remembered layout, overflow, accessibility and the
// reference screenshots. One instance is enough: nothing here needs a second person.
const URL = ALICE_URL;

const toolbar = (p: Page) => p.getByRole("toolbar");
const headingButton = (p: Page) => p.getByRole("button", { name: /Style de paragraphe/ });

/** A document that shows the interface at its best, the same every time. */
async function sample(p: Page) {
  await clear(p);
  await editor(p).click();
  await p.keyboard.type(
    '= Plan de continuité <plan>\n\nAvant toute action, vérifier l\'état de la *réplication*\net prévenir le responsable, voir @plan.\n#raw("pg_ctl promote")\n+ Geler les écritures\n+ Promouvoir le réplica',
  );
  await expect(p.getByTestId("words")).toHaveText(/mots/);
}

test("UI-03 keyboard only: arrows move in the menu, Escape closes it and gives the editor its focus back", async ({
  browser,
}) => {
  const page = await open(browser, URL);
  await clear(page);
  await headingButton(page).focus();
  await page.keyboard.press("ArrowDown");
  const menu = page.getByRole("menu", { name: "Style de paragraphe" });
  await expect(menu).toBeVisible();
  const items = menu.getByRole("menuitemradio");
  await expect(items.first()).toBeFocused();
  await page.keyboard.press("ArrowDown");
  await expect(items.nth(1)).toBeFocused();
  await page.keyboard.press("Enter");
  // Choosing "Titre 1" is one command: the line becomes a heading, focus is back in the text.
  await expect(menu).toBeHidden();
  await expect(editor(page)).toBeFocused();
  await expect(headingButton(page)).toContainText("Titre 1");

  await headingButton(page).focus();
  await page.keyboard.press("Enter");
  await expect(menu).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(menu).toBeHidden();
  await expect(editor(page)).toBeFocused();

  // Tab walks the bar, button by button.
  await page.getByRole("button", { name: "Gras" }).focus();
  await page.keyboard.press("Tab");
  await expect(page.getByRole("button", { name: "Italique" })).toBeFocused();
  await clear(page);
});

test("UI-04 Ctrl+Shift+T hides the bar, and the project remembers it", async ({ browser }) => {
  const page = await open(browser, URL);
  await expect(toolbar(page)).toBeVisible();
  await page.keyboard.press("ControlOrMeta+Shift+t");
  await expect(toolbar(page)).toBeHidden();
  await expect(editor(page)).toBeFocused();

  await page.reload();
  await expect(editor(page)).toBeVisible();
  await expect(toolbar(page)).toBeHidden();

  // The main menu says the same thing and brings it back.
  await page.getByRole("button", { name: "Menu principal" }).click();
  const entry = page.getByRole("menuitemcheckbox", { name: /Barre de mise en forme/ });
  await expect(entry).toHaveAttribute("aria-checked", "false");
  await entry.click();
  await expect(toolbar(page)).toBeVisible();
});

test("UI-05 in an 800 px window the buttons that do not fit sit in the ⋯ menu", async ({
  browser,
}) => {
  const page = await open(browser, URL);
  const more = page.getByRole("button", { name: "Plus d'actions" });
  await expect(more).toHaveCount(0);

  await page.setViewportSize({ width: 800, height: 640 });
  await expect(more).toBeVisible();
  const onBar = await toolbar(page)
    .locator("button[data-bar]")
    .evaluateAll((els) => els.map((e) => (e as HTMLElement).dataset.bar));
  await more.click();
  const inMenu = await page
    .getByRole("menu", { name: "Plus d'actions" })
    .locator("[data-bar]")
    .evaluateAll((els) => els.map((e) => (e as HTMLElement).dataset.bar));
  expect(inMenu.length).toBeGreaterThan(0);
  // Typst has twelve buttons: none is lost, none is shown twice.
  expect([...onBar, ...inMenu].sort()).toEqual(
    [
      "bold",
      "code",
      "codeblock",
      "formula",
      "italic",
      "link",
      "ol",
      "quote",
      "strike",
      "table",
      "ul",
      "underline",
    ].sort(),
  );
  // And nothing sticks out of the bar.
  const box = await toolbar(page).boundingBox();
  for (const b of await toolbar(page).getByRole("button").all()) {
    const r = await b.boundingBox();
    if (r && box) expect(r.x + r.width).toBeLessThanOrEqual(box.x + box.width + 1);
  }
  await page.keyboard.press("Escape");

  await page.setViewportSize({ width: 1100, height: 640 });
  await expect(more).toHaveCount(0);
});

test("UI-02 the bar follows the caret in a real editor, and Ctrl+B is one undo step", async ({
  browser,
}) => {
  const page = await open(browser, URL);
  await clear(page);
  await editor(page).click();
  await page.keyboard.type("un mot ici");
  await page.keyboard.press("Home");
  for (let i = 0; i < 4; i++) await page.keyboard.press("ArrowRight");
  const bold = page.getByRole("button", { name: "Gras" });
  await expect(bold).toHaveAttribute("aria-pressed", "false");
  await page.keyboard.press("ControlOrMeta+b");
  await expect(editor(page)).toContainText("un *mot* ici");
  await expect(bold).toHaveAttribute("aria-pressed", "true");
  await page.keyboard.press("ControlOrMeta+z");
  await expect(editor(page)).toContainText("un mot ici");
  await expect(bold).toHaveAttribute("aria-pressed", "false");

  // Code shows pressed inside inline code.
  await page.keyboard.press("ControlOrMeta+e");
  await expect(editor(page)).toContainText("un `mot` ici");
  await expect(page.getByRole("button", { name: "Code en ligne" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await clear(page);
});

test("UI-07 the status bar hides the encoding for a UTF-8 file with LF line ends", async ({
  browser,
}) => {
  const page = await open(browser, URL);
  await sample(page);
  const bar = page.getByRole("contentinfo", { name: "Barre d'état" });
  await expect(bar).toContainText("Ligne 7, colonne 24");
  await expect(bar).toContainText("Typst");
  await expect(bar).not.toContainText("UTF-8");
  await expect(bar).not.toContainText("LF");
  await clear(page);
});

/** Where the line numbers and the text start: what must not move when something opens. */
const gutterGeometry = (p: Page) =>
  p.evaluate(() => {
    const box = (el: Element | null) => {
      const r = el?.getBoundingClientRect();
      return r ? { x: r.x, y: r.y, width: r.width, height: r.height } : null;
    };
    const number = document.querySelector(".cm-lineNumbers .cm-gutterElement:nth-child(2)");
    return {
      gutters: box(document.querySelector(".cm-gutters")),
      number: box(number),
      line: box(document.querySelector(".cm-line")),
      align: number ? getComputedStyle(number).textAlign : "",
      minWidth: number ? getComputedStyle(number).minWidth : "",
    };
  });

test("UI-12 the line-number gutter keeps its geometry while a popover or a dialog is open", async ({
  browser,
}) => {
  const page = await open(browser, URL);
  await sample(page);
  const before = await gutterGeometry(page);
  // The 56 px column of the design; a lost theme would give CodeMirror's 20 px default.
  expect(before.gutters?.width).toBe(56);
  expect(before.minWidth).toBe("56px");
  expect(before.align).toBe("right");

  await headingButton(page).click();
  await expect(page.getByRole("menu", { name: "Style de paragraphe" })).toBeVisible();
  expect(await gutterGeometry(page)).toEqual(before);
  await page.keyboard.press("Escape");

  await page.getByRole("button", { name: /Insérer/ }).click();
  await expect(page.getByRole("menu", { name: /Insérer/ })).toBeVisible();
  expect(await gutterGeometry(page)).toEqual(before);
  await page.keyboard.press("Escape");

  await page.getByRole("button", { name: "Menu principal" }).click();
  await page.getByRole("menuitem", { name: "Préférences" }).click();
  await expect(page.getByRole("dialog", { name: "Préférences" })).toBeVisible();
  expect(await gutterGeometry(page)).toEqual(before);
  await page.keyboard.press("Escape");

  await expect(page.getByRole("dialog", { name: "Préférences" })).toBeHidden();
  expect(await gutterGeometry(page)).toEqual(before);
  await clear(page);
});

/** The colour a CSS variable resolves to in the page. */
const resolved = (p: Page, variable: string) =>
  p.evaluate((name) => {
    const probe = document.createElement("i");
    probe.style.color = `var(${name})`;
    document.body.append(probe);
    const colour = getComputedStyle(probe).color;
    probe.remove();
    return colour;
  }, variable);

for (const dark of [false, true]) {
  test(`UI-13 Typst source is coloured with the Adwaita palette, ${dark ? "dark" : "light"}`, async ({
    browser,
  }) => {
    const page = await open(browser, URL, dark);
    await sample(page);
    // CodeMirror redraws the spans when the parser delivers its tree: poll until it settled.
    const style = (text: string, prop: "color" | "fontWeight") =>
      expect.poll(() =>
        editor(page)
          .locator(".cm-line span", { hasText: text })
          .first()
          .evaluate((el, p) => getComputedStyle(el)[p], prop),
      );
    const colour = async (text: string, variable: string) =>
      style(text, "color").toBe(await resolved(page, variable));

    await colour("continuité", "--hl-heading");
    await style("continuité", "fontWeight").toBe("700");
    await colour("<plan>", "--hl-label");
    await colour("@plan", "--hl-label");
    await colour('"pg_ctl promote"', "--hl-string");
    await colour("raw", "--hl-function");
    await style("réplication", "fontWeight").toBe("700");
    await clear(page);
  });
}

for (const dark of [false, true]) {
  test(`UI-06 ${dark ? "dark" : "light"} theme has no accessibility violation (WCAG A and AA)`, async ({
    browser,
  }) => {
    const page = await open(browser, URL, dark);
    await sample(page);
    const scan = () =>
      new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
    const summary = async () =>
      (await scan()).violations.map(
        (v) => `${v.id}: ${v.nodes.map((n) => n.target.join(" ")).join(" | ")}`,
      );

    expect(await summary()).toEqual([]);

    await headingButton(page).click();
    await expect(page.getByRole("menu", { name: "Style de paragraphe" })).toBeVisible();
    expect(await summary()).toEqual([]);
    await page.keyboard.press("Escape");

    await page.getByRole("button", { name: /Insérer/ }).click();
    await expect(page.getByRole("menu", { name: /Insérer/ })).toBeVisible();
    expect(await summary()).toEqual([]);
    await page.keyboard.press("Escape");

    await page.getByRole("button", { name: "Menu principal" }).click();
    await page.getByRole("menuitem", { name: "Préférences" }).click();
    await expect(page.getByRole("dialog", { name: "Préférences" })).toBeVisible();
    expect(await summary()).toEqual([]);
    await page.keyboard.press("Escape");
    await clear(page);
  });
}

// Reference screenshots (UI-11). They are taken with Chromium: WebKit renders text with its
// own rasteriser, so CI (WebKit) would need baselines of its own, made on its own runner.
test.describe("UI-11 reference screenshots", () => {
  test.skip(({ browserName }) => browserName !== "chromium", "baselines are made with Chromium");

  const shot = { maxDiffPixelRatio: 0.002 } as const;
  // What depends on who else is connected (the two instances stay paired with each other
  // after the collaboration tests, so "Seul" or "Synchronisé" are both true here), on a
  // generated identity, on a blinking caret or on an outside edit left by an earlier test
  // (the "modified outside" toast) is hidden by a constructed stylesheet, which the page's CSP
  // allows and which covers what appears later. The status itself is tested in unit tests
  // (workspace.test.ts) and by the pairing helper.
  const hideVolatile = (p: Page) =>
    p.evaluate(() => {
      const sheet = new CSSStyleSheet();
      sheet.replaceSync(
        '[role="status"], .people, ul[aria-label$="document"], .cm-cursorLayer, [role="alert"] { visibility: hidden !important }',
      );
      document.adoptedStyleSheets = [...document.adoptedStyleSheets, sheet];
    });
  for (const dark of [false, true]) {
    const tag = dark ? "dark" : "light";
    test(`window, menus, status bar and preferences, ${tag}`, async ({ browser }) => {
      const page = await open(browser, URL, dark);
      await page.setViewportSize({ width: 1408, height: 868 });
      await sample(page);
      // The file list is read from the project folder, a moment after the editor appears (the
      // window polls every 4 s if the first read came too early).
      await expect(page.getByText("principal", { exact: true })).toBeVisible({ timeout: 15_000 });
      // Keep the caret off the text under the menus and the pointer out of the way.
      await page.mouse.move(700, 600);

      await hideVolatile(page);
      await expect(page).toHaveScreenshot(`window-${tag}.png`, { ...shot });

      await headingButton(page).click();
      await expect(page.getByRole("menu", { name: "Style de paragraphe" })).toBeVisible();
      await expect(page).toHaveScreenshot(`heading-menu-${tag}.png`, { ...shot });
      await page.keyboard.press("Escape");

      await page.getByRole("button", { name: /Insérer/ }).click();
      await expect(page.getByRole("menu", { name: /Insérer/ })).toBeVisible();
      await expect(page).toHaveScreenshot(`insert-menu-${tag}.png`, { ...shot });
      await page.keyboard.press("Escape");

      await expect(page.getByRole("contentinfo", { name: "Barre d'état" })).toHaveScreenshot(
        `status-bar-${tag}.png`,
        shot,
      );

      await page.getByRole("button", { name: "Menu principal" }).click();
      await page.getByRole("menuitem", { name: "Préférences" }).click();
      await expect(page.getByRole("dialog", { name: "Préférences" })).toBeVisible();
      await page.mouse.move(10, 10);
      await expect(page.getByRole("dialog", { name: "Préférences" })).toHaveScreenshot(
        `preferences-${tag}.png`,
        shot,
      );
      await page.keyboard.press("Escape");
      await clear(page);
    });
  }
});
