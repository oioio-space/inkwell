import type { Page } from "@playwright/test";
import { expect, test } from "../fixtures";
import { clear, open, pair, typeAtEnd } from "../helpers";
import { ALICE_URL, BOB_URL } from "../ports";

// Defects of the S7 walkthrough (people), found by looking at the screen.

const panel = (p: Page) => p.getByRole("dialog", { name: "Personnes" });
const rowOf = (p: Page, name: string) =>
  panel(p).getByRole("listitem").filter({ hasText: name }).first();
const openPanel = async (p: Page) => {
  await p.getByRole("button", { name: "Voir les personnes" }).click();
  await expect(panel(p)).toBeVisible();
};

async function twoPeople(browser: Parameters<typeof open>[0]) {
  const alice = await open(browser, ALICE_URL);
  const bob = await open(browser, BOB_URL);
  await clear(alice);
  await clear(bob);
  await pair(alice, bob);
  return { alice, bob };
}

test("UX-S7-1 a person who can only read has a formatting bar that is switched off", async ({
  browser,
}) => {
  test.setTimeout(120_000);
  const { alice, bob } = await twoPeople(browser);
  await openPanel(alice);
  try {
    await alice.getByLabel("Rôle de Bob").selectOption("reader");
    await expect(bob.getByTestId("access-banner")).toBeVisible({ timeout: 20_000 });
    // « Gras » and « Insérer » would do nothing, silently: the whole bar is off, not just inert.
    await expect(bob.locator(".tb")).toHaveAttribute("inert", "", { timeout: 5_000 });
    await expect(bob.locator(".tb")).toHaveCSS("opacity", /^0\.[3-6]/);
    // The writer's bar (Alice) stays usable.
    await expect(alice.locator(".tb")).not.toHaveAttribute("inert", "");
  } finally {
    await alice.getByLabel("Rôle de Bob").selectOption("writer");
    await expect(bob.locator(".tb")).not.toHaveAttribute("inert", "", { timeout: 20_000 });
  }
});

test("UX-S7-2 the admin who disconnected someone reads « par vous », not « par Un admin »", async ({
  browser,
}) => {
  test.setTimeout(120_000);
  const { alice } = await twoPeople(browser);
  await openPanel(alice);
  await rowOf(alice, "Bob").getByRole("button", { name: "Déconnecter" }).click();
  await rowOf(alice, "Bob").getByRole("group").getByRole("button", { name: "Déconnecter" }).click();
  try {
    await expect(rowOf(alice, "Bob")).toContainText("Déconnecté par", { timeout: 20_000 });
    await expect(rowOf(alice, "Bob")).toContainText("par vous");
    await expect(rowOf(alice, "Bob")).not.toContainText("Un admin");
  } finally {
    await rowOf(alice, "Bob").getByRole("button", { name: "Laisser revenir" }).click();
    await expect(rowOf(alice, "Bob")).not.toContainText("Déconnecté par", { timeout: 20_000 });
  }
});

test("UX-S7-3 a person let back in keeps their name in the list and in the messages", async ({
  browser,
}) => {
  test.setTimeout(120_000);
  const { alice, bob } = await twoPeople(browser);
  await typeAtEnd(bob, "Bob ecrit."); // Bob is seen present: Alice knows his name
  await openPanel(alice);
  await expect(rowOf(alice, "Bob")).toContainText("En ligne");
  await rowOf(alice, "Bob").getByRole("button", { name: "Déconnecter" }).click();
  await rowOf(alice, "Bob").getByRole("group").getByRole("button", { name: "Déconnecter" }).click();
  await expect(rowOf(alice, "Bob")).toContainText("Déconnecté par", { timeout: 20_000 });
  await expect(bob.getByTestId("access-banner")).toBeVisible({ timeout: 20_000 });
  // Time passes and the screen moves on: Bob is no longer among the people present.
  await panel(alice).getByRole("button", { name: "Fermer" }).click();
  await typeAtEnd(alice, " Alice continue.");
  await expect(
    alice.getByRole("button", { name: "Voir les personnes" }).getByRole("listitem"),
  ).toHaveCount(1);
  await openPanel(alice);
  await rowOf(alice, "Bob").getByRole("button", { name: "Laisser revenir" }).click();
  // Right away, and until Bob types again: his name, never the name of his machine.
  await expect(alice.getByText("Bob peut de nouveau modifier")).toBeVisible({ timeout: 10_000 });
  await expect(panel(alice).getByRole("listitem").nth(1)).toContainText("Bob");
});

test("UX-S7-4 a person who cannot invite is told so, instead of a technical error", async ({
  browser,
}) => {
  test.setTimeout(120_000);
  const { bob } = await twoPeople(browser); // Bob joined as a writer: only an admin invites
  await bob.getByRole("button", { name: "Partager" }).click();
  const invite = bob.getByRole("button", { name: "Inviter" });
  await expect(invite).toBeDisabled();
  await expect(bob.getByText("Seul un admin peut inviter quelqu'un")).toBeVisible();
  await expect(bob.getByText(/RuntimeError|signer is not an admin/)).toHaveCount(0);
});
