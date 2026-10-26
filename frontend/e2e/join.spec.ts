import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { clear, docText, freshInvitation, open, pair, rename, typeAtEnd } from "./helpers";
import { startInstance, stopInstance } from "./instance";
import { ALICE_URL, BOB_URL, CAROL_PORT } from "./ports";

// N5: joining a project means receiving everything in it, whoever invited and however many people
// are already there.

/** `host` invites and `guest` joins with the invitation text. */
async function joinWith(host: Page, guest: Page, guestName: string) {
  const text = await freshInvitation(host);

  await guest.getByRole("button", { name: "Partager" }).click();
  await rename(guest, guestName);
  await guest.getByPlaceholder("Collez l'invitation ici").fill(text);
  await guest.getByRole("button", { name: "Rejoindre" }).click();
  await expect(guest.getByRole("status")).toHaveText(/Synchronisé/, { timeout: 60_000 });
}

test("JOIN-3 a third person who joins receives everything, and the three converge", async ({
  browser,
}) => {
  test.setTimeout(90_000); // three instances, two joins
  const alice = await open(browser, ALICE_URL);
  const bob = await open(browser, BOB_URL, true);
  await clear(alice);
  await clear(bob);
  await pair(alice, bob);

  const before = "Texte d'Alice avant Carole.";
  await typeAtEnd(alice, before);
  await expect.poll(() => docText(bob)).toBe(before);

  const carolApp = await startInstance(CAROL_PORT, mkdtempSync(join(tmpdir(), "inkwell-e2e-carol-")));
  try {
    const carol = await open(browser, `http://127.0.0.1:${CAROL_PORT}/`);
    await joinWith(alice, carol, "Carole");

    // Joining = receiving everything that was typed before.
    await expect.poll(() => docText(carol), { timeout: 20_000 }).toBe(before);

    // And from then on all three see each other's typing.
    await typeAtEnd(carol, " Carole.");
    await expect.poll(() => docText(alice), { timeout: 20_000 }).toContain("Carole.");
    await typeAtEnd(bob, " Bob.");
    await expect.poll(() => docText(carol), { timeout: 20_000 }).toContain("Bob.");
    await expect.poll(() => docText(alice), { timeout: 20_000 }).toContain("Bob.");
    const text = await docText(alice);
    expect(await docText(bob)).toBe(text);
    expect(await docText(carol)).toBe(text);
    expect(text).toContain(before);
  } finally {
    await stopInstance(carolApp);
  }
});
