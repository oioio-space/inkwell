import type { ChildProcess } from "node:child_process";
import { existsSync, mkdtempSync, readdirSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import type { Browser, Page } from "@playwright/test";
import { encodeEnvelope, encodeHeads, Kind } from "../packages/protocol/src/index";
import { tapBridge } from "./bridgeTap";
import { mainTypIn, readDisk } from "./disk";
import { expect, test } from "./fixtures";
import { clear, docText, editor, joinAs, open, typeAtEnd } from "./helpers";
import { startInstance, stopInstance } from "./instance";
import { CAROL_PORT, DAVE_PORT, SPARE_PORT } from "./ports";

// PC1a (N-10, N-11): a second profile on the first one's folder takes the document over, and
// Inkwell restarts on it. Every wait is on the startup id of the process image that answers
// (`Session.StartupID`, the only proof of a restart: the pid and /health stay the same) or on what
// the page shows, never on a duration. The reading half is W58a's (foreign-project.spec.ts).

const FIRST = "Texte du premier profil.";
const FOREIGN =
  "Ce document vient d'un autre appareil. Vous pouvez le lire et en copier le texte, mais pas le modifier ni le partager.";
const TAKE_OVER = "Reprendre ce document ici";
const DONE = "Ce document est maintenant le vôtre sur cet appareil.";
const url = (port: number) => `http://127.0.0.1:${port}/`;
const banner = (p: Page) => p.getByTestId("access-banner");
const toast = (p: Page, text: string) => p.locator(".toast", { hasText: text });

/** One profile: its own identity folder, on a project folder that may be shared. */
interface Profile {
  home: string;
  docs: string;
}
const profile = (docs = mkdtempSync(join(tmpdir(), "inkwell-e2e-change-docs-"))): Profile => ({
  home: mkdtempSync(join(tmpdir(), "inkwell-e2e-change-home-")),
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

/** The startup id of the image that answers the page; undefined while no application page is up. */
const startupId = (p: Page): Promise<string | undefined> =>
  p
    .evaluate(() =>
      (globalThis as { __inkwellStartupId?: () => Promise<string> }).__inkwellStartupId?.(),
    )
    .catch(() => undefined);

/** What the page holds of its session (`observeSession`). */
const held = (p: Page): Promise<{ signers: string[]; peers: string[] }> =>
  p.evaluate(
    () =>
      (
        globalThis as { __inkwellSession?: () => { signers: string[]; peers: string[] } }
      ).__inkwellSession?.() ?? { signers: [], peers: [] },
  );

/** A second profile opens the first one's folder: readable, not a member. */
async function secondProfileOpens(
  browser: Browser,
  first: Profile,
): Promise<{ app: ChildProcess; page: Page }> {
  const app = await startInstance(CAROL_PORT, first.docs, { home: profile().home });
  try {
    const page = await open(browser, url(CAROL_PORT));
    await expect(banner(page)).toContainText(FOREIGN, { timeout: 30_000 });
    await expect.poll(() => docText(page)).toBe(FIRST);
    return { app, page };
  } catch (err) {
    await stopInstance(app);
    throw err;
  }
}

/**
 * « Reprendre ce document ici », confirmed once; returns the startup ids of the image that was
 * asked and of the one that answers afterwards, once the application is back on it.
 */
async function takeOver(page: Page): Promise<{ before: string; after: string }> {
  const before = await startupId(page);
  expect(before, "the old image has a startup id").toMatch(/\S/);
  await page.getByRole("button", { name: TAKE_OVER }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText("Inkwell va redémarrer");
  await dialog.getByRole("button", { name: TAKE_OVER }).click();
  const confirmed = Date.now();
  let after: string | undefined;
  await expect
    .poll(
      async () => {
        after = await startupId(page);
        return after !== undefined && after !== before;
      },
      { timeout: 60_000, message: "another process image answers the application" },
    )
    .toBe(true);
  // How long the person waits, from the confirmation to the application on the new image.
  test.info().annotations.push({ type: "take-over ms", description: `${Date.now() - confirmed}` });
  return { before: before as string, after: after as string };
}

test("PC1-E2E the take-over restarts Inkwell on the same document, writable", async ({
  browser,
}) => {
  test.setTimeout(120_000);
  const first = profile();
  await firstProfileWrites(browser, first);
  const { app, page } = await secondProfileOpens(browser, first);
  try {
    // Before: it reads and cannot invite (the refusal crosses the real bridge).
    const invite = () =>
      page.evaluate(() =>
        (
          globalThis as {
            __inkwellInvite?: (role: string) => Promise<{ refused?: string; text?: string }>;
          }
        ).__inkwellInvite?.("writer"),
      );
    expect((await invite())?.refused).toBe("notMember");

    const { before, after } = await takeOver(page);
    expect(after).not.toBe(before);

    // The same document, on the same folder, now this device's: no banner, the notice, the focus.
    await expect(toast(page, DONE)).toHaveCount(1, { timeout: 30_000 });
    await expect(banner(page)).toHaveCount(0);
    await expect.poll(() => docText(page)).toBe(FIRST);
    await expect(editor(page)).toBeFocused();
    await expect(page).toHaveURL(url(CAROL_PORT));

    // Writable, and it reaches the file of the same folder.
    await page.keyboard.press("ControlOrMeta+End");
    await page.keyboard.type(" Puis le deuxième.");
    const text = `${FIRST} Puis le deuxième.`;
    await expect.poll(() => docText(page)).toBe(text);
    await expect.poll(() => readDisk(mainTypIn(first.docs)), { timeout: 20_000 }).toContain(text);
    await expect
      .poll(async () => (await invite())?.text ?? "", { timeout: 60_000 })
      .toMatch(/^inkwell1:/);

    // The old member list was archived by copy, once.
    const archive = join(dirname(mainTypIn(first.docs)), ".inkwell", "trust-archive");
    expect(existsSync(archive) ? readdirSync(archive) : []).toHaveLength(1);

    // The notice shows once: the same image answers a reloaded page, and says nothing more.
    await page.reload();
    await expect(page.locator('.cm-content:not([aria-readonly="true"])')).toBeVisible();
    await expect.poll(() => docText(page)).toBe(text);
    expect(await startupId(page)).toBe(after);
    await expect(toast(page, DONE)).toHaveCount(0);
  } finally {
    await stopInstance(app);
  }
});

test("PC1-E2E the banner's actions stay whole in a narrow window and keep a shape in forced colours", async ({
  browser,
}) => {
  test.setTimeout(120_000);
  const first = profile();
  await firstProfileWrites(browser, first);
  const { app, page } = await secondProfileOpens(browser, first);
  try {
    const strip = page.locator(".banner", { has: banner(page) });
    const actions = [
      page.getByRole("button", { name: "Demander une invitation" }),
      page.getByRole("button", { name: TAKE_OVER }),
    ];
    // UX-PC1a-5: in a narrow window each action stays inside the banner, with its whole label.
    for (const width of [480, 360]) {
      await page.setViewportSize({ width, height: 640 });
      const box = await strip.boundingBox();
      if (!box) throw new Error("no banner");
      for (const action of actions) {
        const b = await action.boundingBox();
        if (!b) throw new Error("no action");
        const label = (await action.textContent())?.trim();
        expect(b.x, `${width} px: « ${label} » starts inside the banner`).toBeGreaterThanOrEqual(
          box.x - 0.5,
        );
        expect(
          b.x + b.width,
          `${width} px: « ${label} » ends inside the banner`,
        ).toBeLessThanOrEqual(Math.min(box.x + box.width, width) + 0.5);
        expect(
          await action.evaluate((el) => el.scrollWidth <= el.clientWidth),
          `${width} px: the label of « ${label} » is not cut`,
        ).toBe(true);
      }
    }
    await page.setViewportSize({ width: 1100, height: 640 });

    // UX-PC1a-4: where the system forces its colours, backgrounds go: a button keeps its shape by
    // a border, the dialog its edge, and the suggested action stays told apart from « Annuler ».
    await page.emulateMedia({ forcedColors: "active" });
    const edge = (el: Element) => {
      const s = getComputedStyle(el);
      return s.borderTopStyle === "solid" ? Number.parseFloat(s.borderTopWidth) : 0;
    };
    for (const action of actions) {
      expect(await action.evaluate(edge), "a banner action has a border").toBeGreaterThanOrEqual(1);
    }
    await actions[1]?.click();
    const dialog = page.getByRole("dialog");
    expect(await dialog.evaluate(edge), "the dialog has an edge").toBeGreaterThanOrEqual(1);
    const cancel = dialog.getByRole("button", { name: "Annuler" });
    const confirm = dialog.getByRole("button", { name: TAKE_OVER });
    for (const b of [cancel, confirm]) {
      expect(await b.evaluate(edge), "a dialog button has a border").toBeGreaterThanOrEqual(1);
    }
    const fill = (el: Element) => getComputedStyle(el).backgroundColor;
    expect(await confirm.evaluate(fill), "the suggested action differs from « Annuler »").not.toBe(
      await cancel.evaluate(fill),
    );
    await cancel.click();
    await expect(dialog).toHaveCount(0);
    await expect(banner(page)).toContainText(FOREIGN); // nothing was asked
  } finally {
    await stopInstance(app);
  }
});

test("PC1-E2E nothing of the old session survives the restart", async ({ browser }) => {
  test.setTimeout(120_000);
  // What the test holds of the old project: an attestation by a writer of the old list, and a
  // frame from that writer. Go's bridge is the only source of both for a page.
  const OLD = "12D3KooWOldListWriterHeldBackByTheTest";
  const SENTINEL = "12D3KooWSentinelAfterTheHeldFrames";
  const attestedBy = (peer: string) =>
    Buffer.from(
      encodeEnvelope({
        kind: Kind.Attested,
        peer,
        payload: encodeHeads({ doc: "documentOfTheOldProject", heads: ["ab".repeat(32)] }),
      }),
    );
  const frames = (peer: string) =>
    [
      encodeEnvelope({ kind: Kind.PeerUp, peer, payload: new Uint8Array() }),
      encodeEnvelope({ kind: Kind.Message, peer, payload: Uint8Array.of(0xa0) }),
    ].map((f) => Buffer.from(f));

  const first = profile();
  await firstProfileWrites(browser, first);
  const { app, page } = await secondProfileOpens(browser, first);
  try {
    // The test sits on the page's network stream, to hand it frames as Go's bridge would.
    const bridge = await tapBridge(page, "net");
    await page.reload();
    await expect(banner(page)).toContainText(FOREIGN, { timeout: 30_000 });

    // The old session holds a marker of its own and the old project's attestation.
    await page.evaluate(() => {
      (globalThis as { __pc1OldSession?: string }).__pc1OldSession = "old";
    });
    await expect
      .poll(async () => {
        bridge.send(attestedBy(OLD));
        return (await held(page)).signers;
      })
      .toContain(OLD);

    const { before, after } = await takeOver(page);
    expect(after, "N-11 another process image").not.toBe(before);
    await expect(banner(page)).toHaveCount(0, { timeout: 30_000 });

    // The page is another one: no marker, no attestation of the old project.
    expect(
      await page.evaluate(() => (globalThis as { __pc1OldSession?: string }).__pc1OldSession),
      "N-11 the old page's context is gone",
    ).toBeUndefined();
    expect((await held(page)).signers, "N-11 no attestation of the old project").not.toContain(OLD);

    // The frames held back are handed to the new page: the old writer is not of the new list, so
    // they are refused. The sentinel sent after them proves they were read.
    await expect
      .poll(async () => {
        for (const f of frames(OLD)) bridge.send(f);
        bridge.send(attestedBy(SENTINEL));
        return (await held(page)).signers;
      })
      .toContain(SENTINEL);
    expect((await held(page)).peers, "N-11 a frame of the old project is refused").not.toContain(
      OLD,
    );
    await expect.poll(() => docText(page)).toBe(FIRST);
  } finally {
    await stopInstance(app);
  }
});

test("PC1-E2E a third profile joins after the take-over and receives the earlier text", async ({
  browser,
}) => {
  test.setTimeout(180_000);
  const first = profile();
  await firstProfileWrites(browser, first);
  const { app, page } = await secondProfileOpens(browser, first);
  try {
    await takeOver(page);
    await expect(banner(page)).toHaveCount(0, { timeout: 30_000 });

    const third = await startInstance(DAVE_PORT, profile().docs);
    try {
      const guest = await open(browser, url(DAVE_PORT));
      await joinAs(page, guest, "Troisième", "Deuxième");
      await expect.poll(() => docText(guest), { timeout: 60_000 }).toBe(FIRST);
    } finally {
      await stopInstance(third);
    }
  } finally {
    await stopInstance(app);
  }
});

test("PC1-E2E the first profile then opens the folder as a readable non-member", async ({
  browser,
}) => {
  test.setTimeout(120_000);
  const first = profile();
  await firstProfileWrites(browser, first);
  const second = await secondProfileOpens(browser, first);
  try {
    await takeOver(second.page);
    await expect(banner(second.page)).toHaveCount(0, { timeout: 30_000 });
    await second.page.context().close();
  } finally {
    await stopInstance(second.app);
  }

  // The first profile comes back to its folder: the list no longer names it.
  const app = await startInstance(SPARE_PORT, first.docs, { home: first.home });
  try {
    const page = await open(browser, url(SPARE_PORT));
    await expect(banner(page)).toContainText(FOREIGN, { timeout: 30_000 });
    await expect(page.getByRole("button", { name: "Demander une invitation" })).toBeVisible();
    await expect(page.getByRole("button", { name: TAKE_OVER })).toBeVisible();
    await expect.poll(() => docText(page)).toBe(FIRST);
    // Typing changes nothing: the text is still what the next read of the state gives.
    await editor(page).click();
    await page.keyboard.press("ControlOrMeta+End");
    await page.keyboard.type("zzz");
    await expect(editor(page)).toHaveAttribute("aria-readonly", "true");
    expect(await docText(page)).toBe(FIRST);
  } finally {
    await stopInstance(app);
  }
});
