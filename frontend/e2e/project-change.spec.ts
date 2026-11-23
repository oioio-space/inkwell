import type { ChildProcess } from "node:child_process";
import { existsSync, mkdtempSync, readdirSync, statSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import type { Browser, Page } from "@playwright/test";
import { encodeEnvelope, encodeHeads, Kind } from "../packages/protocol/src/index";
import { tapBridge } from "./bridgeTap";
import { holds, mainTypIn, readDisk } from "./disk";
import { expect, test } from "./fixtures";
import {
  clear,
  docText,
  editor,
  freshInvitation,
  joinAs,
  open,
  rename,
  typeAtEnd,
} from "./helpers";
import { startInstance, stopInstance } from "./instance";
import { signalOwned } from "./own";
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

// PC1b (J-1..J-3, J-6): an accepted invitation into another document restarts Inkwell on that
// document, in a folder of its own beside the one that was open. The inviter and the person who
// joins are two fresh profiles, each on its own temporary folders.

const HOST_TEXT = "Le texte de la personne qui invite.";
const RESTART_NOTE = "Inkwell va redémarrer pour ouvrir ce document.";
const JOINED = "Vous avez rejoint ce document.";
const WAITING = /^En attente de .+ pour ouvrir le document…$/;
const writable = (p: Page) => p.locator('.cm-content:not([aria-readonly="true"])');
const waitingPage = (p: Page) => p.getByTestId("join-waiting");
/** The sentence that says who is waited for, on the waiting page. */
const waitingSentence = (p: Page) => waitingPage(p).getByRole("status");
const KEPT = "Votre document actuel est conservé.";
const WAITING_NEXT =
  "Le document s'ouvrira dès que cette personne sera connectée. Vous pouvez fermer Inkwell : l'attente reprendra à la prochaine ouverture.";

/** The project folders an instance has in `docs`, by name. */
const foldersIn = (docs: string): string[] =>
  readdirSync(docs).filter((name) => statSync(join(docs, name)).isDirectory());

/** A profile that must be a fresh temporary one: never a folder of the person running the tests. */
function fresh(): Profile {
  const made = profile();
  for (const dir of [made.home, made.docs]) {
    expect(dir.startsWith(tmpdir()) && dir.length > tmpdir().length + 1, `temporary: ${dir}`).toBe(
      true,
    );
    expect(readdirSync(dir), `fresh: ${dir}`).toEqual([]);
  }
  return made;
}

/** The inviter: a fresh profile whose document holds `HOST_TEXT`. */
async function inviter(browser: Browser): Promise<{ app: ChildProcess; page: Page }> {
  const made = fresh();
  const app = await startInstance(SPARE_PORT, made.docs, { home: made.home });
  try {
    const page = await open(browser, url(SPARE_PORT));
    await clear(page);
    await typeAtEnd(page, HOST_TEXT);
    await expect
      .poll(() => readDisk(mainTypIn(made.docs)), { timeout: 20_000 })
      .toContain(HOST_TEXT);
    return { app, page };
  } catch (err) {
    await stopInstance(app);
    throw err;
  }
}

/**
 * `guest` uses an invitation of `host`: « Rejoindre », then « Redémarrer et ouvrir » in the
 * question that names the folder (J-2). Returns when another process image answers the guest's
 * page, with both startup ids, the time of the confirmation and the folder the question named;
 * `onLeft` runs as soon as the guest's page has left for the restart page (Go then holds the
 * verified invitation).
 */
async function acceptInvitation(
  host: Page,
  guest: Page,
  onLeft: () => void = () => {},
): Promise<{ before: string; after: string; clicked: number; folder: string }> {
  const invitation = await freshInvitation(host, "Alice");
  await guest.getByRole("button", { name: "Partager" }).click();
  await rename(guest, "Bob");
  const dialog = guest.getByRole("dialog");
  await expect(dialog, "J-2 said before « Rejoindre »").toContainText(RESTART_NOTE);
  await expect(dialog, "J-2 the current document is kept, said before").toContainText(KEPT);
  await guest.getByPlaceholder("Collez l'invitation ici").fill(invitation);
  const before = await startupId(guest);
  expect(before, "the old image has a startup id").toMatch(/\S/);
  const left = guest.waitForURL(/restart\.html/, { timeout: 60_000 }).then(onLeft);
  await guest.getByRole("button", { name: "Rejoindre" }).click();
  // The question stays until the person chooses: it names the folder, and nothing restarts yet.
  const asked = guest.getByTestId("join-confirm");
  await expect(asked).toContainText(
    /Inkwell va redémarrer pour ouvrir ce document dans un nouveau dossier, « .+ », à côté du vôtre\./,
    { timeout: 60_000 },
  );
  const folder = /dossier, « (.+) », à côté/.exec((await asked.textContent()) ?? "")?.[1] ?? "";
  expect(await startupId(guest), "J-2 nothing restarts before the confirmation").toBe(before);
  expect(guest.url()).not.toContain("restart.html");
  await asked.getByRole("button", { name: "Redémarrer et ouvrir" }).click();
  const clicked = Date.now();
  await left;
  let after: string | undefined;
  await expect
    .poll(
      async () => {
        after = await startupId(guest);
        return after !== undefined && after !== before;
      },
      { timeout: 60_000, message: "another process image answers the application" },
    )
    .toBe(true);
  return { before: before as string, after: after as string, clicked, folder };
}

test("PC1-E2E a join restarts Inkwell on the document of the inviter", async ({ browser }) => {
  test.setTimeout(180_000);
  const host = await inviter(browser);
  // J-3: the person who joins is on a first launch, and restarts all the same.
  const me = fresh();
  const app = await startInstance(DAVE_PORT, me.docs, { home: me.home });
  try {
    const page = await open(browser, url(DAVE_PORT));
    const own = foldersIn(me.docs);
    expect(own, "a first launch has its one document").toHaveLength(1);

    const { before, after, clicked, folder } = await acceptInvitation(host.page, page);
    expect(after, "J-1 another process image").not.toBe(before);

    // The inviter's text, writable, and the sentence, once.
    await expect.poll(() => docText(page).catch(() => ""), { timeout: 60_000 }).toBe(HOST_TEXT);
    await expect(writable(page)).toBeVisible({ timeout: 30_000 });
    // How long the person waits, from « Rejoindre » to the joined document, writable.
    test.info().annotations.push({
      type: "first-launch join ms",
      description: `${Date.now() - clicked}`,
    });
    await expect(toast(page, JOINED)).toHaveCount(1, { timeout: 30_000 });
    await expect(waitingPage(page)).toHaveCount(0);
    await expect(banner(page)).toHaveCount(0);
    await expect(page).toHaveURL(url(DAVE_PORT));

    await typeAtEnd(page, " Et la suite.");
    await expect
      .poll(() => docText(host.page), { timeout: 60_000 })
      .toBe(`${HOST_TEXT} Et la suite.`);

    // J-6: a new folder beside the one that was open, which is still there.
    const now = foldersIn(me.docs);
    expect(now, "one new folder beside the previous one").toHaveLength(2);
    expect(now).toEqual(expect.arrayContaining(own));
    expect(
      now.filter((name) => !own.includes(name)),
      "J-2 the folder named before the restart is the one made",
    ).toEqual([folder]);

    // The record is consumed: a reloaded page opens the joined document and says nothing more.
    await page.reload();
    await expect(writable(page)).toBeVisible({ timeout: 30_000 });
    await expect.poll(() => docText(page)).toBe(`${HOST_TEXT} Et la suite.`);
    expect(await startupId(page)).toBe(after);
    await expect(toast(page, JOINED)).toHaveCount(0);
  } finally {
    await stopInstance(app);
    await stopInstance(host.app);
  }
});

test("PC1-E2E a pending join waits while the inviter is away", async ({ browser }) => {
  test.setTimeout(180_000);
  const host = await inviter(browser);
  const me = fresh();
  let app = await startInstance(DAVE_PORT, me.docs, { home: me.home });
  let frozen = false;
  try {
    const page = await open(browser, url(DAVE_PORT));
    const own = foldersIn(me.docs);
    // The inviter goes away (its process is frozen) as soon as the invitation is accepted, before
    // Inkwell restarts here: the document cannot arrive.
    const { before, after } = await acceptInvitation(host.page, page, () => {
      signalOwned(host.app, "SIGSTOP");
      frozen = true;
    });
    expect(after).not.toBe(before);

    // The sentence in place of the editor, for as long as the inviter is away.
    await expect(waitingSentence(page)).toHaveText(WAITING, { timeout: 30_000 });
    await expect(editor(page)).toHaveCount(0);
    const joined = foldersIn(me.docs).filter((name) => !own.includes(name));
    expect(joined, "the joined folder is there").toHaveLength(1);
    const text = join(me.docs, joined[0] as string, "main.typ");
    await holds("J-2 no document is created while the join waits", () => !existsSync(text), 3000);
    await expect(waitingSentence(page)).toHaveText(WAITING);
    await expect(toast(page, JOINED)).toHaveCount(0);

    // Every start waits again: Inkwell is closed and opened, and still waits, on the joined folder.
    await stopInstance(app);
    app = await startInstance(DAVE_PORT, me.docs, { home: me.home });
    await page.goto(url(DAVE_PORT));
    await expect(waitingSentence(page)).toHaveText(WAITING, { timeout: 30_000 });
    await expect(editor(page)).toHaveCount(0);
    expect(existsSync(text), "J-2 still no document of its own").toBe(false);

    // The inviter is back: the document arrives, and the join ends as any other.
    signalOwned(host.app, "SIGCONT");
    frozen = false;
    await expect.poll(() => docText(page).catch(() => ""), { timeout: 60_000 }).toBe(HOST_TEXT);
    await expect(waitingPage(page)).toHaveCount(0);
    await expect(toast(page, JOINED)).toHaveCount(1, { timeout: 30_000 });
    await expect(writable(page)).toBeVisible({ timeout: 30_000 });
  } finally {
    if (frozen) signalOwned(host.app, "SIGCONT");
    await stopInstance(app);
    await stopInstance(host.app);
  }
});

test("PC1-E2E a waiting join returns to the previous document", async ({ browser }) => {
  test.setTimeout(180_000);
  const PREVIOUS = "Mon texte d'avant l'invitation.";
  const host = await inviter(browser);
  const me = fresh();
  const app = await startInstance(DAVE_PORT, me.docs, { home: me.home });
  let frozen = false;
  try {
    const page = await open(browser, url(DAVE_PORT));
    await clear(page);
    await typeAtEnd(page, PREVIOUS);
    await expect.poll(() => readDisk(mainTypIn(me.docs)), { timeout: 20_000 }).toContain(PREVIOUS);
    const previous = mainTypIn(me.docs);
    const own = foldersIn(me.docs);

    // The inviter goes away as soon as the invitation is accepted: the join waits.
    const { after } = await acceptInvitation(host.page, page, () => {
      signalOwned(host.app, "SIGSTOP");
      frozen = true;
    });
    await expect(waitingSentence(page)).toHaveText(WAITING, { timeout: 30_000 });
    // J-2: the page says what happens next, and nothing on the screen contradicts the wait.
    await expect(waitingPage(page)).toContainText(WAITING_NEXT);
    await expect(page.getByTestId("preview-empty")).toHaveCount(0);
    await expect(page.locator("footer .people")).not.toContainText("Seul");
    const joined = foldersIn(me.docs).filter((name) => !own.includes(name));
    expect(joined, "the joined folder is there").toHaveLength(1);

    // « Revenir à mon document »: Inkwell restarts on the folder that was open before.
    await waitingPage(page).getByRole("button", { name: "Revenir à mon document" }).click();
    let back: string | undefined;
    await expect
      .poll(
        async () => {
          back = await startupId(page);
          return back !== undefined && back !== after;
        },
        { timeout: 60_000, message: "another process image answers the application" },
      )
      .toBe(true);
    await expect.poll(() => docText(page).catch(() => ""), { timeout: 60_000 }).toBe(PREVIOUS);
    await expect(writable(page)).toBeVisible({ timeout: 30_000 });
    await expect(waitingPage(page)).toHaveCount(0);
    await expect(page.locator(".toast")).toHaveCount(0);
    await expect(banner(page)).toHaveCount(0);
    await expect(page).toHaveURL(url(DAVE_PORT));
    await typeAtEnd(page, " Et je continue.");
    await expect
      .poll(() => readDisk(previous), { timeout: 20_000 })
      .toContain(`${PREVIOUS} Et je continue.`);
    // The join still waits in its own folder, which holds no document of its own.
    expect(foldersIn(me.docs)).toEqual(expect.arrayContaining([...own, ...joined]));
    expect(existsSync(join(me.docs, joined[0] as string, "main.typ"))).toBe(false);
  } finally {
    if (frozen) signalOwned(host.app, "SIGCONT");
    await stopInstance(app);
    await stopInstance(host.app);
  }
});

test("PC1-E2E after a join and a save of the joined document, the previous text is still in its folder", async ({
  browser,
}) => {
  test.setTimeout(180_000);
  const PREVIOUS = "Mon texte d'avant l'invitation.";
  const host = await inviter(browser);
  const me = fresh();
  const app = await startInstance(DAVE_PORT, me.docs, { home: me.home });
  try {
    const page = await open(browser, url(DAVE_PORT));
    await clear(page);
    await typeAtEnd(page, PREVIOUS);
    await expect.poll(() => readDisk(mainTypIn(me.docs)), { timeout: 20_000 }).toContain(PREVIOUS);
    const previous = mainTypIn(me.docs);

    const { clicked } = await acceptInvitation(host.page, page);
    await expect.poll(() => docText(page).catch(() => ""), { timeout: 60_000 }).toBe(HOST_TEXT);
    await expect(writable(page)).toBeVisible({ timeout: 30_000 });
    test.info().annotations.push({ type: "join ms", description: `${Date.now() - clicked}` });
    const before = readDisk(previous);
    expect(before, "the edits made before « Rejoindre » are in the previous folder").toContain(
      PREVIOUS,
    );

    // The joined document is edited and saved, in its own folder.
    await typeAtEnd(page, " Suite écrite ici.");
    const saved = `${HOST_TEXT} Suite écrite ici.`;
    await expect.poll(() => readDisk(mainTypIn(me.docs)), { timeout: 20_000 }).toContain(saved);
    const joined = mainTypIn(me.docs);
    expect(dirname(joined), "J-6 a folder of its own").not.toBe(dirname(previous));
    expect(dirname(dirname(joined)), "J-6 beside the previous one").toBe(
      dirname(dirname(previous)),
    );

    // J-6: the previous folder holds what it held, and nothing of the joined document.
    expect(readDisk(previous), "J-6 previous folder unchanged").toBe(before);
    expect(readDisk(previous)).not.toContain(HOST_TEXT);
  } finally {
    await stopInstance(app);
    await stopInstance(host.app);
  }
});
