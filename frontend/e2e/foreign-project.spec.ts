import { cpSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { Browser, Page } from "@playwright/test";
import { mainTypIn, readDisk } from "./disk";
import { expect, test } from "./fixtures";
import { clear, docText, editor, freshInvitation, open, rename, typeAtEnd } from "./helpers";
import { startInstance, stopInstance } from "./instance";
import { CAROL_PORT, SPARE_PORT } from "./ports";

// W58a (N-10): a project folder whose member list does not name this device. A second profile on
// the first one's folder reads, sees one sentence and its two actions, and cannot invite; it asks
// for an invitation, joins with the first profile's and can then write. The other way out, the
// take-over by a restart, is PC1a's: project-change.spec.ts.

const FIRST = "Texte du premier profil.";
const url = (port: number) => `http://127.0.0.1:${port}/`;
const banner = (p: Page) => p.getByTestId("access-banner");

/** One profile: its own identity folder, on a project folder that may be shared. */
interface Profile {
  home: string;
  docs: string;
}
const profile = (docs = mkdtempSync(join(tmpdir(), "inkwell-e2e-foreign-docs-"))): Profile => ({
  home: mkdtempSync(join(tmpdir(), "inkwell-e2e-foreign-home-")),
  docs,
});

/** The first profile writes `FIRST` into the project and quits, once it is on the disk. */
async function firstProfileWrites(browser: Browser, first: Profile): Promise<void> {
  const app = await startInstance(SPARE_PORT, first.docs, { home: first.home });
  try {
    const page = await open(browser, url(SPARE_PORT));
    await clear(page);
    await typeAtEnd(page, FIRST);
    await expect.poll(() => readDisk(mainTypIn(first.docs)), { timeout: 20_000 }).toContain(FIRST);
    await page.context().close();
  } finally {
    await stopInstance(app);
  }
}

/** Typing changes nothing for a device that cannot edit. */
async function cannotType(p: Page): Promise<void> {
  const before = await docText(p);
  await editor(p).click();
  await p.keyboard.press("ControlOrMeta+End");
  await p.keyboard.type("zzz");
  await p.waitForTimeout(300);
  expect(await docText(p)).toBe(before);
}

/** The banner of a device the member list does not name: its sentence and its two actions. */
async function expectForeign(p: Page): Promise<void> {
  await expect(banner(p)).toContainText(
    "Ce document vient d'un autre appareil. Vous pouvez le lire et en copier le texte, mais pas le modifier ni le partager.",
    { timeout: 30_000 },
  );
  await expect(p.getByRole("button", { name: "Demander une invitation" })).toBeVisible();
  await expect(p.getByRole("button", { name: "Reprendre ce document ici" })).toBeVisible();
  await expect.poll(() => docText(p)).toBe(FIRST);
  await cannotType(p);
}

test("W58a-E2E a second profile reads and cannot invite", async ({ browser }) => {
  test.setTimeout(120_000);
  const first = profile();
  await firstProfileWrites(browser, first);

  const second = await startInstance(CAROL_PORT, first.docs, { home: profile().home });
  try {
    const page = await open(browser, url(CAROL_PORT));
    await expectForeign(page);

    // The refusal crosses the real bridge as a result with a stable key, not as an error.
    const made = await page.evaluate(() =>
      (
        globalThis as typeof globalThis & {
          __inkwellInvite?: (role: string) => Promise<{ refused?: string; text?: string }>;
        }
      ).__inkwellInvite?.("writer"),
    );
    expect(made?.refused).toBe("notMember");
    expect(made?.text ?? "").toBe("");

    // Inviting is not offered, and the dialog says why in its own words.
    await page.getByRole("button", { name: "Partager" }).click();
    await expect(page.getByRole("button", { name: "Inviter" })).toBeDisabled();
    await expect(page.getByRole("dialog")).toContainText(
      "Ce document n'est pas à vous sur cet appareil : vous ne pouvez pas inviter.",
    );
    await page.getByRole("button", { name: "Fermer" }).click();

    // « Demander une invitation » leads to the join part of the same dialog.
    await page.getByRole("button", { name: "Demander une invitation" }).click();
    await expect(page.getByRole("dialog")).toContainText(
      "Demandez à la personne qui partage ce document de vous inviter",
    );
    await expect(page.getByPlaceholder("Collez l'invitation ici")).toBeVisible();
  } finally {
    await stopInstance(second);
  }
});

test("W58a-E2E a non-member asks for an invitation, joins and can write", async ({ browser }) => {
  test.setTimeout(180_000);
  const first = profile();
  await firstProfileWrites(browser, first);
  // The second profile has the first one's folder as a synced copy: two instances never run on
  // the same folder.
  const copy = mkdtempSync(join(tmpdir(), "inkwell-e2e-foreign-copy-"));
  cpSync(first.docs, copy, { recursive: true });

  const firstApp = await startInstance(SPARE_PORT, first.docs, { home: first.home });
  try {
    const host = await open(browser, url(SPARE_PORT));
    const secondApp = await startInstance(CAROL_PORT, copy, { home: profile().home });
    try {
      const page = await open(browser, url(CAROL_PORT));
      await expectForeign(page);

      const invitation = await freshInvitation(host, "Premier");
      await page.getByRole("button", { name: "Demander une invitation" }).click();
      await rename(page, "Deuxième");
      await page.getByPlaceholder("Collez l'invitation ici").fill(invitation);
      await page.getByRole("button", { name: "Rejoindre" }).click();

      // Once in, the banner is gone and the text takes keys.
      await expect(banner(page)).toHaveCount(0, { timeout: 60_000 });
      await expect(page.getByRole("status").first()).toHaveText(/Synchronisé/, { timeout: 60_000 });
      await typeAtEnd(page, " Puis le deuxième.");
      const text = `${FIRST} Puis le deuxième.`;
      await expect.poll(() => docText(page)).toBe(text);
      await expect.poll(() => docText(host), { timeout: 30_000 }).toBe(text);
    } finally {
      await stopInstance(secondApp);
    }
  } finally {
    await stopInstance(firstApp);
  }
});
