import { mkdtempSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import AxeBuilder from "@axe-core/playwright";
import type { Page } from "@playwright/test";
import { docsRoot, mainTypIn, readDisk } from "./disk";
import { expect, test } from "./fixtures";
import { clear, open, pair } from "./helpers";
import { startInstance, stopInstance } from "./instance";
import { ALICE_PORT, ALICE_URL, BOB_URL, SPARE_PORT, SPARE_URL } from "./ports";
import {
  cell,
  clipboardText,
  cspErrors,
  DATA,
  drag,
  grid,
  head,
  openSheet,
  pasteInTextView,
  pick,
  readText,
  rowHead,
  setClipboard,
  setText,
} from "./sheetHelpers";

// B9 — the spreadsheet. Everything here drives the real app (two real Inkwell instances) with
// the real mouse, keyboard and clipboard. The text of the file is read back through the text
// view, so each check is on the CSV itself, not on what the grid shows.

const SHOTS =
  "/tmp/claude-1000/-home-mathieu-GolandProjects-inkwell/9e09cb9d-09c5-4b33-8b2b-0d87e3efdfbf/scratchpad/shots";

async function range(p: Page, r0: number, c0: number, r1: number, c1: number) {
  await pick(p, r0, c0);
  await cell(p, r1, c1).click({ modifiers: ["Shift"] });
}

// The instances keep their project between specs: leave the document empty, as found.
test.afterAll(async ({ browser }) => {
  const page = await openSheet(browser, ALICE_URL);
  await setText(page, "");
  await page.context().close();
});

/** Ctrl+Z with the keyboard in the grid (after the text view the focus is elsewhere). */
const undo = async (p: Page) => {
  await grid(p).focus();
  await p.keyboard.press("ControlOrMeta+z");
};

test.describe("copy, cut and paste (the user's first priority)", () => {
  test("SHEET-CLIP-01 copying a range puts TSV and an HTML table on the system clipboard", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL, { clipboard: true });
    await setText(page, DATA);
    await range(page, 0, 0, 1, 1);
    await page.keyboard.press("ControlOrMeta+c");
    await expect.poll(() => clipboardText(page)).toBe("Pomme\t3\nPoire\t12");
    const types = await page.evaluate(async () => {
      const items = await navigator.clipboard.read();
      const item = items[0];
      if (!item) return {};
      const html = item.types.includes("text/html")
        ? await (await item.getType("text/html")).text()
        : "";
      return { types: item.types, html };
    });
    expect(types.types).toContain("text/html");
    expect(types.html).toContain("<table>");
    expect(types.html).toContain("<td>Poire</td>");
    expect(cspErrors(page)).toEqual([]);
  });

  test("CSV-07 a 200 x 10 block from a spreadsheet extends the grid in one change; one undo removes it", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL, { clipboard: true });
    await setText(page, DATA);
    const block = Array.from({ length: 200 }, (_, r) =>
      Array.from({ length: 10 }, (_, c) => `r${r}c${c}`).join("\t"),
    ).join("\n");
    await setClipboard(page, block);
    await pick(page, 0, 0);
    await page.keyboard.press("ControlOrMeta+v");
    // More than 1 000 cells: a preview first.
    const dialog = page.getByRole("dialog", { name: "Coller beaucoup de cellules ?" });
    await expect(dialog).toContainText("200 lignes sur 10 colonnes");
    await dialog.getByRole("button", { name: "Coller" }).click();
    await expect(grid(page)).toHaveAttribute("aria-rowcount", "201");
    await grid(page).focus();
    await page.keyboard.press("ControlOrMeta+End");
    await expect(cell(page, 199, 9)).toHaveText("r199c9");
    await expect(page.getByTestId("uneven")).toBeHidden();
    // One undo step.
    await undo(page);
    expect(await readText(page)).toBe(DATA);
  });

  test("CSV-08 / CSV-09 paste a range over a smaller selection extends; HTML tables and values-only", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL, { clipboard: true });
    await setText(page, DATA);
    await pick(page, 3, 2);
    await setClipboard(page, "a\tb\nc\td");
    await page.keyboard.press("ControlOrMeta+v");
    // The table grows to four columns: every row gets the new, empty one.
    expect(await readText(page)).toBe(
      "Nom;Qté;Prix;\nPomme;3;1,50;\nPoire;12;2,25;\nCerise;7;10;\nFigue;1;a;b\n;;c;d\n",
    );
    await grid(page).focus();
    await undo(page);
    expect(await readText(page)).toBe(DATA);

    // An HTML table from a web page: the cells, no formatting, nothing executable.
    await pick(page, 0, 0);
    await setClipboard(
      page,
      "x y",
      '<table><tr><td onclick="alert(1)"><b>x</b><script>alert(2)</script></td><td style="color:red">y</td></tr></table>',
    );
    await page.keyboard.press("ControlOrMeta+v");
    await expect(cell(page, 0, 0)).toHaveText("x");
    await expect(cell(page, 0, 1)).toHaveText("y");
    expect(await readText(page)).not.toMatch(/script|alert|onclick/);
    await grid(page).focus();
    await undo(page);

    // Ctrl+Shift+V: the plain text only, in one cell.
    await pick(page, 0, 0);
    await page.keyboard.press("ControlOrMeta+Shift+v");
    await expect(cell(page, 0, 0)).toHaveText("x y");
  });

  test("one value over a range fills it; cut then paste moves the cells as one undo step", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL, { clipboard: true });
    await setText(page, DATA);
    await range(page, 0, 1, 3, 1);
    await setClipboard(page, "9");
    await page.keyboard.press("ControlOrMeta+v");
    expect(await readText(page)).toBe(
      "Nom;Qté;Prix\nPomme;9;1,50\nPoire;9;2,25\nCerise;9;10\nFigue;9;4,2\n",
    );
    await grid(page).focus();
    await undo(page);
    expect(await readText(page)).toBe(DATA);

    // Cut Nom of rows 1-2, paste at the Prix column (one step).
    await range(page, 0, 0, 1, 0);
    await page.keyboard.press("ControlOrMeta+x");
    await pick(page, 2, 0);
    await page.keyboard.press("ControlOrMeta+v");
    expect(await readText(page)).toBe("Nom;Qté;Prix\n;3;1,50\n;12;2,25\nPomme;7;10\nPoire;1;4,2\n");
    await grid(page).focus();
    await undo(page);
    expect(await readText(page)).toBe(DATA);
  });

  test("CSV-22 copying a cell that looks like a formula neutralises it and says so", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL, { clipboard: true });
    await setText(page, "a;b\n=1+1;-5\n+cmd;x\n");
    await range(page, 0, 0, 1, 1);
    await page.keyboard.press("ControlOrMeta+c");
    await expect.poll(() => clipboardText(page)).toBe("'=1+1\t-5\n'+cmd\tx");
    await expect(page.getByText(/apostrophe/).first()).toBeVisible();
    await page.getByRole("button", { name: "Copier sans apostrophe" }).click();
    await expect.poll(() => clipboardText(page)).toBe("=1+1\t-5\n+cmd\tx");
  });

  test("copying from the text view and pasting in the table lands in the right cells", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL, { clipboard: true });
    await setText(page, DATA);
    await page.getByRole("button", { name: "Voir le texte" }).click();
    const content = page.locator(".cm-content");
    await content.click();
    await page.keyboard.press("ControlOrMeta+Home");
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("Home");
    await page.keyboard.press("Shift+End");
    await page.keyboard.press("ControlOrMeta+c");
    await page.getByRole("button", { name: "Voir le tableau" }).click();
    await pick(page, 3, 0);
    await page.keyboard.press("ControlOrMeta+v");
    await expect(cell(page, 3, 0)).toHaveText("Pomme");
    await expect(cell(page, 3, 1)).toHaveText("3");
    await expect(cell(page, 3, 2)).toHaveText("1,50");
  });
});

