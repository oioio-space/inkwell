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
  opts: {
    dark?: boolean;
    width?: number;
    height?: number;
    /** Preferences already chosen, as the Preferences dialog would store them. */
    settings?: Record<string, unknown>;
  } = {},
): Promise<Page> {
  const context = await browser.newContext({
    locale: "fr-FR",
    viewport: { width: opts.width ?? 1200, height: opts.height ?? 720 },
    colorScheme: opts.dark ? "dark" : "light",
    // Only Chromium has a permission to give (WebKit rejects the name): there, the app's own
    // writes to the clipboard (a button) need it. Reading and writing for the tests goes through
    // real copy and paste, in both.
    permissions: browser.browserType().name() === "chromium" ? ["clipboard-write"] : [],
  });
  if (opts.settings)
    await context.addInitScript((value) => {
      localStorage.setItem("inkwell.settings", JSON.stringify(value));
    }, opts.settings);
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
  await expect(grid(page)).toBeVisible({ timeout: 20_000 });
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

/**
 * Replaces the whole file by `csv` with ONE paste in the text view (typing a big file takes
 * forever), and leaves the caret at its start. The text view stays open.
 */
export async function pasteInTextView(p: Page, csv: string): Promise<void> {
  await p.getByRole("button", { name: "Voir le texte" }).click();
  const content = p.locator(".cm-content");
  await content.click();
  await p.keyboard.press("ControlOrMeta+a");
  await p.keyboard.press("Delete");
  await p.evaluate((text) => {
    const dt = new DataTransfer();
    dt.setData("text/plain", text);
    document
      .querySelector(".cm-content")
      ?.dispatchEvent(
        new ClipboardEvent("paste", { clipboardData: dt, bubbles: true, cancelable: true }),
      );
  }, csv);
  // The caret is at the end of what came in: wait for it to show, then go back to the start.
  await expect(p.locator(".cm-line").first()).not.toBeEmpty({ timeout: 60_000 });
  await p.keyboard.press("ControlOrMeta+Home");
  await expect(p.locator(".cm-line").first()).toContainText(csv.slice(0, csv.indexOf("\n")), {
    timeout: 60_000,
  });
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

/*
 * The system clipboard, through real copy and paste: the Clipboard API needs a permission that
 * only Chromium can be granted (WebKit has none), so these go through a scratch field and the
 * keyboard, which is also what a person does. The field is removed afterwards.
 */

/** What a paste gives a page: the text and, when there is one, the HTML. */
export async function readClipboard(p: Page): Promise<{ text: string; html: string }> {
  await p.evaluate(() => {
    const field = document.createElement("textarea");
    field.id = "e2e-clipboard";
    (window as unknown as { __was: unknown }).__was = document.activeElement;
    field.addEventListener("paste", (e) => {
      e.preventDefault();
      const d = e.clipboardData;
      (window as unknown as { __clip: unknown }).__clip = {
        text: d?.getData("text/plain") ?? "",
        html: d?.getData("text/html") ?? "",
      };
    });
    document.body.append(field);
    field.focus();
  });
  await p.keyboard.press("ControlOrMeta+v");
  const got = await p.evaluate(() => {
    document.getElementById("e2e-clipboard")?.remove();
    (window as unknown as { __was?: HTMLElement }).__was?.focus();
    const w = window as unknown as { __clip?: { text: string; html: string } };
    const clip = w.__clip ?? { text: "", html: "" };
    w.__clip = undefined;
    return clip;
  });
  return got;
}

export const clipboardText = async (p: Page) => (await readClipboard(p)).text;

/** Puts `text` (and `html`, if given) on the system clipboard. */
export async function setClipboard(p: Page, text: string, html?: string): Promise<void> {
  await p.evaluate(
    ([t, h]) => {
      const field = document.createElement("textarea");
      field.id = "e2e-clipboard";
      (window as unknown as { __was: unknown }).__was = document.activeElement;
      field.value = "x";
      field.addEventListener("copy", (e) => {
        e.preventDefault();
        e.clipboardData?.setData("text/plain", t ?? "");
        if (h) e.clipboardData?.setData("text/html", h);
      });
      document.body.append(field);
      field.focus();
      field.select();
    },
    [text, html] as const,
  );
  await p.keyboard.press("ControlOrMeta+c");
  await p.evaluate(() => {
    document.getElementById("e2e-clipboard")?.remove();
    (window as unknown as { __was?: HTMLElement }).__was?.focus();
  });
}

/** Content-Security-Policy violations seen by the page so far. */
export const cspErrors = (p: Page) => (p as Page & { csp?: string[] }).csp ?? [];

export const DATA = "Nom;Qté;Prix\nPomme;3;1,50\nPoire;12;2,25\nCerise;7;10\nFigue;1;4,2\n";

/**
 * A screenshot for the eye. Playwright's WebKit injects a stylesheet to take it, which the page's
 * CSP refuses and logs: that is the tool, not the app, so it is not counted as a violation.
 */
export async function shot(p: Page, path: string): Promise<void> {
  const seen = cspErrors(p).length;
  await p.screenshot({ path });
  cspErrors(p).length = seen;
}
