import { type Browser, expect, type Page } from "@playwright/test";

export const editor = (p: Page) => p.locator(".cm-content");

/**
 * The document text as typed, read from the editor's state through the `__inkwellDocText` hook of
 * instrumented builds (VITE_E2E_OBSERVE=1, what `mise run e2e:build` makes), not from the page:
 * CodeMirror does not render the parts of lines beyond 20 000 characters, so the rendered text of a
 * long paste is incomplete. Line breaks are dropped, as the callers compare lines joined.
 */
export const docText = (p: Page) =>
  p.evaluate(() => {
    const f = (globalThis as { __inkwellDocText?: () => string | undefined }).__inkwellDocText;
    const text = typeof f === "function" ? f() : undefined;
    if (text === undefined)
      throw new Error("docText: no __inkwellDocText (build with VITE_E2E_OBSERVE=1)");
    return text.replace(/[\u200b\u2060\n]/g, "");
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
  // A page resource that fails to load (ERR_NETWORK_CHANGED, reset, ...) breaks the page: fail at once,
  // naming it, instead of letting a later wait run to its timeout. Only the page's own resources: the
  // app may abort its own fetches on purpose.
  const origin = new URL(url).origin;
  const pageResources = new Set(["document", "stylesheet", "script", "font"]);
  const broken = new Promise<never>((_, reject) => {
    page.on("requestfailed", (req) => {
      if (new URL(req.url()).origin !== origin || !pageResources.has(req.resourceType())) return;
      reject(
        new Error(
          `open ${url}: request ${req.method()} ${req.url()} failed: ${req.failure()?.errorText}`,
        ),
      );
    });
  });
  broken.catch(() => {}); // a failure after the load is not an unhandled rejection
  await Promise.race([
    (async () => {
      await page.goto(url);
      await expect(editor(page)).toBeVisible();
    })(),
    broken,
  ]);
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

/**
 * `host` invites and `guest` joins under `guestName` with the invitation text; the guest ends
 * up "Synchronisé". The dialog is closed afterwards.
 */
export async function joinAs(host: Page, guest: Page, guestName: string, hostName?: string) {
  const text = await freshInvitation(host, hostName);
  await guest.getByRole("button", { name: "Partager" }).click();
  await rename(guest, guestName);
  await guest.getByPlaceholder("Collez l'invitation ici").fill(text);
  await guest.getByRole("button", { name: "Rejoindre" }).click();
  await expect(guest.getByRole("status").first()).toHaveText(/Synchronisé/, { timeout: 60_000 });
  await guest.keyboard.press("Escape");
}

/**
 * Alice invites, Bob joins with the invitation text; both end up "Synchronisé" and every given
 * page counts `people` people (2 for a pair; 3 when a third person has joined `alice`).
 */
export async function pair(alice: Page, bob: Page, people = 2) {
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
    await expect(bar).toContainText(`${people} personnes`);
    await expect(bar).not.toContainText("Seul");
    await expect(page.getByRole("status")).not.toHaveText(/Seul/);
  }
}

/** Pastes `text` at the end of the document as one edit (one change, one clipboard insertion). */
export async function pasteAtEnd(p: Page, text: string) {
  await editor(p).click();
  await p.keyboard.press("ControlOrMeta+End");
  await p.keyboard.insertText(text);
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
