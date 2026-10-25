import { expect, test } from "@playwright/test";
import { clear, docText, editor, rename } from "./helpers";

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
  await clear(alice);
  await clear(bob);

  await alice.getByRole("button", { name: "Partager" }).click();
  await rename(alice, "Alice");
  await alice.getByRole("button", { name: "Inviter" }).click();
  await expect(alice.getByLabel("Invitation", { exact: true })).toHaveValue(/^inkwell1:/);
  const invitation = await alice.getByLabel("Invitation", { exact: true }).inputValue();
  await alice.getByRole("button", { name: "Fermer" }).click();

  await bob.getByRole("button", { name: "Partager" }).click();
  await rename(bob, "Bob");
  await bob.getByPlaceholder("Collez l'invitation ici").fill(invitation);
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
