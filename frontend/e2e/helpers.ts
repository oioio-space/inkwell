import { type Browser, expect, type Page } from "@playwright/test";

export const editor = (p: Page) => p.locator(".cm-content");

/** The document text as typed, without remote-cursor name labels. */
export const docText = (p: Page) =>
  editor(p).evaluate((el) => {
    const copy = el.cloneNode(true) as HTMLElement;
    for (const label of copy.querySelectorAll(".cm-remote-caret, .cm-placeholder")) label.remove();
    // CodeMirror renders trailing spaces as NBSP and may insert zero-width breaks.
    return copy.innerText.replace(/ /g, " ").replace(/[​⁠\n]/g, "");
  });

export async function rename(p: Page, name: string) {
  const field = p.getByLabel(/Votre nom/);
  await field.fill(name);
  await field.blur();
}

/** Opens one instance in a fresh browser context, French interface. */
export async function open(browser: Browser, url: string, dark = false): Promise<Page> {
  const page = await (
    await browser.newContext({
      locale: "fr-FR",
      viewport: { width: 1100, height: 640 },
      colorScheme: dark ? "dark" : "light",
    })
  ).newPage();
  await page.goto(url);
  await expect(editor(page)).toBeVisible();
  return page;
}

/**
 * Opens "Partager" on `host` (naming them `name`, if given), asks for an invitation and returns
 * its text. An invitation works once, and the dialog keeps showing the last one after it is
 * closed: a new one is awaited, never the stale one (a third person given the second's used
 * invitation never connects). Asking is retried: the node refuses until it has an address to put
 * in the ticket (through a relay, after its reservation).
 */
export async function freshInvitation(host: Page, name?: string): Promise<string> {
  await host.getByRole("button", { name: "Partager" }).click();
  if (name) await rename(host, name);
  const field = host.getByLabel("Invitation", { exact: true });
  const stale = (await field.count()) > 0 ? await field.inputValue() : "";
  await expect(async () => {
    await host.getByRole("button", { name: "Inviter" }).click();
    await expect(field).toHaveValue(/^inkwell1:/, { timeout: 1_000 });
    expect(await field.inputValue()).not.toBe(stale);
  }).toPass({ timeout: 60_000 });
  const text = await field.inputValue();
  await host.getByRole("button", { name: "Fermer" }).click();
  return text;
}

/** Alice invites, Bob joins with the invitation text; both end up "Synchronisé". */
export async function pair(alice: Page, bob: Page) {
  const text = await freshInvitation(alice, "Alice");

  await bob.getByRole("button", { name: "Partager" }).click();
  await rename(bob, "Bob");
  await bob.getByPlaceholder("Collez l'invitation ici").fill(text);
  await bob.getByRole("button", { name: "Rejoindre" }).click();

  await expect(alice.getByRole("status")).toHaveText(/Synchronisé/, { timeout: 60_000 });
  await expect(bob.getByRole("status")).toHaveText(/Synchronisé/, { timeout: 60_000 });

  // The header and the status bar come from the same list of people: they never disagree.
  for (const page of [alice, bob]) {
    const bar = page.getByRole("contentinfo", { name: "Barre d'état" });
    await expect(bar).toContainText("2 personnes");
    await expect(bar).not.toContainText("Seul");
    await expect(page.getByRole("status")).not.toHaveText(/Seul/);
  }
}

/** Types at the end of the document. */
export async function typeAtEnd(p: Page, text: string) {
  await editor(p).click();
  await p.keyboard.press("ControlOrMeta+End");
  await p.keyboard.type(text);
}

/** Starts from an empty document: the instances keep their project between tests and runs. */
export async function clear(p: Page) {
  await editor(p).click();
  await p.keyboard.press("ControlOrMeta+a");
  await p.keyboard.press("Delete");
  await expect.poll(() => docText(p)).toBe("");
}
