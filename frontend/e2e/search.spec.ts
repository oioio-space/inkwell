import { expect, test } from "./fixtures";
import { clear, docText, editor, open, typeAtEnd } from "./helpers";
import { ALICE_URL } from "./ports";

// B6 search and replace. The engine (frontend/src/editor/search/) and the panel
// (frontend/src/ui/search/SearchPanel.svelte) are covered by unit and component tests, SRC-01…22.
// Two cases need the real app, with the panel mounted in the shell and the keys wired into the
// editor. That wiring belongs to task A4 (the Adwaita shell: App.svelte, HeaderBar, Sidebar,
// StatusBar, editor/setup.ts). How to wire it is written at the top of
// frontend/src/editor/search/index.ts. Until A4 lands these stay fixme; remove `.fixme` then.

test.fixme("SRC-11 clicking a result in another file opens that file, selects the match and centres it", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await clear(page);
  await typeAtEnd(page, "début\nun mot à trouver\nfin");

  // Ctrl+Shift+F searches the whole project; the panel lists a.typ and b.typ.
  await page.keyboard.press("Control+Shift+F");
  await page.getByRole("searchbox").fill("trouver");
  await page.keyboard.press("Enter");
  const tree = page.getByRole("tree");
  await expect(tree.getByRole("treeitem", { level: 2 })).not.toHaveCount(0);

  // A result in the other file: the file opens, the match is selected and in view.
  await tree.getByRole("treeitem", { level: 2 }).last().click();
  const selected = await page.evaluate(() => String(window.getSelection()));
  expect(selected).toBe("trouver");
  await expect(editor(page)).toBeInViewport();
});

test.fixme("SRC-20 keyboard in the app: Ctrl+F, Ctrl+H, Ctrl+Shift+F, Enter, Shift+Enter, F3/Shift+F3, F4/Shift+F4, Escape", async ({
  browser,
}) => {
  const page = await open(browser, ALICE_URL);
  await clear(page);
  await typeAtEnd(page, "foo bar foo bar foo");

  await page.keyboard.press("Control+F");
  const field = page.getByRole("searchbox");
  await expect(field).toBeFocused();
  await field.fill("foo");

  // Enter and F3 go forward, Shift+Enter and Shift+F3 go back; the match is the selection.
  const selection = () => page.evaluate(() => window.getSelection()?.anchorOffset ?? -1);
  await page.keyboard.press("Enter");
  const first = await selection();
  await page.keyboard.press("Enter");
  expect(await selection()).not.toBe(first);
  await page.keyboard.press("Shift+Enter");
  expect(await selection()).toBe(first);
  await page.keyboard.press("F3");
  await page.keyboard.press("Shift+F3");
  expect(await selection()).toBe(first);

  // F4 / Shift+F4 walk the results list from the editor too.
  await editor(page).click();
  await page.keyboard.press("F4");
  await page.keyboard.press("Shift+F4");

  // Ctrl+H opens the replacement row; Ctrl+Shift+F searches the project.
  await page.keyboard.press("Control+H");
  await expect(page.getByLabel("Remplacer par")).toBeVisible();
  await page.keyboard.press("Control+Shift+F");
  await expect(page.getByRole("button", { name: "Tout le projet" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );

  // Escape closes the panel and gives the focus back to the editor.
  await page.keyboard.press("Escape");
  await expect(field).toBeHidden();
  await expect(editor(page)).toBeFocused();
  expect(await docText(page)).toBe("foo bar foo bar foo");
});
