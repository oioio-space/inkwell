import type { ChildProcess } from "node:child_process";
import { mkdtempSync, readdirSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { Browser, Page } from "@playwright/test";
import { expect, test } from "../fixtures";
import { clear, docText, freshInvitation, joinAs, open, rename, typeAtEnd } from "../helpers";
import { startInstance, stopInstance } from "../instance";
import { ALICE_URL, CAROL_PORT } from "../ports";

// Defects of the PC1b2 walkthrough (joining by invitation restarts into the joined document),
// found by looking at the screen: gates/PC1b2/UX.md.

/**
 * Bob, the walkthrough's guest: a person with a document of his own, on a fresh profile. The
 * shared instance on BOB_URL keeps its state from one spec to the next and is often in Alice's
 * document already, where a join asks nothing (J-5) and there is nothing to go back to.
 */
async function guest(browser: Browser): Promise<{ bob: Page; app: ChildProcess }> {
  const docs = mkdtempSync(join(tmpdir(), "inkwell-e2e-pc1b2-guest-"));
  expect(readdirSync(docs), `fresh: ${docs}`).toEqual([]);
  const app = await startInstance(CAROL_PORT, docs);
  return { bob: await open(browser, `http://127.0.0.1:${CAROL_PORT}/`), app };
}

test("UX-PC1b2-1 after the guest cancels, the inviter is not offered the spent invitation again", async ({
  browser,
}) => {
  test.setTimeout(120_000);
  const alice = await open(browser, ALICE_URL);
  const { bob, app } = await guest(browser);
  try {
    await clear(alice);
    const spent = await freshInvitation(alice, "Alice");
    await bob.getByRole("button", { name: "Partager" }).click();
    await rename(bob, "Bob");
    await bob.getByPlaceholder("Collez l'invitation ici").fill(spent);
    await bob.getByRole("button", { name: "Rejoindre" }).click();
    const asked = bob.getByTestId("join-confirm");
    await expect(asked).toBeVisible({ timeout: 60_000 });
    await asked.getByRole("button", { name: "Annuler" }).click();
    await expect(bob.getByText("Invitation annulée.")).toBeVisible();
    // « Invitation annulée. Pour rejoindre ce document plus tard, demandez une nouvelle invitation. »
    // Alice is asked for another one: her dialog must not hand her the spent one to copy.
    await alice.getByRole("button", { name: "Partager" }).click();
    const field = alice.getByLabel("Invitation", { exact: true });
    const offered = (await field.count()) > 0 ? await field.inputValue() : "";
    expect(
      offered,
      "the invitation the guest cancelled is used up: it is not offered again",
    ).not.toBe(spent);
    await expect(alice.getByRole("button", { name: "Copier l'invitation" })).toHaveCount(0);
  } finally {
    await stopInstance(app);
  }
});

test("UX-PC1b2-2 after joining, the person can reach the document they had before from the application", async ({
  browser,
}) => {
  test.setTimeout(180_000);
  const alice = await open(browser, ALICE_URL);
  const { bob, app } = await guest(browser);
  try {
    await clear(alice);
    await typeAtEnd(bob, "Notes de Bob avant l'invitation.");
    await joinAs(alice, bob, "Bob", "Alice");
    // The question promised « il reste dans « … » »: the application itself must lead back to it,
    // in the main menu or the document list, not only the file manager.
    // « Revenir à mon document » is the wording the waiting page already uses (PC1b, J-9).
    await bob.getByRole("button", { name: "Menu principal" }).click();
    const back = bob.getByRole("menuitem", {
      name: /Ouvrir|Mes documents|Document précédent|Autre document|Revenir à mon document/,
    });
    await expect(back, "a way from the joined document to the previous one").toBeVisible();
    // And it leads there: Inkwell restarts on the text Bob had before the invitation.
    await back.click();
    await expect
      .poll(() => docText(bob).catch(() => ""), { timeout: 60_000 })
      .toContain("Notes de Bob avant l'invitation.");
  } finally {
    await stopInstance(app);
  }
});