test.describe("dragging", () => {
  test("SHEET-DRAG-01 the fill handle continues a series down, copies, goes up and left", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, "n;l\n1;Lundi\n2;\n;\n;\n;\n");
    await range(page, 0, 0, 1, 0);
    await drag(page, page.locator("[data-fill]"), cell(page, 4, 0));
    await expect
      .poll(() => readText(page), { timeout: 10_000 })
      .toBe("n;l\n1;Lundi\n2;\n3;\n4;\n5;\n");
    await grid(page).focus();
    await undo(page);

    // A day name continues the week.
    await pick(page, 0, 1);
    await drag(page, page.locator("[data-fill]"), cell(page, 2, 1));
    await expect
      .poll(() => readText(page), { timeout: 10_000 })
      .toBe("n;l\n1;Lundi\n2;Mardi\n;Mercredi\n;\n;\n");

    // Up: 3, 5 selected, dragged two rows up.
    await setText(page, "a\n\n\n3\n5\n");
    await range(page, 2, 0, 3, 0);
    await drag(page, page.locator("[data-fill]"), cell(page, 0, 0));
    await expect.poll(() => readText(page), { timeout: 10_000 }).toBe("a\n-1\n1\n3\n5\n");

    // Left: 5, 7 selected, dragged two columns left.
    await setText(page, "a;b;c;d\n;;5;7\n");
    await range(page, 0, 2, 0, 3);
    await drag(page, page.locator("[data-fill]"), cell(page, 0, 0));
    await expect.poll(() => readText(page), { timeout: 10_000 }).toBe("a;b;c;d\n1;3;5;7\n");
  });

  test("SHEET-DRAG-02 a selected row is moved by dragging its header, as one undo step", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, DATA);
    await rowHead(page, 0).click();
    const from = rowHead(page, 0);
    const target = await rowHead(page, 2).boundingBox();
    const a = await from.boundingBox();
    if (!a || !target) throw new Error("no rows");
    await page.mouse.move(a.x + a.width / 2, a.y + a.height / 2);
    await page.mouse.down();
    await page.mouse.move(a.x + a.width / 2, a.y + 40, { steps: 4 });
    await page.mouse.move(target.x + target.width / 2, target.y + target.height * 0.8, {
      steps: 6,
    });
    await page.mouse.up();
    expect(await readText(page)).toBe(
      "Nom;Qté;Prix\nPoire;12;2,25\nCerise;7;10\nPomme;3;1,50\nFigue;1;4,2\n",
    );
    await grid(page).focus();
    await undo(page);
    expect(await readText(page)).toBe(DATA);
  });

  test("SHEET-DRAG-03 a selected column is moved by dragging its header", async ({ browser }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, DATA);
    await head(page, 0).click();
    const a = await head(page, 0).boundingBox();
    const b = await head(page, 2).boundingBox();
    if (!a || !b) throw new Error("no columns");
    await page.mouse.move(a.x + a.width / 2, a.y + a.height / 2);
    await page.mouse.down();
    await page.mouse.move(a.x + a.width / 2 + 60, a.y + a.height / 2, { steps: 4 });
    await page.mouse.move(b.x + b.width * 0.8, b.y + b.height / 2, { steps: 6 });
    await page.mouse.up();
    expect(await readText(page)).toBe(
      "Qté;Prix;Nom\n3;1,50;Pomme\n12;2,25;Poire\n7;10;Cerise\n1;4,2;Figue\n",
    );
    await grid(page).focus();
    await undo(page);
    expect(await readText(page)).toBe(DATA);
  });

  test("SHEET-DRAG-04 a header border resizes the column; a double-click fits it to its contents", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, DATA);
    const before = (await head(page, 1).boundingBox())?.width ?? 0;
    const box = await head(page, 1).boundingBox();
    if (!box) throw new Error("no header");
    await page.mouse.move(box.x + box.width - 2, box.y + box.height / 2);
    await page.mouse.down();
    await page.mouse.move(box.x + box.width + 78, box.y + box.height / 2, { steps: 5 });
    await page.mouse.up();
    const wide = (await head(page, 1).boundingBox())?.width ?? 0;
    expect(wide).toBeGreaterThan(before + 60);
    const edge = await head(page, 1).boundingBox();
    if (!edge) throw new Error("no header");
    await page.mouse.dblclick(edge.x + edge.width - 2, edge.y + edge.height / 2);
    const fitted = (await head(page, 1).boundingBox())?.width ?? 0;
    expect(fitted).toBeLessThan(wide - 30);
    // A resize is a view: the file did not change.
    expect(await readText(page)).toBe(DATA);
  });
});

