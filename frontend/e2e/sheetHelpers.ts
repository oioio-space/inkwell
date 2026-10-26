import { type Browser, expect, type Page } from "@playwright/test";

/** The grid of a sheet and one of its cells (by displayed row and column, from 0). */
export const grid = (p: Page) => p.getByRole("grid");
export const cell = (p: Page, r: number, c: number) => p.locator(`#cell-${r}-${c}`);
export const head = (p: Page, c: number) => p.locator(`[data-colhead="${c}"]`);
export const rowHead = (p: Page, r: number) => p.locator(`[data-rowhead="${r}"]`).first();

/** Opens one instance with `data.csv` as its file: the editor area shows it as a sheet. */
export async function openSheet(
  browser: Browser,
  url: string,
  opts: { dark?: boolean; clipboard?: boolean; width?: number; height?: number } = {},
): Promise<Page> {
  const context = await browser.newContext({
    locale: "fr-FR",
    viewport: { width: opts.width ?? 1200, height: opts.height ?? 720 },
    colorScheme: opts.dark ? "dark" : "light",
    permissions: opts.clipboard ? ["clipboard-read", "clipboard-write"] : [],
  });
  const page = await context.newPage();
  const violations: string[] = [];
  page.on("console", (m) => {
    if (m.type() === "error" && /Content Security Policy|Refused to/.test(m.text()))
      violations.push(m.text());
  });
  // The app's own store stream closing while a page goes away is not a sheet problem.
  page.on("pageerror", (e) => {
    if (!/store stream closed/.test(e.message)) violations.push(`pageerror: ${e.message}`);
  });
  (page as Page & { csp?: string[] }).csp = violations;
  await page.goto(`${url}?file=data.csv`);
  await expect(grid(page)).toBeVisible();
  return page;
}

/** The text of the file as the text view shows it, newlines included. */
async function textView(p: Page): Promise<string> {
  const lines = p.locator(".cm-content .cm-line");
  await expect(lines.first()).toBeVisible();
  return lines.evaluateAll((els) =>
    els
      .map((l) =>
        Array.from(l.childNodes)
          .map((n) =>
            (n as HTMLElement).classList?.contains("cm-remote-caret") ||
            (n as HTMLElement).classList?.contains("cm-placeholder")
              ? ""
              : (n.textContent ?? ""),
          )
          .join("")
          .replace(/ /g, " ")
          .replace(/[​⁠]/g, ""),
      )
      .join("\n"),
  );
}

/** Reads the file's text through the text view, then comes back to the table. */
export async function readText(p: Page): Promise<string> {
  await p.getByRole("button", { name: "Voir le texte" }).click();
  const text = await textView(p);
  await p.getByRole("button", { name: "Voir le tableau" }).click();
  await expect(grid(p)).toBeVisible();
  return text;
}

/** Replaces the whole file by `csv` (typed in the text view), then shows the table. */
export async function setText(p: Page, csv: string): Promise<void> {
  await p.getByRole("button", { name: "Voir le texte" }).click();
  const content = p.locator(".cm-content");
  await expect(content).toBeVisible();
  await content.click();
  await p.keyboard.press("ControlOrMeta+a");
  await p.keyboard.press("Delete");
  if (csv) await p.keyboard.insertText(csv);
  await expect.poll(() => textView(p)).toBe(csv);
  await p.getByRole("button", { name: "Voir le tableau" }).click();
  await expect(grid(p)).toBeVisible();
  if (csv) await expect(cell(p, 0, 0)).toBeVisible();
}

/** Clicks a cell and checks it is the active one. */
export async function pick(p: Page, r: number, c: number) {
  await cell(p, r, c).click();
  await expect(grid(p)).toHaveAttribute("aria-activedescendant", `cell-${r}-${c}`);
}

/** Drags with the real mouse from the centre of one element to the centre of another. */
export async function drag(
  p: Page,
  from: ReturnType<Page["locator"]>,
  to: ReturnType<Page["locator"]>,
) {
  const a = await from.boundingBox();
  const b = await to.boundingBox();
  if (!a || !b) throw new Error("nothing to drag");
  await p.mouse.move(a.x + a.width / 2, a.y + a.height / 2);
  await p.mouse.down();
  await p.mouse.move((a.x + b.x) / 2 + a.width / 2, (a.y + b.y) / 2 + a.height / 2, { steps: 4 });
  await p.mouse.move(b.x + b.width / 2, b.y + b.height / 2, { steps: 6 });
  await p.mouse.up();
}

export const clipboardText = (p: Page) => p.evaluate(() => navigator.clipboard.readText());
export const setClipboard = (p: Page, text: string, html?: string) =>
  p.evaluate(
    async ([t, h]) => {
      const data: Record<string, Blob> = {
        "text/plain": new Blob([t ?? ""], { type: "text/plain" }),
      };
      if (h) data["text/html"] = new Blob([h], { type: "text/html" });
      await navigator.clipboard.write([new ClipboardItem(data)]);
    },
    [text, html] as const,
  );

/** Content-Security-Policy violations seen by the page so far. */
export const cspErrors = (p: Page) => (p as Page & { csp?: string[] }).csp ?? [];

export const DATA = "Nom;Qté;Prix\nPomme;3;1,50\nPoire;12;2,25\nCerise;7;10\nFigue;1;4,2\n";
