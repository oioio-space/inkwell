import { expect, type Page, test } from "@playwright/test";

const editor = (p: Page) => p.locator(".cm-content");

/** The document text as typed, without remote-cursor name labels. */
const docText = (p: Page) =>
  editor(p).evaluate((el) => {
    const copy = el.cloneNode(true) as HTMLElement;
    for (const label of copy.querySelectorAll(".cm-remote-caret")) label.remove();
    // CodeMirror renders trailing spaces as NBSP and may insert zero-width breaks.
    return copy.innerText.replace(/\u00a0/g, " ").replace(/[\u200b\u2060\n]/g, "");
  });

async function rename(p: Page, name: string) {
  const field = p.getByLabel(/Votre nom/);
  await field.fill(name);
  await field.blur();
}

test("two people type in the same file, see each other, and undo only their own text", async ({
  browser,
}) => {
  const alice = await (
    await browser.newContext({ locale: "fr-FR", viewport: { width: 1100, height: 640 } })
  ).newPage();
  const bob = await (
    await browser.newContext({
      locale: "fr-FR",
      viewport: { width: 1100, height: 640 },
      colorScheme: "dark",
    })
  ).newPage();
  await alice.goto("http://127.0.0.1:18081/");
  await bob.goto("http://127.0.0.1:18082/");
  await expect(editor(alice)).toBeVisible();
  await expect(editor(bob)).toBeVisible();

  await alice.getByRole("button", { name: "Partager" }).click();
  await rename(alice, "Alice");
  await alice.getByRole("button", { name: "Inviter" }).click();
  const address = (await alice.locator("dialog code").first().textContent())?.trim() ?? "";
  expect(address).toMatch(/:\d+$/);
  await alice.getByRole("button", { name: "Fermer" }).click();

  await bob.getByRole("button", { name: "Partager" }).click();
  await rename(bob, "Bob");
  await bob.getByPlaceholder("192.168.1.20:7419").fill(address);
  await bob.getByRole("button", { name: "Rejoindre" }).click();

  await expect(alice.getByRole("status")).toHaveText(/Synchronisé/);
  await expect(bob.getByRole("status")).toHaveText(/Synchronisé/);

  await editor(alice).click();
  await alice.keyboard.type("Bonjour ");
  await expect(editor(bob)).toContainText("Bonjour");

  await editor(bob).click();
  await bob.keyboard.press("End");
  await bob.keyboard.type("tout le monde");
  await expect(editor(alice)).toContainText("Bonjour tout le monde");

  // Each sees the other's named caret.
  await expect(alice.locator(".cm-remote-name", { hasText: "Bob" })).toBeVisible();
  await expect(bob.locator(".cm-remote-name", { hasText: "Alice" })).toBeVisible();

  // Visual record of the collaborative state (kept as test artifacts).
  await alice.screenshot({ path: "test-results/alice.png" });
  await bob.screenshot({ path: "test-results/bob.png" });

  // Bob's undo removes only Bob's words, everywhere.
  await bob.keyboard.press("ControlOrMeta+z");
  await expect.poll(() => docText(alice)).toBe("Bonjour ");
  await expect.poll(() => docText(bob)).toBe("Bonjour ");
});