test.describe("inserting and deleting rows and columns", () => {
  test("SHEET-ROWS context menu, Ctrl++ and Ctrl+-, several rows, each one undo step", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, DATA);
    await cell(page, 1, 1).click({ button: "right" });
    const menu = page.getByRole("menu", { name: "Actions sur la sélection" });
    await menu.getByRole("menuitem", { name: "Insérer une ligne au-dessus" }).click();
    expect(await readText(page)).toBe(
      "Nom;Qté;Prix\nPomme;3;1,50\n;;\nPoire;12;2,25\nCerise;7;10\nFigue;1;4,2\n",
    );
    // The new row is the one selected.
    await expect(grid(page)).toHaveAttribute("aria-activedescendant", "cell-1-0");
    await grid(page).focus();
    await undo(page);
    expect(await readText(page)).toBe(DATA);

    // Two rows selected by their headers, Ctrl+Plus inserts two.
    await rowHead(page, 1).click();
    await rowHead(page, 2).click({ modifiers: ["Shift"] });
    await page.keyboard.press("ControlOrMeta+Shift+Equal");
    expect(await readText(page)).toBe(
      "Nom;Qté;Prix\nPomme;3;1,50\n;;\n;;\nPoire;12;2,25\nCerise;7;10\nFigue;1;4,2\n",
    );
    await undo(page);

    // Ctrl+Minus deletes the selected rows; the selection stays on a real cell.
    await rowHead(page, 0).click();
    await rowHead(page, 1).click({ modifiers: ["Shift"] });
    await page.keyboard.press("ControlOrMeta+Minus");
    expect(await readText(page)).toBe("Nom;Qté;Prix\nCerise;7;10\nFigue;1;4,2\n");
    await expect(grid(page)).toHaveAttribute("aria-activedescendant", /cell-0-/);
    await undo(page);
    expect(await readText(page)).toBe(DATA);

    // Below, from the menu, and delete from the menu.
    await cell(page, 0, 0).click({ button: "right" });
    await page.getByRole("menuitem", { name: "Insérer une ligne en dessous" }).click();
    expect(await readText(page)).toBe(
      "Nom;Qté;Prix\nPomme;3;1,50\n;;\nPoire;12;2,25\nCerise;7;10\nFigue;1;4,2\n",
    );
    await undo(page);
  });

  test("SHEET-COLS context menu and Ctrl+Minus on whole columns", async ({ browser }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, DATA);
    await head(page, 1).click({ button: "right" });
    await page.getByRole("menuitem", { name: "Insérer une colonne à droite" }).click();
    expect(await readText(page)).toBe(
      "Nom;Qté;;Prix\nPomme;3;;1,50\nPoire;12;;2,25\nCerise;7;;10\nFigue;1;;4,2\n",
    );
    await grid(page).focus();
    await undo(page);
    await head(page, 1).click();
    await head(page, 2).click({ modifiers: ["Shift"] });
    await page.keyboard.press("ControlOrMeta+Minus");
    expect(await readText(page)).toBe("Nom\nPomme\nPoire\nCerise\nFigue\n");
    await undo(page);
    expect(await readText(page)).toBe(DATA);
  });
});

test.describe("keyboard and editing", () => {
  test("SHEET-KEY typing replaces, F2 edits, Escape cancels, Enter and Tab walk, Delete clears, undo", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, DATA);
    await pick(page, 0, 0);
    await page.keyboard.type("Poi");
    await page.keyboard.press("Escape");
    await expect(cell(page, 0, 0)).toHaveText("Pomme");
    await page.keyboard.press("F2");
    await page.keyboard.type("!");
    await page.keyboard.press("Tab");
    await expect(cell(page, 0, 0)).toHaveText("Pomme!");
    await expect(grid(page)).toHaveAttribute("aria-activedescendant", "cell-0-1");
    await page.keyboard.type("4");
    await page.keyboard.press("Enter");
    await expect(grid(page)).toHaveAttribute("aria-activedescendant", "cell-1-1");
    await page.keyboard.press("Shift+Enter");
    await expect(grid(page)).toHaveAttribute("aria-activedescendant", "cell-0-1");
    await page.keyboard.press("Delete");
    await expect(cell(page, 0, 1)).toHaveText("");
    expect(await readText(page)).toBe(
      "Nom;Qté;Prix\nPomme!;;1,50\nPoire;12;2,25\nCerise;7;10\nFigue;1;4,2\n",
    );
    await grid(page).focus();
    await undo(page);
    await undo(page);
    await undo(page);
    expect(await readText(page)).toBe(DATA);
  });

  test("SHEET-KEY Ctrl+arrows, Ctrl+Space, Ctrl+A, Alt+Enter, Ctrl+;, Ctrl+D and the name box", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, DATA);
    await pick(page, 0, 0);
    await page.keyboard.press("ControlOrMeta+ArrowDown");
    await expect(grid(page)).toHaveAttribute("aria-activedescendant", "cell-3-0");
    await page.keyboard.press("ControlOrMeta+ArrowRight");
    await expect(grid(page)).toHaveAttribute("aria-activedescendant", "cell-3-2");
    await page.keyboard.press("ControlOrMeta+Space");
    await expect(page.getByLabel(/^Cellule \(tapez/)).toHaveValue("C2:C5");
    await page.keyboard.press("ControlOrMeta+a");
    await expect(page.getByLabel(/^Cellule \(tapez/)).toHaveValue("A2:C5");

    const nameBox = page.getByLabel(/^Cellule \(tapez/);
    await nameBox.fill("B4");
    await nameBox.press("Enter");
    await expect(grid(page)).toHaveAttribute("aria-activedescendant", "cell-2-1");
    await expect(grid(page)).toBeFocused();

    await page.keyboard.press("F2");
    await page.keyboard.press("Alt+Enter");
    await page.keyboard.type("x");
    await page.keyboard.press("Enter");
    expect(await readText(page)).toContain('"7\nx"');
    await undo(page);

    await pick(page, 0, 1);
    await page.keyboard.press("ControlOrMeta+;");
    await expect(cell(page, 0, 1)).toHaveText(/^\d{2}\/\d{2}\/\d{4}\s*$/);
    await undo(page);
    await range(page, 0, 0, 3, 0);
    await page.keyboard.press("ControlOrMeta+d");
    expect(await readText(page)).toBe(
      "Nom;Qté;Prix\nPomme;3;1,50\nPomme;12;2,25\nPomme;7;10\nPomme;1;4,2\n",
    );
    await undo(page);
  });

  test("CSV-21 a formula typed is stored as text and the interface says it is not calculated", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, DATA);
    await pick(page, 0, 1);
    await page.keyboard.type("=SUM(B1:B3)");
    await expect(page.getByRole("note")).toContainText("ne sont pas calculées");
    await page.keyboard.press("Enter");
    expect(await readText(page)).toContain("Pomme;=SUM(B1:B3);1,50");
    await expect(cell(page, 0, 1)).toHaveText("=SUM(B1:B3)");
  });

  test("CSV-02 / CSV-01 a cell with the separator, quote and newline is quoted on save; an unchanged file is byte for byte the same", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, DATA);
    expect(await readText(page)).toBe(DATA);
    await pick(page, 0, 0);
    await page.keyboard.type('a;b "c"');
    await page.keyboard.press("Enter");
    expect(await readText(page)).toBe(DATA.replace("Pomme", '"a;b ""c"""'));
    await undo(page);
    expect(await readText(page)).toBe(DATA);
  });
});

