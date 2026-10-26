import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { Locator, Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { clear, docText, editor, freshInvitation, open, rename, typeAtEnd } from "./helpers";
import { startInstance, stopInstance } from "./instance";
import { ALICE_URL, BOB_URL, CAROL_PORT } from "./ports";

// S7 — people: Alice changes a role, disconnects Paul and lets him back; Léa is a reader.
// ACL-01, -03, -04, -08 and the panel of B3b (docs/plans/2026-10-25-t11-interface.md).

/** `host` invites and `guest` joins under `guestName`. */
async function joinAs(host: Page, guest: Page, guestName: string, hostName?: string) {
  const text = await freshInvitation(host, hostName);
  await guest.getByRole("button", { name: "Partager" }).click();
  await rename(guest, guestName);
  await guest.getByPlaceholder("Collez l'invitation ici").fill(text);
  await guest.getByRole("button", { name: "Rejoindre" }).click();
  await expect(guest.getByRole("status").first()).toHaveText(/Synchronisé/, { timeout: 60_000 });
  await guest.keyboard.press("Escape");
}

const panel = (p: Page) => p.getByRole("dialog", { name: "Personnes" });
const rowOf = (p: Page, name: string): Locator =>
  panel(p).getByRole("listitem").filter({ hasText: name }).first();
const openPanel = async (p: Page) => {
  await p.getByRole("button", { name: "Voir les personnes" }).click();
  await expect(panel(p)).toBeVisible();
};
const closePanel = (p: Page) => panel(p).getByRole("button", { name: "Fermer" }).click();
const banner = (p: Page) => p.getByTestId("access-banner");

/** Typing in the editor changes nothing for a person who cannot edit. */
async function cannotType(p: Page) {
  const before = await docText(p);
  await editor(p).click();
  await p.keyboard.press("ControlOrMeta+End");
  await p.keyboard.type("zzz");
  await p.waitForTimeout(300);
  expect(await docText(p)).toBe(before);
}

test("S7 people: a role, disconnect Paul, let him back, and Léa as a reader", async ({
  browser,
}) => {
  test.setTimeout(150_000);
  const alice = await open(browser, ALICE_URL);
  const paul = await open(browser, BOB_URL, true);
  await clear(alice);
  await clear(paul);
  await joinAs(alice, paul, "Paul", "Alice");
  await expect(alice.getByRole("status").first()).toHaveText(/Synchronisé/, { timeout: 60_000 });

  const leaApp = await startInstance(CAROL_PORT, mkdtempSync(join(tmpdir(), "inkwell-e2e-lea-")));
  try {
    const lea = await open(browser, `http://127.0.0.1:${CAROL_PORT}/`);
    await joinAs(alice, lea, "Léa");

    // Alice: the panel lists everyone, with a role and who is online.
    await openPanel(alice);
    await expect(rowOf(alice, "Paul")).toContainText("En ligne");
    await expect(rowOf(alice, "Léa")).toContainText("En ligne");
    await expect(rowOf(alice, "(vous)")).toBeVisible();
    await expect(alice.getByLabel("Rôle de Paul")).toHaveValue("writer");
    await alice.mouse.move(5, 5);
    // One row, not the list: earlier specs add people to Alice's project.
    await expect(rowOf(alice, "Paul")).toHaveScreenshot("people-admin-light.png", {
      maxDiffPixelRatio: 0.02,
      mask: [rowOf(alice, "Paul").locator(".avatar")], // colours come from random identities
    });

    // ACL-08: Léa becomes a reader. She reads, she cannot type, and she is told why.
    await alice.getByLabel("Rôle de Léa").selectOption("reader");
    await expect(alice.getByLabel("Rôle de Léa")).toHaveValue("reader");
    await closePanel(alice);
    await expect(banner(lea)).toContainText("Vous lisez ce document sans pouvoir le modifier", {
      timeout: 20_000,
    });
    await cannotType(lea);

    // Léa (reader) and Paul (writer): the panel shows the people, no control at all.
    for (const person of [lea, paul]) {
      await openPanel(person);
      await expect(rowOf(person, "Alice")).toContainText("Admin");
      await expect(panel(person).locator("select")).toHaveCount(0);
      await expect(panel(person).getByRole("button")).toHaveCount(1); // « Fermer »
      if (person === paul) {
        await person.mouse.move(5, 5);
        // Léa seen from Paul is only a machine name and a colour: not part of the reference.
        // His own row only: other specs add people to the list.
        await expect(rowOf(person, "(vous)")).toHaveScreenshot("people-writer-dark.png", {
          mask: [rowOf(person, "(vous)").locator(".avatar")],
          maxDiffPixelRatio: 0.02,
        });
      }
      await closePanel(person);
    }

    // Paul writes; everyone sees it.
    await typeAtEnd(paul, "Avant. ");
    await expect.poll(() => docText(alice), { timeout: 20_000 }).toContain("Avant.");

    // ACL-01: Alice disconnects Paul, after a question that says what will happen.
    await openPanel(alice);
    await rowOf(alice, "Paul").getByRole("button", { name: "Déconnecter" }).click();
    await expect(rowOf(alice, "Paul")).toContainText("Vous pourrez le laisser revenir");
    await expect(banner(paul)).toHaveCount(0); // nothing happened yet
    await rowOf(alice, "Paul")
      .getByRole("group")
      .getByRole("button", { name: "Déconnecter" })
      .click();
    await expect(rowOf(alice, "Paul")).toContainText("Déconnecté par", { timeout: 20_000 });
    await expect(
      rowOf(alice, "Paul").getByRole("button", { name: "Laisser revenir" }),
    ).toBeVisible();

    // Paul's side: calm, clear, read-only, and how to come back.
    await expect(banner(paul)).toContainText("vous a déconnecté", { timeout: 20_000 });
    await expect(banner(paul)).toContainText("laisser revenir");
    // The header says it too, instead of "Seul".
    await expect(paul.locator("header .status")).toHaveText(/Déconnecté/);
    // The clock time of the disconnection is the one thing that changes from run to run.
    await banner(paul).evaluate((el) => {
      el.textContent = (el.textContent ?? "").replace(/\d{2}:\d{2}/, "12:00");
    });
    await expect(banner(paul).locator("..")).toHaveScreenshot("people-suspended-dark.png", {
      maxDiffPixelRatio: 0.02,
    });
    await cannotType(paul);
    await openPanel(paul);
    await expect(panel(paul).locator("select")).toHaveCount(0);
    await closePanel(paul);
    // Disconnected is not removed: Léa is still a reader and Paul's role is kept.
    await expect(rowOf(alice, "Paul")).toContainText("Écriture");

    // ACL-03: Alice lets him back. He types again, and Alice gets it.
    await rowOf(alice, "Paul").getByRole("button", { name: "Laisser revenir" }).click();
    await expect(rowOf(alice, "Paul")).not.toContainText("Déconnecté par", { timeout: 20_000 });
    await closePanel(alice);
    await expect(banner(paul)).toHaveCount(0, { timeout: 20_000 });
    await typeAtEnd(paul, "Après.");
    await expect.poll(() => docText(alice), { timeout: 30_000 }).toContain("Après.");
    await expect.poll(() => docText(lea), { timeout: 30_000 }).toContain("Après.");
    await expect(banner(lea)).toBeVisible();
  } finally {
    await stopInstance(leaApp);
  }
});
