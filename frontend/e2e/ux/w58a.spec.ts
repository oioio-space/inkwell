import { cpSync, mkdirSync, mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import type { Browser, Page } from "@playwright/test";
import { mainTypIn, readDisk } from "../disk";
import { expect, test } from "../fixtures";
import { clear, docText, editor, freshInvitation, rename, typeAtEnd } from "../helpers";
import { startInstance, stopInstance } from "../instance";
import { CAROL_PORT, SPARE_PORT } from "../ports";

// W58a UX walkthrough (gates/W58a/UX.md). Evidence goes to gates/W58a/ux/ (ignored by git except
// *.md: captures are kept locally). Run on purpose, not part of the default run: set W58A_UX=1.

const SHOTS = resolve(import.meta.dirname, "../../../gates/W58a/ux");
mkdirSync(SHOTS, { recursive: true });
const FIRST = "Texte du premier profil. Une deuxième phrase pour la sélection.";
const url = (port: number) => `http://127.0.0.1:${port}/`;
const banner = (p: Page) => p.getByTestId("access-banner");
const shot = (p: Page, name: string) => p.screenshot({ path: join(SHOTS, `${name}.png`) });

interface Profile {
  home: string;
  docs: string;
}
const profile = (docs = mkdtempSync(join(tmpdir(), "inkwell-ux-docs-"))): Profile => ({
  home: mkdtempSync(join(tmpdir(), "inkwell-ux-home-")),
  docs,
});

/** A page like helpers.open, but it does not wait for the lock to end: timings need the raw load. */
async function rawPage(browser: Browser, port: number, dark: boolean, width = 1280, height = 800) {
  const ctx = await browser.newContext({
    locale: "fr-FR",
    viewport: { width, height },
    colorScheme: dark ? "dark" : "light",
  });
  return ctx.newPage();
}

async function firstProfileWrites(browser: Browser, first: Profile): Promise<void> {
  const app = await startInstance(SPARE_PORT, first.docs, { home: first.home });
  try {
    const page = await rawPage(browser, SPARE_PORT, false);
    await page.goto(url(SPARE_PORT));
    await expect(page.locator('.cm-content:not([aria-readonly="true"])')).toBeVisible();
    await clear(page);
    await typeAtEnd(page, FIRST);
    await expect.poll(() => readDisk(mainTypIn(first.docs)), { timeout: 20_000 }).toContain(FIRST);
    await page.context().close();
  } finally {
    await stopInstance(app);
  }
}

test.skip(!process.env.W58A_UX, "UX walkthrough, run on purpose");

test("W58a-UX-1 Alice reinstalled and kept her Documents", async ({ browser }) => {
  test.setTimeout(240_000);
  const first = profile();
  await firstProfileWrites(browser, first);
  const app = await startInstance(CAROL_PORT, first.docs, { home: profile().home });
  try {
    for (const dark of [false, true]) {
      const tag = dark ? "dark" : "light";
      const page = await rawPage(browser, CAROL_PORT, dark);
      await page.goto(url(CAROL_PORT));
      await expect(banner(page)).toBeVisible({ timeout: 30_000 });
      await expect.poll(() => docText(page)).toContain("Texte du premier");
      await page.waitForTimeout(500);
      await shot(page, `s1-banner-full-${tag}`);
      await page.setViewportSize({ width: 720, height: 800 });
      await page.waitForTimeout(300);
      await shot(page, `s1-banner-720-${tag}`);
      await page.setViewportSize({ width: 1280, height: 800 });
      await page.context().close();
    }
  } finally {
    await stopInstance(app);
  }
});

import { appendFileSync } from "node:fs";

const LOG = join(SHOTS, "observations.log");
const log = (...a: unknown[]) =>
  appendFileSync(
    LOG,
    `${a.map((x) => (typeof x === "string" ? x : JSON.stringify(x))).join(" ")}\n`,
  );

test("W58a-UX-1b Alice explores: copy, type, format, dialog, menus", async ({ browser }) => {
  test.setTimeout(200_000);
  const first = profile();
  await firstProfileWrites(browser, first);
  const app = await startInstance(CAROL_PORT, first.docs, { home: profile().home });
  try {
    const ctx = await browser.newContext({
      locale: "fr-FR",
      viewport: { width: 1280, height: 800 },
      permissions: ["clipboard-read", "clipboard-write"],
    });
    const page = await ctx.newPage();
    page.setDefaultTimeout(3000);
    await page.goto(url(CAROL_PORT));
    await expect(banner(page)).toBeVisible({ timeout: 30_000 });
    await expect.poll(() => docText(page)).toContain("Texte du premier");
    const before = await docText(page);
    log(
      "BUTTONS",
      await page
        .getByRole("button")
        .evaluateAll((els) =>
          els.map(
            (e) =>
              `${e.getAttribute("aria-label") ?? e.textContent?.trim()}${(e as HTMLButtonElement).disabled ? "[disabled]" : ""}${e.getAttribute("aria-disabled") === "true" ? "[aria-disabled]" : ""}`,
          ),
        ),
    );
    log(
      "EDITOR attrs",
      await editor(page).evaluate((e) => ({
        ro: e.getAttribute("aria-readonly"),
        ce: e.getAttribute("contenteditable"),
      })),
    );
    await editor(page).click();
    await page.keyboard.press("ControlOrMeta+a");
    await page.keyboard.press("ControlOrMeta+c");
    log("COPY keyboard", await page.evaluate(() => navigator.clipboard.readText()));
    await page.locator(".cm-line").first().dblclick();
    await page.keyboard.press("ControlOrMeta+c");
    log("COPY dblclick", await page.evaluate(() => navigator.clipboard.readText()));
    // mouse drag selection then copy
    const box = (await page.locator(".cm-line").first().boundingBox())!;
    await page.mouse.move(box.x + 5, box.y + 8);
    await page.mouse.down();
    await page.mouse.move(box.x + 200, box.y + 8, { steps: 5 });
    await page.mouse.up();
    await page.keyboard.press("ControlOrMeta+c");
    log("COPY drag", await page.evaluate(() => navigator.clipboard.readText()));
    for (const k of [
      "x",
      "Enter",
      "Backspace",
      "Delete",
      "ControlOrMeta+b",
      "ControlOrMeta+i",
      "ControlOrMeta+z",
      "ControlOrMeta+v",
      "ControlOrMeta+x",
      "Tab",
      "ControlOrMeta+Shift+x",
    ]) {
      await page.keyboard.press(k);
    }
    await page.keyboard.insertText("ins");
    await page.waitForTimeout(300);
    log("UNCHANGED after keys", (await docText(page)) === before);
    await page.keyboard.press("ControlOrMeta+f");
    await page.waitForTimeout(300);
    await shot(page, "s1-ctrlf-light");
    log("CTRL+F panel", await page.locator(".cm-search, [role=search]").count());
    await page.keyboard.press("Escape");
    await shot(page, "s1-after-keys-light");
    await page
      .getByRole("button", { name: "Historique" })
      .click()
      .catch((e) => log("hist", String(e).slice(0, 60)));
    await page.waitForTimeout(600);
    await shot(page, "s1-history-light");
    log("HISTORY text", await page.locator("body").innerText());
    await page.keyboard.press("Escape");
    await page.getByRole("button", { name: "Partager" }).click();
    await page.waitForTimeout(800);
    await shot(page, "s1-share-nonmember-light");
    log("DIALOG", await page.getByRole("dialog").innerText());
    await ctx.close();
  } finally {
    await stopInstance(app);
  }
});

test("W58a-UX-1c keyboard only: banner action, dialog, Esc, focus return; locked controls", async ({
  browser,
}) => {
  test.setTimeout(200_000);
  const first = profile();
  await firstProfileWrites(browser, first);
  const app = await startInstance(CAROL_PORT, first.docs, { home: profile().home });
  try {
    const ctx = await browser.newContext({
      locale: "fr-FR",
      viewport: { width: 1280, height: 800 },
    });
    const page = await ctx.newPage();
    page.setDefaultTimeout(3000);
    await page.goto(url(CAROL_PORT));
    await expect(banner(page)).toBeVisible({ timeout: 30_000 });
    await expect.poll(() => docText(page)).toContain("Texte du premier");
    const gras = page.getByRole("button", { name: "Gras" });
    log(
      "GRAS attrs",
      await gras.evaluate((e) => ({
        disabled: (e as HTMLButtonElement).disabled,
        aria: e.getAttribute("aria-disabled"),
        inert: e.closest("[inert]") !== null,
        cls: e.className,
        tabindex: e.getAttribute("tabindex"),
        pe: getComputedStyle(e).pointerEvents,
      })),
    );
    log("GRAS count visible", await gras.count());
    // focus order from document start
    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    const order: string[] = [];
    for (let i = 0; i < 40; i++) {
      await page.keyboard.press("Tab");
      const d = await page.evaluate(() => {
        const e = document.activeElement as HTMLElement;
        return `${e.tagName}:${e.getAttribute("aria-label") ?? e.textContent?.trim().slice(0, 30)}`;
      });
      order.push(d);
      if (d.includes("Demander une invitation")) break;
    }
    log("TAB ORDER", order);
    await shot(page, "s1-focus-ask-light");
    await page.keyboard.press("Enter");
    await page.waitForTimeout(500);
    log("AFTER ENTER dialog", await page.getByRole("dialog").count());
    log(
      "FOCUS in dialog",
      await page.evaluate(() => {
        const e = document.activeElement as HTMLElement;
        return `${e.tagName}:${e.getAttribute("aria-label") ?? e.getAttribute("placeholder") ?? e.textContent?.trim().slice(0, 30)}`;
      }),
    );
    await shot(page, "s1-dialog-from-banner-light");
    await page.keyboard.press("Escape");
    await page.waitForTimeout(400);
    log("AFTER ESC dialog", await page.getByRole("dialog").count());
    log(
      "FOCUS after Esc",
      await page.evaluate(() => {
        const e = document.activeElement as HTMLElement;
        return `${e.tagName}:${e.getAttribute("aria-label") ?? e.textContent?.trim().slice(0, 30)}`;
      }),
    );
    // Partager via keyboard, then Esc
    await page.getByRole("button", { name: "Partager" }).focus();
    await page.keyboard.press("Enter");
    await page.waitForTimeout(400);
    log(
      "FOCUS in share dialog (via Partager)",
      await page.evaluate(() => {
        const e = document.activeElement as HTMLElement;
        return `${e.tagName}:${e.getAttribute("aria-label") ?? e.getAttribute("placeholder") ?? e.textContent?.trim().slice(0, 30)}`;
      }),
    );
    log(
      "Inviter disabled attrs",
      await page.getByRole("button", { name: "Inviter" }).evaluate((e) => ({
        disabled: (e as HTMLButtonElement).disabled,
        aria: e.getAttribute("aria-disabled"),
        describedby: e.getAttribute("aria-describedby"),
      })),
    );
    await page.keyboard.press("Escape");
    log(
      "FOCUS after Esc (share)",
      await page.evaluate(() => {
        const e = document.activeElement as HTMLElement;
        return `${e.tagName}:${e.getAttribute("aria-label") ?? e.textContent?.trim().slice(0, 30)}`;
      }),
    );
    // review / history panel
    await page.getByRole("button", { name: "Relire", exact: true }).click();
    await page.waitForTimeout(800);
    await shot(page, "s1-review-panel-light");
    log(
      "REVIEW buttons",
      await page
        .getByRole("button")
        .evaluateAll((els) =>
          els.map(
            (e) =>
              `${e.getAttribute("aria-label") ?? e.textContent?.trim()}${(e as HTMLButtonElement).disabled ? "[disabled]" : ""}`,
          ),
        ),
    );
    await ctx.close();
  } finally {
    await stopInstance(app);
  }
});

test("W58a-UX-1d history entry actions while read-only", async ({ browser }) => {
  test.setTimeout(200_000);
  const first = profile();
  await firstProfileWrites(browser, first);
  const app = await startInstance(CAROL_PORT, first.docs, { home: profile().home });
  try {
    const ctx = await browser.newContext({
      locale: "fr-FR",
      viewport: { width: 1400, height: 900 },
    });
    const page = await ctx.newPage();
    page.setDefaultTimeout(3000);
    await page.goto(url(CAROL_PORT));
    await expect(banner(page)).toBeVisible({ timeout: 30_000 });
    await page.getByRole("button", { name: "Relire", exact: true }).click();
    await page.waitForTimeout(800);
    await shot(page, "s1-history-open-light");
    await page.mouse.click(1100, 270);
    await page.waitForTimeout(800);
    await shot(page, "s1-history-entry-light");
    log(
      "ENTRY buttons",
      await page
        .getByRole("button")
        .evaluateAll((els) =>
          els.map(
            (e) =>
              `${e.getAttribute("aria-label") ?? e.textContent?.trim()}${(e as HTMLButtonElement).disabled ? "[disabled]" : ""}${e.closest("[inert]") ? "[inert]" : ""}`,
          ),
        ),
    );
    log(
      "ENTRY panel text",
      await page
        .locator("aside, [role=complementary], .panel")
        .last()
        .innerText()
        .catch(() => "?"),
    );
    await ctx.close();
  } finally {
    await stopInstance(app);
  }
});

test("W58a-UX-1e other write doors while read-only", async ({ browser }) => {
  test.setTimeout(200_000);
  const first = profile();
  await firstProfileWrites(browser, first);
  const app = await startInstance(CAROL_PORT, first.docs, { home: profile().home });
  try {
    const ctx = await browser.newContext({
      locale: "fr-FR",
      viewport: { width: 1400, height: 900 },
    });
    const page = await ctx.newPage();
    page.setDefaultTimeout(3000);
    await page.goto(url(CAROL_PORT));
    await expect(banner(page)).toBeVisible({ timeout: 30_000 });
    await page.getByText("main.typ").first().click({ button: "right" });
    await page.waitForTimeout(400);
    await shot(page, "s1-file-context-light");
    log(
      "CTX menu",
      await page
        .getByRole("menu")
        .first()
        .innerText()
        .catch(() => "none"),
    );
    await page.keyboard.press("Escape");
    // title click
    await page
      .getByText("Sans titre")
      .first()
      .click()
      .catch((e) => log("title", String(e).slice(0, 50)));
    await page.waitForTimeout(300);
    log("TITLE editable?", await page.locator("input:focus, [contenteditable=true]:focus").count());
    await page.keyboard.press("Escape");
    await page.getByRole("button", { name: "Voir les personnes" }).click();
    await page.waitForTimeout(500);
    await shot(page, "s1-people-light");
    log("PEOPLE", await page.locator("body").innerText());
    await page.keyboard.press("Escape");
    await page.getByRole("button", { name: "Relire", exact: true }).click();
    for (const tab of ["Commentaires", "Activité"]) {
      await page
        .getByRole("tab", { name: tab })
        .click()
        .catch(async () => page.getByText(tab, { exact: true }).first().click());
      await page.waitForTimeout(400);
      await shot(page, `s1-tab-${tab}-light`);
      log(
        `TAB ${tab} buttons`,
        await page
          .getByRole("button")
          .evaluateAll((els) =>
            els
              .map(
                (e) =>
                  `${e.getAttribute("aria-label") ?? e.textContent?.trim()}${(e as HTMLButtonElement).disabled ? "[disabled]" : ""}`,
              )
              .slice(10),
          ),
      );
    }
    // drop a file / paste image: dispatch a paste with a file
    await ctx.close();
  } finally {
    await stopInstance(app);
  }
});

test("W58a-UX-2 Bob asks for an invitation, pastes, joins and writes", async ({ browser }) => {
  test.setTimeout(240_000);
  const first = profile();
  await firstProfileWrites(browser, first);
  const copy = mkdtempSync(join(tmpdir(), "inkwell-ux-copy-"));
  cpSync(first.docs, copy, { recursive: true });
  const firstApp = await startInstance(SPARE_PORT, first.docs, { home: first.home });
  try {
    const hostCtx = await browser.newContext({
      locale: "fr-FR",
      viewport: { width: 1280, height: 800 },
    });
    const host = await hostCtx.newPage();
    await host.goto(url(SPARE_PORT));
    await expect(host.locator('.cm-content:not([aria-readonly="true"])')).toBeVisible();
    const secondApp = await startInstance(CAROL_PORT, copy, { home: profile().home });
    try {
      for (const dark of [process.env.W58A_DARK === "1"]) {
        const tag = dark ? "dark" : "light";
        // dark pass: a second copy joins would need another invitation; only the dialog is captured in dark
        const ctx = await browser.newContext({
          locale: "fr-FR",
          viewport: { width: 1280, height: 800 },
          colorScheme: dark ? "dark" : "light",
        });
        const page = await ctx.newPage();
        page.setDefaultTimeout(5000);
        await page.goto(url(CAROL_PORT));
        await expect(banner(page)).toBeVisible({ timeout: 30_000 });
        await page.getByRole("button", { name: "Demander une invitation" }).click();
        await page.waitForTimeout(500);
        await shot(page, `s2-ask-dialog-${tag}`);
        const invitation = await freshInvitation(host, "Premier");
        await rename(page, "Bob");
        await page.getByPlaceholder("Collez l'invitation ici").fill(invitation);
        await shot(page, `s2-pasted-${tag}`);
        const t0 = Date.now();
        await page.getByRole("button", { name: "Rejoindre" }).click();
        const marks: Record<string, number> = {};
        for (let i = 0; i < 1200 && Object.keys(marks).length < 4; i++) {
          const st = await page.evaluate(() => ({
            banner: document.querySelector('[data-testid="access-banner"]') !== null,
            ro: document.querySelector(".cm-content")?.getAttribute("aria-readonly") === "true",
            dialog: !!document.querySelector("dialog[open]"),
            status: document.querySelector('[role="status"]')?.textContent?.trim() ?? "",
          }));
          const at = Date.now() - t0;
          if (!st.banner && !("bannerGone" in marks)) marks.bannerGone = at;
          if (!st.ro && !("writable" in marks)) marks.writable = at;
          if (/Synchronisé/.test(st.status) && !("synced" in marks)) marks.synced = at;
          if (!st.dialog && !("dialogClosed" in marks)) marks.dialogClosed = at;
          if (i % 4 === 0) {
            if (at < 400) await shot(page, `s2-joining-${String(at).padStart(5, "0")}`);
          }
          await page.waitForTimeout(25);
        }
        log("JOIN marks (ms after Rejoindre)", marks);
        await page.waitForTimeout(1500);
        await shot(page, `s2-joined-${tag}`);
        log("JOINED status", await page.locator('[role="status"]').first().innerText());
        log("JOINED banner count", await banner(page).count());
        // write without reload
        await typeAtEnd(page, " Bob écrit.");
        await expect.poll(() => docText(page)).toContain("Bob écrit.");
        await expect.poll(() => docText(host), { timeout: 30_000 }).toContain("Bob écrit.");
        await shot(page, `s2-bob-wrote-${tag}`);
        await host.getByRole("button", { name: "Voir les personnes" }).click();
        await host.waitForTimeout(500);
        await shot(host, `s2-host-people-${tag}`);
        log("HOST people", await host.getByRole("dialog").innerText());
        await host.keyboard.press("Escape");
        await page.getByRole("button", { name: "Partager" }).click();
        await page.waitForTimeout(500);
        await shot(page, `s2-bob-share-after-join-${tag}`);
        log("BOB share dialog after join", await page.getByRole("dialog").innerText());
        await page.keyboard.press("Escape");
        await page.reload();
        await expect(page.locator('.cm-content:not([aria-readonly="true"])')).toBeVisible({
          timeout: 20000,
        });
        log("BOB after reload banner count", await banner(page).count());
        await ctx.close();
      }
      await hostCtx.close();
    } finally {
      await stopInstance(secondApp);
    }
  } finally {
    await stopInstance(firstApp);
  }
});

test("W58a-UX-3 a member's first launch: no visible lock, no foreign banner, times", async ({
  browser,
}) => {
  test.setTimeout(240_000);
  const docs = profile();
  const app = await startInstance(CAROL_PORT, docs.docs, { home: docs.home });
  try {
    const rows: Record<string, number | boolean>[] = [];
    for (let i = 0; i < 6; i++) {
      const ctx = await browser.newContext({
        locale: "fr-FR",
        viewport: { width: 1280, height: 800 },
      });
      await ctx.addInitScript(() => {
        const m: Record<string, number | boolean> = {};
        (window as unknown as { __m: typeof m }).__m = m;
        const mark = (k: string) => {
          if (!(k in m)) m[k] = Math.round(performance.now());
        };
        const poll = () => {
          const cm = document.querySelector(".cm-content");
          if (cm) {
            mark("editorShown");
            if (cm.getAttribute("aria-readonly") !== "true") mark("editable");
            else m.roSeen = true;
          }
          if (document.querySelector('[data-testid="access-banner"]')) mark("bannerSeen");
          const bold = document.querySelector('button[aria-label="Gras"]');
          if (bold) {
            mark("boldShown");
            if (!bold.closest("[inert]")) mark("boldActive");
          }
          const share = [...document.querySelectorAll("button")].find(
            (b) => b.textContent?.trim() === "Partager",
          );
          if (share && !m.clicked) {
            m.clicked = true;
            mark("shareClicked");
            share.click();
          }
          const inv = [...document.querySelectorAll("dialog[open] button")].find(
            (b) => b.textContent?.trim() === "Inviter",
          );
          if (inv) {
            mark("inviterShown");
            if (!(inv as HTMLButtonElement).disabled) mark("inviterEnabled");
          }
          if (document.querySelector("dialog[open]")?.textContent?.includes("Vérification"))
            mark("verifSeen");
          requestAnimationFrame(poll);
        };
        requestAnimationFrame(poll);
      });
      const page = await ctx.newPage();
      await page.goto(url(CAROL_PORT), { waitUntil: "commit" });
      if (i === 0) {
        for (let k = 0; k < 6; k++) {
          await shot(page, `s3-early-${k}`).catch(() => {});
          await page.waitForTimeout(150);
        }
      }
      await expect
        .poll(
          async () =>
            (await page.evaluate(
              () => (window as unknown as { __m: Record<string, number> }).__m.inviterEnabled,
            )) !== undefined,
          { timeout: 30_000 },
        )
        .toBe(true);
      await page.waitForTimeout(1500); // a late banner would show here
      const m = await page.evaluate(
        () => (window as unknown as { __m: Record<string, number | boolean> }).__m,
      );
      rows.push(m);
      log(`LOAD ${i + 1}`, m);
      if (i === 5) {
        await page.getByRole("button", { name: "Fermer" }).click();
        await shot(page, "s3-member-first-screen-light");
      }
      await ctx.close();
    }
    const ctx = await browser.newContext({
      locale: "fr-FR",
      viewport: { width: 1280, height: 800 },
      colorScheme: "dark",
    });
    const page = await ctx.newPage();
    await page.goto(url(CAROL_PORT));
    await expect(page.locator('.cm-content:not([aria-readonly="true"])')).toBeVisible();
    await page.waitForTimeout(1500);
    await shot(page, "s3-member-first-screen-dark");
    await page.setViewportSize({ width: 720, height: 800 });
    await page.waitForTimeout(400);
    await shot(page, "s3-member-720-dark");
    await ctx.close();
    const med = (k: string) => {
      const v = rows.map((r) => r[k] as number).sort((a, b) => a - b);
      return { min: v[0], median: v[Math.floor(v.length / 2)], max: v[v.length - 1] };
    };
    for (const k of ["editorShown", "editable", "boldActive", "inviterShown", "inviterEnabled"])
      log(`STAT ${k}`, med(k));
    log(
      "bannerSeen in any load",
      rows.some((r) => "bannerSeen" in r),
    );
    log(
      "roSeen in loads",
      rows.map((r) => !!r.roSeen),
    );
    log(
      "verifSeen in loads",
      rows.map((r) => "verifSeen" in r),
    );
  } finally {
    await stopInstance(app);
  }
});

const OBSERVER = () => {
  const m: Record<string, number> = {};
  (window as unknown as { __m: typeof m }).__m = m;
  const mark = (k: string) => {
    if (!(k in m)) m[k] = Math.round(performance.now());
  };
  const poll = () => {
    const cm = document.querySelector(".cm-content");
    if (cm && cm.getAttribute("aria-readonly") !== "true") mark("editable");
    if (
      !document.querySelector('[data-testid="access-banner"]') &&
      document.querySelector(".cm-content")
    )
      mark("noBanner");
    if (/Synchronisé/.test(document.querySelector('[role="status"]')?.textContent ?? ""))
      mark("synced");
    if (
      /personnes|2 /.test(document.querySelector("footer, [role=contentinfo]")?.textContent ?? "")
    )
      mark("twoPeople");
    requestAnimationFrame(poll);
  };
  requestAnimationFrame(poll);
};

test("W58a-UX-2b time from Rejoindre to a writable editor (2 joins)", async ({ browser }) => {
  test.setTimeout(360_000);
  const first = profile();
  await firstProfileWrites(browser, first);
  const firstApp = await startInstance(SPARE_PORT, first.docs, { home: first.home });
  try {
    const hostCtx = await browser.newContext({
      locale: "fr-FR",
      viewport: { width: 1280, height: 800 },
    });
    const host = await hostCtx.newPage();
    await host.goto(url(SPARE_PORT));
    await expect(host.locator('.cm-content:not([aria-readonly="true"])')).toBeVisible();
    for (let n = 1; n <= 2; n++) {
      const copy = mkdtempSync(join(tmpdir(), "inkwell-ux-copy-"));
      cpSync(first.docs, copy, { recursive: true });
      const secondApp = await startInstance(CAROL_PORT, copy, { home: profile().home });
      try {
        const ctx = await browser.newContext({
          locale: "fr-FR",
          viewport: { width: 1280, height: 800 },
        });
        await ctx.addInitScript(OBSERVER);
        const page = await ctx.newPage();
        await page.goto(url(CAROL_PORT));
        await expect(banner(page)).toBeVisible({ timeout: 30_000 });
        const invitation = await freshInvitation(host, "Premier");
        await page.getByRole("button", { name: "Demander une invitation" }).click();
        await page.getByPlaceholder("Collez l'invitation ici").fill(invitation);
        await page.evaluate(() => {
          const m = (window as unknown as { __m: Record<string, number> }).__m;
          m.click = Math.round(performance.now());
        });
        await page.getByRole("button", { name: "Rejoindre" }).click();
        await page.waitForTimeout(400);
        if ((await banner(page).count()) > 0) {
          log(`JOIN ${n} banner at 400ms`, await banner(page).innerText());
          log(
            `JOIN ${n} ask buttons at 400ms`,
            await page.getByRole("button", { name: "Demander une invitation" }).count(),
          );
          await shot(page, `s2-waiting-banner-${n}`);
          await page.waitForTimeout(2000);
          await shot(page, `s2-waiting-banner-${n}-plus2s`);
        }
        await expect
          .poll(
            () =>
              page.evaluate(
                () => (window as unknown as { __m: Record<string, number> }).__m.editable,
              ),
            { timeout: 60_000 },
          )
          .toBeDefined();
        await page.waitForTimeout(500);
        const m = await page.evaluate(
          () => (window as unknown as { __m: Record<string, number> }).__m,
        );
        const rel = Object.fromEntries(
          Object.entries(m)
            .filter(([k]) => k !== "click")
            .map(([k, v]) => [k, v - m.click]),
        );
        log(`JOIN ${n} ms after click`, rel);
        await ctx.close();
      } finally {
        await stopInstance(secondApp);
      }
    }
  } finally {
    await stopInstance(firstApp);
  }
});

test("W58a-UX-1f dark: ask dialog, share dialog of a non-member, keyboard focus", async ({
  browser,
}) => {
  test.setTimeout(200_000);
  const first = profile();
  await firstProfileWrites(browser, first);
  const app = await startInstance(CAROL_PORT, first.docs, { home: profile().home });
  try {
    const ctx = await browser.newContext({
      locale: "fr-FR",
      viewport: { width: 1280, height: 800 },
      colorScheme: "dark",
    });
    const page = await ctx.newPage();
    page.setDefaultTimeout(5000);
    await page.goto(url(CAROL_PORT));
    await expect(banner(page)).toBeVisible({ timeout: 30_000 });
    await page.getByRole("button", { name: "Demander une invitation" }).focus();
    await shot(page, "s1-focus-ask-dark");
    await page.keyboard.press("Enter");
    await page.waitForTimeout(500);
    await shot(page, "s2-ask-dialog-dark");
    await page.keyboard.press("Escape");
    await page.getByRole("button", { name: "Partager" }).click();
    await page.waitForTimeout(500);
    await shot(page, "s1-share-nonmember-dark");
    await page.keyboard.press("Escape");
    await page.setViewportSize({ width: 720, height: 800 });
    await page.getByRole("button", { name: "Partager" }).click();
    await page.waitForTimeout(500);
    await shot(page, "s1-share-nonmember-720-dark");
    await page.keyboard.press("Escape");
    await page.getByRole("button", { name: "Demander une invitation" }).click();
    await page.waitForTimeout(500);
    await shot(page, "s2-ask-dialog-720-dark");
    // contrast of banner text and action
    const c = await page.evaluate(() => {
      const f = (el: Element | null) =>
        el ? { color: getComputedStyle(el).color, bg: getComputedStyle(el).backgroundColor } : null;
      const b = document.querySelector('[data-testid="access-banner"]');
      return {
        text: f(b),
        bannerBox: f(b?.parentElement ?? null),
        btn: f(document.querySelector('[data-testid="ask-invitation"]')),
      };
    });
    log("DARK colors", c);
    await ctx.close();
  } finally {
    await stopInstance(app);
  }
});

test("W58a-UX-1g axe, forced colors and 200 percent on the foreign banner and dialog", async ({
  browser,
}) => {
  test.setTimeout(200_000);
  const { default: AxeBuilder } = await import("@axe-core/playwright");
  const first = profile();
  await firstProfileWrites(browser, first);
  const app = await startInstance(CAROL_PORT, first.docs, { home: profile().home });
  try {
    for (const dark of [false, true]) {
      const tag = dark ? "dark" : "light";
      const ctx = await browser.newContext({
        locale: "fr-FR",
        viewport: { width: 1280, height: 800 },
        colorScheme: dark ? "dark" : "light",
      });
      const page = await ctx.newPage();
      page.setDefaultTimeout(5000);
      await page.goto(url(CAROL_PORT));
      await expect(banner(page)).toBeVisible({ timeout: 30_000 });
      await page.waitForTimeout(800);
      const res = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
      log(
        `AXE page ${tag}`,
        res.violations.map((v) => ({
          id: v.id,
          impact: v.impact,
          n: v.nodes.length,
          first: v.nodes[0]?.target,
        })),
      );
      await page.getByRole("button", { name: "Demander une invitation" }).click();
      await page.waitForTimeout(500);
      const res2 = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
      log(
        `AXE dialog ${tag}`,
        res2.violations.map((v) => ({
          id: v.id,
          impact: v.impact,
          n: v.nodes.length,
          first: v.nodes[0]?.target,
        })),
      );
      await page.keyboard.press("Escape");
      if (!dark) {
        await page.emulateMedia({ forcedColors: "active" });
        await page.waitForTimeout(300);
        await shot(page, "s1-forced-colors-light");
        await page.emulateMedia({ forcedColors: "none" });
        await page.setViewportSize({ width: 640, height: 400 });
        await page.waitForTimeout(500);
        await shot(page, "s1-zoom200-light");
      }
      await ctx.close();
    }
  } finally {
    await stopInstance(app);
  }
});