test.describe("sort, filter, freeze, zoom, dialect", () => {
  test("CSV-11 / CSV-12 sort and filter are views; the file changes only when the sort is applied", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, "n;v\na;1 234,5\nb;2\nc;10\n");
    await head(page, 1).hover();
    await head(page, 1)
      .getByRole("button", { name: /Filtrer la colonne/ })
      .click();
    const menu = page.getByRole("dialog", { name: /Filtrer la colonne/ });
    await menu.getByRole("button", { name: "Trier de Z à A" }).click();
    await expect(cell(page, 0, 0)).toHaveText("a");
    expect(await readText(page)).toBe("n;v\na;1 234,5\nb;2\nc;10\n");
    await page.getByRole("button", { name: "Appliquer le tri au fichier" }).click();
    expect(await readText(page)).toBe("n;v\na;1 234,5\nc;10\nb;2\n");

    // Filter by the value of a cell, then edit a visible cell: the right file row changes.
    await cell(page, 2, 0).click({ button: "right" });
    await page.getByRole("menuitem", { name: "Filtrer par la valeur de cette cellule" }).click();
    await expect(page.getByText(/Filtre actif/)).toBeVisible();
    await pick(page, 0, 1);
    await page.keyboard.type("99");
    await page.keyboard.press("Enter");
    expect(await readText(page)).toBe("n;v\na;1 234,5\nc;10\nb;99\n");
  });

  test("SHEET-FREEZE / SHEET-ZOOM freeze up to the selection; zoom 50 to 200 %", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, DATA);
    await pick(page, 1, 1);
    await page.getByRole("button", { name: "Figer jusqu'à la sélection" }).click();
    await expect(page.getByRole("button", { name: "Libérer les volets" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    const zoom = page.getByTestId("zoom");
    await expect(zoom).toHaveText("100 %");
    await page.getByRole("button", { name: "Agrandir" }).click();
    await expect(zoom).toHaveText("110 %");
    await grid(page).focus();
    await page.keyboard.press("ControlOrMeta+0");
    await expect(zoom).toHaveText("100 %");
    for (let i = 0; i < 20; i++) await page.getByRole("button", { name: "Réduire" }).click();
    await expect(zoom).toHaveText("50 %");
  });

  test("CSV-EDGE-14 the status bar shows how the file is written and converts it once", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, DATA);
    await expect(page.getByTestId("dialect")).toHaveText("Point-virgule, avec en-tête");
    await page.getByTestId("dialect").click();
    await page.getByRole("menuitem", { name: "Convertir en : Tabulation" }).click();
    expect(await readText(page)).toBe(DATA.replaceAll(";", "\t"));
    await undo(page);
    expect(await readText(page)).toBe(DATA);
  });

  test("SHEET-STATUS the status bar sums, averages and counts the selection, in French", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, DATA);
    await range(page, 0, 2, 3, 2);
    await expect(page.getByTestId("stats")).toContainText("Nombre : 4");
    await expect(page.getByTestId("stats")).toContainText("Somme : 17,95");
    await expect(page.getByTestId("stats")).toContainText("Moyenne : 4,4875");
  });
});

