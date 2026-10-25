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
 * Alice invites, Bob joins with the invitation text; both end up "Synchronisé".
 * Inviting is refused (the UI shows why) until the node has an address to put in the
 * ticket (through a relay, after its reservation), so asking is retried.
 */
export async function pair(alice: Page, bob: Page) {
  await alice.getByRole("button", { name: "Partager" }).click();
  await rename(alice, "Alice");
  const invitation = alice.getByLabel("Invitation", { exact: true });
  await expect(async () => {
    await alice.getByRole("button", { name: "Inviter" }).click();
    await expect(invitation).toHaveValue(/^inkwell1:/, { timeout: 1_000 });
  }).toPass({ timeout: 60_000 });
  const text = await invitation.inputValue();
  await alice.getByRole("button", { name: "Fermer" }).click();

  await bob.getByRole("button", { name: "Partager" }).click();
  await rename(bob, "Bob");
  await bob.getByPlaceholder("Collez l'invitation ici").fill(text);
  await bob.getByRole("button", { name: "Rejoindre" }).click();

  await expect(alice.getByRole("status")).toHaveText(/Synchronisé/, { timeout: 60_000 });
  await expect(bob.getByRole("status")).toHaveText(/Synchronisé/, { timeout: 60_000 });
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