test.describe("find, replace and the file's quirks (CSV-EDGE)", () => {
  test("CSV-EDGE-16..19 find in the cells, jump with F3, replace re-quotes, locked structure untouched", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, 'Nom;Note\nPomme;"a;b"\nPoire;pomme verte\nCerise;POMME\n');
    await grid(page).focus();
    await page.keyboard.press("ControlOrMeta+f");
    const find = page.getByRole("search", { name: "Chercher et remplacer dans les cellules" });
    await find.getByPlaceholder("Chercher dans les cellules").fill("pomme");
    await expect(find.getByTestId("find-count")).toHaveText("1 sur 3");
    // The separator of the file is not a match.
    await find.getByPlaceholder("Chercher dans les cellules").fill(";");
    await expect(find.getByTestId("find-count")).toHaveText("1 sur 1");
    await find.getByPlaceholder("Chercher dans les cellules").fill("pomme");
    await page.keyboard.press("F3");
    await expect(grid(page)).toHaveAttribute("aria-activedescendant", "cell-1-1");
    await find.getByRole("button", { name: /Afficher le remplacement/ }).click();
    await find.getByPlaceholder("Remplacer par").fill("x;y");
    await find.getByRole("button", { name: /Tout remplacer/ }).click();
    expect(await readText(page)).toBe('Nom;Note\n"x;y";"a;b"\nPoire;"x;y verte"\nCerise;"x;y"\n');
    await undo(page);
    expect(await readText(page)).toBe('Nom;Note\nPomme;"a;b"\nPoire;pomme verte\nCerise;POMME\n');
  });

  test("CSV-EDGE-04 / EDGE-01 a badly quoted file is flagged with its line and left byte for byte as it is; the separator is said to be guessed", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL);
    const text = 'a;b\n1;"never closed\n3;4\n5;6\n';
    await setText(page, text);
    await expect(page.getByTestId("problems")).toContainText("ligne 2");
    expect(await readText(page)).toBe(text);
    await page.getByTestId("dialect").click();
    await expect(page.getByTestId("confidence")).toBeVisible();
  });

  test("a double-click edits the cell, Enter commits it", async ({ browser }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, DATA);
    await cell(page, 1, 0).dblclick();
    const editor = page.getByRole("textbox", { name: /Modifier la cellule/ });
    await expect(editor).toBeFocused();
    await editor.fill("Poire Williams");
    await editor.press("Enter");
    await expect(cell(page, 1, 0)).toHaveText("Poire Williams");
  });
});

test.describe("switching to the text view (CSV-15)", () => {
  test("the same position on both sides, and undo is shared", async ({ browser }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, DATA);
    await pick(page, 2, 1);
    await page.keyboard.type("77");
    await page.keyboard.press("Enter");
    await pick(page, 2, 1);
    await page.getByRole("button", { name: "Voir le texte" }).click();
    // The caret is in the cell's text.
    const line = await page.evaluate(() => {
      const sel = document.getSelection();
      return sel?.anchorNode?.parentElement?.closest(".cm-line")?.textContent ?? "";
    });
    expect(line).toBe("Cerise;77;10");
    await page.getByRole("button", { name: "Voir le tableau" }).click();
    await expect(grid(page)).toHaveAttribute("aria-activedescendant", "cell-2-1");
    await grid(page).focus();
    await undo(page);
    await expect(cell(page, 2, 1)).toHaveText("7");
  });
});

test.describe("accessibility, security and looks (CSV-20)", () => {
  test("CSV-20 ARIA grid, keyboard navigation, spoken cell, no axe violation, no CSP violation", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, DATA);
    await pick(page, 1, 2);
    const g = grid(page);
    await expect(g).toHaveAttribute("aria-rowcount", "5");
    await expect(g).toHaveAttribute("aria-colcount", "3");
    await expect(page.getByRole("row").first()).toHaveAttribute("aria-rowindex", "1");
    await expect(cell(page, 1, 2)).toHaveAttribute("role", "gridcell");
    await expect(cell(page, 1, 2)).toHaveAttribute("aria-colindex", "3");
    await expect(page.getByRole("columnheader", { name: /Prix/ })).toBeVisible();
    await expect(page.locator('[aria-live="polite"]').filter({ hasText: "ligne" })).toHaveText(
      "ligne 3, colonne Prix, 2,25",
    );
    const result = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
    expect(result.violations.map((v) => `${v.id}: ${v.nodes.length}`)).toEqual([]);
    expect(cspErrors(page)).toEqual([]);
  });

  for (const dark of [false, true]) {
    test(`screenshots (${dark ? "dark" : "light"}): selection, fill handle, filter dropdown, context menu`, async ({
      browser,
    }) => {
      const page = await openSheet(browser, ALICE_URL, { dark, width: 1100, height: 640 });
      await setText(
        page,
        "Fruit;Quantité;Prix;Date\nPomme;3;1,50;2026-10-01\nPoire;12;2,25;2026-10-02\nCerise;7;10;2026-10-03\nFigue;1;4,2;2026-10-04\nPrune;24;3,1;2026-10-05\n",
      );
      await range(page, 1, 1, 3, 2);
      await page.screenshot({ path: `${SHOTS}/sheet-selection-${dark ? "dark" : "light"}.png` });
      await head(page, 1).hover();
      await head(page, 1)
        .getByRole("button", { name: /Filtrer la colonne/ })
        .click();
      await expect(page.getByRole("dialog", { name: /Filtrer la colonne/ })).toBeVisible();
      // The numbers sit at the right edge of their column, under the menu in the picture: they
      // are there (the column is not empty), just covered.
      await expect(cell(page, 0, 1)).toHaveText("3");
      await expect(cell(page, 4, 1)).toHaveText("24");
      await page.screenshot({ path: `${SHOTS}/sheet-filter-${dark ? "dark" : "light"}.png` });
      await page.keyboard.press("Escape");
      await cell(page, 2, 1).click({ button: "right" });
      await expect(page.getByRole("menu", { name: "Actions sur la sélection" })).toBeVisible();
      await page.screenshot({ path: `${SHOTS}/sheet-context-${dark ? "dark" : "light"}.png` });
      await page.keyboard.press("Escape");
      expect(cspErrors(page)).toEqual([]);
    });
  }
});

test.describe("a file too big to edit (SHEET-BIG)", () => {
  // PERF-02 (ADR 0028): a file of 12 MB is on the disk. It is too big to enter a document, so it
  // is read from there as one plain string: no Automerge document ever holds it (that took more
  // than 8 GB of memory). Its own instance leaves no history behind in the shared ones.
  test("a 12 MB CSV on the disk opens read-only: view, sort, filter, search and copy work, editing does not", async ({
    browser,
  }) => {
    test.setTimeout(240_000);
    const docs = mkdtempSync(join(tmpdir(), "inkwell-e2e-big-"));
    const instance = await startInstance(SPARE_PORT, docs);
    try {
      const page = await openSheet(browser, SPARE_URL, { clipboard: true });
      const rows = 500_000; // about 12 MB
      const csv = `nom;qte;prix\n${Array.from({ length: rows }, (_, i) => `ligne ${i};${i % 97};${i},5`).join("\n")}\n`;
      // The project folder exists once the instance runs: another program writes the file.
      await expect.poll(() => readdirSync(docs).filter((d) => !d.startsWith(".")).length).toBe(1);
      const dir = join(docs, readdirSync(docs).filter((d) => !d.startsWith("."))[0] ?? "");
      writeFileSync(join(dir, "main.typ"), csv);
      await expect(cell(page, 0, 0)).toHaveText("ligne 0", { timeout: 120_000 });

      // Said clearly, and the toolbar's editing buttons are off.
      await expect(page.getByTestId("readonly")).toHaveText(
        /^Fichier trop grand pour l'édition à plusieurs \(1\d(,\d)? Mo\) : ouvert en lecture seule$/,
      );
      await expect(grid(page)).toHaveAttribute("aria-rowcount", String(rows + 1));
      await expect(page.getByRole("button", { name: "Insérer une ligne" })).toBeDisabled();

      // Editing is refused: typing, Enter on a cell, Delete.
      await pick(page, 5, 0);
      await page.keyboard.type("x");
      await page.keyboard.press("Enter");
      await page.keyboard.press("Delete");
      await expect(cell(page, 5, 0)).toHaveText("ligne 5");
      await expect(
        page.getByRole("status").filter({ hasText: "Fichier trop grand" }),
      ).toBeVisible();

      // Copy.
      await pick(page, 5, 0);
      await page.keyboard.press("ControlOrMeta+c");
      await expect.poll(() => clipboardText(page)).toBe("ligne 5");

      // Search.
      await grid(page).focus();
      await page.keyboard.press("ControlOrMeta+f");
      const find = page.getByRole("search", { name: "Chercher et remplacer dans les cellules" });
      await find.getByPlaceholder("Chercher dans les cellules").fill("ligne 4242");
      await expect(find.getByTestId("find-count")).toHaveText(/^1 sur \d+$/);
      await page.keyboard.press("Escape");

      // Filter, then sort.
      await cell(page, 4, 1).click({ button: "right" });
      await page.getByRole("menuitem", { name: "Filtrer par la valeur de cette cellule" }).click();
      await expect(page.getByText(/Filtre actif/)).toBeVisible();
      await expect(cell(page, 0, 1)).toHaveText("4");
      await page.getByRole("button", { name: "Retirer le tri et le filtre" }).click();
      await pick(page, 0, 2);
      await page.getByRole("button", { name: "Trier de Z à A" }).click();
      await expect(cell(page, 0, 0)).toHaveText(`ligne ${rows - 1}`, { timeout: 30_000 });
      expect(cspErrors(page)).toEqual([]);
    } finally {
      await stopInstance(instance);
      rmSync(docs, { recursive: true, force: true });
    }
  });
});

test.describe("performance in the real browser (SHEET-PERF)", () => {
  // A 5 MB document is not left behind for the next test, pass or fail.
  test.afterEach(async ({ browser }) => {
    const page = await open(browser, ALICE_URL);
    await clear(page);
    await page.context().close();
  });

  // The document is an Automerge text, which the browser holds whole: 1.5 MB here (the model alone
  // is measured on 130 MB in src/editor/csv/perf.test.ts). Logged; asserted with INKWELL_PERF=1.
  test("SHEET-PERF scrolling, moving, selecting and editing on 12 000 rows x 12 columns", async ({
    browser,
  }) => {
    // A 1.5 MB document in the shared instances weighs on every spec after this one (their
    // storage keeps its history): measured on request, with INKWELL_PERF=1.
    test.skip(process.env.INKWELL_PERF !== "1", "run with INKWELL_PERF=1");
    test.setTimeout(240_000);
    const page = await openSheet(browser, ALICE_URL, { width: 1280, height: 800 });
    const rows = 12_000;
    const header = Array.from({ length: 12 }, (_, c) => `col${c}`).join(";");
    const body = Array.from({ length: rows }, (_, r) =>
      Array.from({ length: 12 }, (_, c) =>
        c % 3 === 0 ? `${r},${c}` : `texte ${c} ligne ${r}`,
      ).join(";"),
    ).join("\n");
    const csv = `${header}\n${body}\n`;
    await pasteInTextView(page, csv);
    const t0 = Date.now();
    await page.getByRole("button", { name: "Voir le tableau" }).click();
    await expect(cell(page, 0, 0)).toBeVisible({ timeout: 30_000 });
    const opened = Date.now() - t0;

    const cdp = await page.context().newCDPSession(page);
    await cdp.send("Performance.enable");
    const metrics = async () =>
      Object.fromEntries(
        (await cdp.send("Performance.getMetrics")).metrics.map((x) => [x.name, x.value]),
      ) as Record<string, number>;
    const before = await metrics();
    const m = await page.evaluate(async () => {
      const grid = document.querySelector('[role="grid"]') as HTMLElement;
      const frame = () => new Promise<number>((r) => requestAnimationFrame((t) => r(t)));
      const stats = (xs: number[]) => {
        const a = [...xs].sort((x, y) => x - y);
        return {
          median: a[a.length >> 1] ?? 0,
          p95: a[Math.floor(a.length * 0.95)] ?? 0,
          max: a[a.length - 1] ?? 0,
        };
      };
      // Scrolling: 150 frames, each a jump of 900 px.
      const scroll: number[] = [];
      let last = await frame();
      for (let i = 0; i < 150; i++) {
        grid.scrollTop += 900;
        const now = await frame();
        scroll.push(now - last);
        last = now;
      }
      grid.scrollTop = 0;
      await frame();
      // Moving the active cell: the time from the key to the frame that shows it.
      grid.focus();
      const move: number[] = [];
      for (let i = 0; i < 60; i++) {
        const t = performance.now();
        grid.dispatchEvent(
          new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true, cancelable: true }),
        );
        await frame();
        move.push(performance.now() - t);
      }
      // Selecting a range by Shift+arrows.
      const select: number[] = [];
      for (let i = 0; i < 40; i++) {
        const t = performance.now();
        grid.dispatchEvent(
          new KeyboardEvent("keydown", {
            key: "ArrowRight",
            shiftKey: true,
            bubbles: true,
            cancelable: true,
          }),
        );
        grid.dispatchEvent(
          new KeyboardEvent("keydown", {
            key: "ArrowDown",
            shiftKey: true,
            bubbles: true,
            cancelable: true,
          }),
        );
        await frame();
        select.push(performance.now() - t);
      }
      return { scroll: stats(scroll), move: stats(move), select: stats(select) };
    });

    const after = await metrics();
    const perFrame = (k: string) =>
      Math.round(((((after[k] ?? 0) - (before[k] ?? 0)) * 1000) / 250) * 100) / 100;
    const cpu = {
      script_ms: perFrame("ScriptDuration"),
      layout_ms: perFrame("LayoutDuration"),
      style_ms: perFrame("RecalcStyleDuration"),
      task_ms: perFrame("TaskDuration"),
    };
    // Editing a cell: type and commit, timed from the key to the new text on screen.
    await grid(page).focus();
    await page.keyboard.press("ControlOrMeta+Home");
    await pick(page, 5, 1);
    const t1 = Date.now();
    await page.keyboard.type("edited");
    await page.keyboard.press("Enter");
    await expect(cell(page, 5, 1)).toContainText("edited");
    const edit = Date.now() - t1;
    const report = { rows, opened_ms: opened, ...m, cpu_per_frame: cpu, edit_roundtrip_ms: edit };
    console.info(`SHEET-PERF ${JSON.stringify(report)}`);
    test.info().annotations.push({ type: "SHEET-PERF", description: JSON.stringify(report) });
    if (process.env.INKWELL_PERF === "1") {
      expect(m.scroll.median).toBeLessThan(17.5);
      expect(m.move.p95).toBeLessThan(33);
      expect(m.select.p95).toBeLessThan(33);
      expect(opened).toBeLessThan(3000);
    }
    expect(cspErrors(page)).toEqual([]);
  });
});

test.describe("two people on one sheet (SHEET-MU)", () => {
  const SEED = "a;b;c;d\n1;2;3;4\n5;6;7;8\n9;10;11;12\n";

  async function both(browser: import("@playwright/test").Browser) {
    const alice = await openSheet(browser, ALICE_URL, { width: 1100, height: 640 });
    const bob = await openSheet(browser, BOB_URL, { dark: true, width: 1100, height: 640 });
    await setText(alice, "");
    await pair(alice, bob);
    await setText(alice, SEED);
    await expect(cell(bob, 1, 3)).toContainText("8");
    return { alice, bob };
  }

  /** Both texts equal, and every record has the same number of cells. */
  async function converged(alice: Page, bob: Page, width: number) {
    await expect
      .poll(async () => (await readText(alice)) === (await readText(bob)), { timeout: 15_000 })
      .toBe(true);
    const text = await readText(alice);
    for (const line of text.split("\n").filter(Boolean)) {
      // A cell with a quoted separator would split wrongly here: the seed has none.
      const quoted = (line.match(/"/g) ?? []).length;
      if (quoted === 0) expect(line.split(";").length, JSON.stringify(text)).toBe(width);
    }
    return text;
  }

  test("SHEET-MU-01 presence: each one sees the other's cell as a coloured rectangle with a name, and it goes when they leave", async ({
    browser,
  }) => {
    const { alice, bob } = await both(browser);
    await range(alice, 1, 1, 2, 2);
    const rect = bob.locator(".cell.rem");
    await expect(rect.first()).toBeVisible({ timeout: 10_000 });
    await expect(rect).toHaveCount(4);
    await expect(bob.locator(".tag", { hasText: "Alice" })).toBeVisible();
    await bob.screenshot({ path: `${SHOTS}/sheet-presence-bob.png` });
    await range(bob, 0, 0, 0, 0);
    await expect(alice.locator(".tag", { hasText: "Bob" })).toBeVisible({ timeout: 10_000 });
    // Alice goes: her rectangle goes.
    await alice.close({ runBeforeUnload: true });
    await expect(bob.locator(".cell.rem")).toHaveCount(0, { timeout: 20_000 });
  });

  test("SHEET-MU-02 same row, different cells; same cell; insert row vs edit below", async ({
    browser,
  }) => {
    const { alice, bob } = await both(browser);
    const type = async (p: Page, r: number, c: number, v: string) => {
      await pick(p, r, c);
      await p.keyboard.type(v);
      await p.keyboard.press("Enter");
    };
    await Promise.all([type(alice, 1, 1, "B3"), type(bob, 1, 3, "D3")]);
    let text = await converged(alice, bob, 4);
    expect(text).toContain("B3");
    expect(text).toContain("D3");

    await Promise.all([type(alice, 0, 0, "from,alice"), type(bob, 0, 0, 'from "bob"')]);
    text = await converged(alice, bob, 4);
    expect(text.split("\n")[1]?.split(";").length).toBeGreaterThanOrEqual(1);

    await cell(alice, 0, 0).click({ button: "right" });
    await Promise.all([
      alice.getByRole("menuitem", { name: "Insérer une ligne au-dessus" }).click(),
      type(bob, 2, 2, "below"),
    ]);
    text = await converged(alice, bob, 4);
    expect(text).toContain("below");
  });

  test("SHEET-MU-02 delete a column vs an edit in it; paste vs typing; sort vs edit; fill vs edit", async ({
    browser,
  }) => {
    const { alice, bob } = await both(browser);
    const type = async (p: Page, r: number, c: number, v: string) => {
      await pick(p, r, c);
      await p.keyboard.type(v);
      await p.keyboard.press("Enter");
    };
    // Delete column B while Bob edits inside it.
    await head(alice, 1).click();
    await Promise.all([alice.keyboard.press("ControlOrMeta+Minus"), type(bob, 1, 1, "lost")]);
    await converged(alice, bob, 3);
    await undo(alice).catch(() => {});

    // Paste a range while Bob types in it.
    await setText(alice, SEED);
    await expect(cell(bob, 1, 3)).toContainText("8");
    await alice
      .context()
      .grantPermissions(["clipboard-read", "clipboard-write"])
      .catch(() => {});
    await pick(alice, 0, 0);
    await alice.evaluate(() => {
      const dt = new DataTransfer();
      dt.setData("text/plain", "p1\tp2\np3\tp4");
      document
        .querySelector('[role="grid"]')
        ?.dispatchEvent(
          new ClipboardEvent("paste", { clipboardData: dt, bubbles: true, cancelable: true }),
        );
    });
    await type(bob, 1, 1, "typing");
    await converged(alice, bob, 4);

    // Sort applied to the file vs an edit.
    await setText(alice, SEED);
    await expect(cell(bob, 1, 3)).toContainText("8");
    await head(alice, 0).hover();
    await head(alice, 0)
      .getByRole("button", { name: /Filtrer la colonne/ })
      .click();
    await alice
      .getByRole("dialog", { name: /Filtrer la colonne/ })
      .getByRole("button", { name: "Trier de Z à A" })
      .click();
    await Promise.all([
      alice.getByRole("button", { name: "Appliquer le tri au fichier" }).click(),
      type(bob, 0, 3, "edited"),
    ]);
    await converged(alice, bob, 4);

    // Fill handle vs an edit.
    await setText(alice, SEED);
    await expect(cell(bob, 1, 3)).toContainText("8");
    await range(alice, 0, 0, 1, 0);
    await Promise.all([
      drag(alice, alice.locator("[data-fill]"), cell(alice, 2, 0)),
      type(bob, 2, 3, "z"),
    ]);
    await converged(alice, bob, 4);
  });

  test("SHEET-MU-03 undo is per person: Ctrl+Z only undoes my own action", async ({ browser }) => {
    const { alice, bob } = await both(browser);
    await pick(alice, 0, 0);
    await alice.keyboard.type("AAA");
    await alice.keyboard.press("Enter");
    await pick(bob, 2, 2);
    await bob.keyboard.type("BBB");
    await bob.keyboard.press("Enter");
    await expect(cell(alice, 2, 2)).toContainText("BBB");
    await grid(alice).focus();
    await alice.keyboard.press("ControlOrMeta+z");
    await expect(cell(alice, 0, 0)).toContainText("1");
    await expect(cell(alice, 2, 2)).toContainText("BBB");
    await expect(cell(bob, 0, 0)).toContainText("1");
    await expect(cell(bob, 2, 2)).toContainText("BBB");
  });

  test.fixme("SHEET-MU-04..07 roles, locked zones, suspension and history entries: need B3 (rights, ADR 0021/0022) and B2 (history); the sheet already refuses edits when `readOnly` / `isLocked` are given (unit tests CSV-18, CSV-19)", () => {});
  test("CSV-DIALECT a .csv that is really tab-separated says so, offers the choice, and remembers it", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL);
    await setText(page, "nom\tqte\nA\t1\nB\t2\nC\t3\n");
    await page.reload();
    await expect(grid(page)).toBeVisible();
    await expect(page.getByTestId("detected")).toHaveText(
      /Séparateur détecté : tabulation \(l'extension indique virgule\)/,
    );
    await expect(cell(page, 0, 1)).toHaveText("1");
    await page.getByRole("button", { name: "Séparer par tabulation" }).click();
    await expect(page.getByTestId("detected")).toBeHidden();
    await page.reload();
    await expect(grid(page)).toBeVisible();
    await expect(page.getByTestId("detected")).toBeHidden();
    await expect(page.getByTestId("dialect")).toContainText("Tabulation");
  });

  test("CSV-14 a file edited outside Inkwell is merged, its cells highlighted; Ctrl+Z takes it back", async ({
    browser,
  }) => {
    const page = await openSheet(browser, ALICE_URL, {
      settings: { saveMode: "auto", saveDelay: 1, externalPolicy: "import" },
    });
    await setText(page, DATA);
    // The file is made at the first save: the document is projected into it.
    let file = "";
    await expect
      .poll(
        () => {
          try {
            file = mainTypIn(join(docsRoot, String(ALICE_PORT)));
          } catch {
            return undefined;
          }
          return readDisk(file);
        },
        { timeout: 10_000 },
      )
      .toBe(DATA);

    // My own edit first: the outside edit lands next to it, and both stay.
    await pick(page, 0, 0);
    await page.keyboard.type("Mine");
    await page.keyboard.press("Enter");
    writeFileSync(file, DATA.replace("Poire;12;2,25", "Poire;99;2,25"));
    await expect(cell(page, 1, 1)).toHaveText("99");
    await expect(cell(page, 1, 1)).toHaveClass(/fresh/);
    await expect(cell(page, 2, 1)).not.toHaveClass(/fresh/);
    await expect(cell(page, 0, 0)).toHaveText("Mine");
    const toast = page.getByRole("status").filter({ hasText: "Modifié hors d'Inkwell" });
    await expect(toast).toContainText("intégrée");

    // "Voir" goes to the changed cell.
    await pick(page, 3, 2);
    await toast.getByRole("button", { name: "Voir" }).click();
    await expect(grid(page)).toHaveAttribute("aria-activedescendant", "cell-1-1");

    // The file follows what is in the document, and Ctrl+Z takes the outside edit back alone.
    await expect.poll(() => readDisk(file), { timeout: 10_000 }).toContain("Poire;99;");
    await undo(page);
    await expect(cell(page, 1, 1)).toHaveText("12");
    await expect(cell(page, 0, 0)).toHaveText("Mine");
    await expect.poll(() => readDisk(file), { timeout: 10_000 }).toContain("Poire;12;");
    expect(cspErrors(page)).toEqual([]);
  });
  test.fixme("CSV-16 / CSV-17 Typst preview of `csv()` tables and charts: needs wave C (Typst sidecar and the plotting package); the code is generated and unit-tested", () => {});
});
