import { cpSync, existsSync, mkdtempSync, readdirSync, utimesSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { Browser, Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { editor } from "./helpers";
import { launchAgain, startInstance, stopInstance } from "./instance";
import { CAROL_PORT, OFFLINE_BOB_PORT, SPARE_PORT, SPARE_URL } from "./ports";

// Block OI (W41): « Ouvrir avec Inkwell » and one Inkwell per profile, with real inkwell-server
// processes (plan docs/plans/2026-11-11-oi-single-instance-open.md, rows R2, R3, R6, R10, R14, R16).
// A second launch is started as the desktop would: same profile (INKWELL_HOME), the file as its
// argument; it must hand the file to the running instance and exit 0 within 5 s.

/** A fresh profile and projects folder; the second launch gets this port only if it wrongly starts. */
function fresh() {
  const root = mkdtempSync(join(tmpdir(), "inkwell-e2e-oi-"));
  return { home: join(root, "home"), docs: join(root, "docs") };
}

/** The project folders of docs. */
const projects = (docs: string) =>
  readdirSync(docs).filter((d) => existsSync(join(docs, d, ".inkwell", "project.json")));

async function openPage(browser: Browser, url: string): Promise<Page> {
  const page = await (await browser.newContext({ locale: "fr-FR" })).newPage();
  await page.goto(url);
  await expect(editor(page)).toBeVisible();
  return page;
}

/** The name of the file the window shows (the header bar subtitle). */
const shown = (page: Page) => page.locator(".ttl small");

test("OI second launch shows a file of the current project in the running window", async ({
  browser,
}) => {
  const { home, docs } = fresh();
  const primary = await startInstance(SPARE_PORT, docs, { home });
  try {
    const page = await openPage(browser, SPARE_URL);
    const [project] = projects(docs);
    expect(project, "the instance made its project").toBeTruthy();
    const dir = join(docs, project as string);
    writeFileSync(join(dir, "notes.md"), "# Notes OI\n");
    // Relative to the second launch's working directory, as a file manager gives it.
    const second = await launchAgain(OFFLINE_BOB_PORT, docs, {
      home,
      args: ["notes.md"],
      cwd: dir,
    });
    expect(second.code, "the second launch exits 0").toBe(0);
    expect(second.ms, "within 5 s").toBeLessThan(5_000);
    await expect(shown(page)).toHaveText("notes.md");
    await expect(editor(page)).toContainText("# Notes OI");
    await expect(page.locator(".toast")).toHaveCount(0);
  } finally {
    await stopInstance(primary);
  }
});

test("OI second launch with no file changes nothing", async ({ browser }) => {
  const { home, docs } = fresh();
  const primary = await startInstance(SPARE_PORT, docs, { home });
  try {
    const page = await openPage(browser, SPARE_URL);
    const before = await shown(page).textContent();
    const second = await launchAgain(OFFLINE_BOB_PORT, docs, { home });
    expect(second.code).toBe(0);
    expect(second.ms).toBeLessThan(5_000);
    await page.waitForTimeout(1_000); // what the request would show has had time to arrive
    await expect(page.locator(".toast")).toHaveCount(0);
    await expect(shown(page)).toHaveText(before ?? "");
    expect(projects(docs), "no project made by the second launch").toHaveLength(1);
  } finally {
    await stopInstance(primary);
  }
});

test("OI second launch with a loose file shows the toast", async ({ browser }) => {
  const { home, docs } = fresh();
  const primary = await startInstance(SPARE_PORT, docs, { home });
  try {
    const page = await openPage(browser, SPARE_URL);
    const before = await shown(page).textContent();
    const loose = join(mkdtempSync(join(tmpdir(), "inkwell-e2e-loose-")), "brouillon.md");
    writeFileSync(loose, "# Seul\n");
    const second = await launchAgain(OFFLINE_BOB_PORT, docs, { home, args: [loose] });
    expect(second.code).toBe(0);
    await expect(page.locator(".toast")).toHaveText(
      "« brouillon.md » n'est dans aucun projet. Inkwell ne sait pas encore ouvrir un fichier seul.",
    );
    await expect(shown(page)).toHaveText(before ?? "");
  } finally {
    await stopInstance(primary);
  }
});

test("OI first launch with a file opens its project", async ({ browser }) => {
  const { home, docs } = fresh();
  // A first project, then a copy of it made the most recent one: without an argument, Inkwell
  // would open the copy (R1).
  await stopInstance(await startInstance(SPARE_PORT, docs, { home }));
  const [first] = projects(docs);
  expect(first).toBeTruthy();
  cpSync(join(docs, first as string), join(docs, "Copie"), { recursive: true });
  const later = new Date(Date.now() + 60_000);
  utimesSync(join(docs, "Copie", ".inkwell", "project.json"), later, later);
  writeFileSync(join(docs, first as string, "chapitre.md"), "# Chapitre OI\n");

  const primary = await startInstance(SPARE_PORT, docs, {
    home,
    args: [join(docs, first as string, "chapitre.md")],
  });
  try {
    const page = await openPage(browser, SPARE_URL);
    await expect(shown(page)).toHaveText("chapitre.md");
    await expect(editor(page)).toContainText("# Chapitre OI");
    await expect(page.locator(".toast")).toHaveCount(0);
  } finally {
    await stopInstance(primary);
  }
});

test("OI another profile runs separately", async ({ browser }) => {
  const a = fresh();
  const b = fresh();
  const first = await startInstance(SPARE_PORT, a.docs, { home: a.home });
  try {
    // Another INKWELL_HOME is another Inkwell (R14): it serves its own window.
    const other = await startInstance(CAROL_PORT, b.docs, { home: b.home });
    try {
      const page = await openPage(browser, `http://127.0.0.1:${CAROL_PORT}/`);
      await expect(page.locator(".toast")).toHaveCount(0);
      expect(projects(b.docs)).toHaveLength(1);
    } finally {
      await stopInstance(other);
    }
    expect((await fetch(`${SPARE_URL}health`)).ok, "the first instance still runs").toBe(true);
  } finally {
    await stopInstance(first);
  }
});

test("OI second launch with a file whose name is not UTF-8 shows the toast", async ({
  browser,
}) => {
  const { home, docs } = fresh();
  const primary = await startInstance(SPARE_PORT, docs, { home });
  try {
    const page = await openPage(browser, SPARE_URL);
    const before = await shown(page).textContent();
    const [project] = projects(docs);
    const dir = join(docs, project as string);
    // A file of the open project named with the byte 0xFF: JSON would turn it into another name.
    writeFileSync(
      Buffer.concat([Buffer.from(`${dir}/`), Buffer.from([0xff]), Buffer.from(".md")]),
      "# Octet\n",
    );
    const second = await launchAgain(OFFLINE_BOB_PORT, docs, {
      home,
      cwd: dir,
      shell: 'exec "$0" "$(printf "\\377.md")"',
    });
    expect(second.code).toBe(0);
    await expect(page.locator(".toast")).toContainText("Inkwell ne sait pas lire le nom de");
    await expect(shown(page)).toHaveText(before ?? "");
  } finally {
    await stopInstance(primary);
  }
});

test("OI second launch whose file the window cannot show is said in the window", async ({
  browser,
}) => {
  const { home, docs } = fresh();
  const primary = await startInstance(SPARE_PORT, docs, { home });
  try {
    const page = await openPage(browser, SPARE_URL);
    const before = await shown(page).textContent();
    const [project] = projects(docs);
    const dir = join(docs, project as string);
    // A hidden file is not in the project's list: the window cannot show it and says so (R19).
    writeFileSync(join(dir, ".brouillon.md"), "# Caché\n");
    const second = await launchAgain(OFFLINE_BOB_PORT, docs, {
      home,
      cwd: dir,
      args: [".brouillon.md"],
      env: { LC_ALL: "fr_FR.UTF-8" },
    });
    expect(second.code, "received and queued: the launch ends with 0").toBe(0);
    await expect(page.locator(".toast")).toHaveText("Inkwell n'a pas pu ouvrir « .brouillon.md ».");
    await expect(shown(page)).toHaveText(before ?? "");
  } finally {
    await stopInstance(primary);
  }
});

test("OI a burst of launches is applied one by one through the window", async ({ browser }) => {
  const { home, docs } = fresh();
  const primary = await startInstance(SPARE_PORT, docs, { home });
  try {
    const page = await openPage(browser, SPARE_URL);
    const [project] = projects(docs);
    const dir = join(docs, project as string);
    const names = Array.from({ length: 8 }, (_, i) => `rafale-${i}.md`);
    for (const n of names) writeFileSync(join(dir, n), `# ${n}\n`);
    // Every toast the window shows from now on, in order (a queued toast replaces the one before).
    await page.evaluate(() => {
      const seen: string[] = [];
      (globalThis as typeof globalThis & { __toastTexts?: string[] }).__toastTexts = seen;
      new MutationObserver(() => {
        for (const el of document.querySelectorAll(".toast .msg")) {
          const text = el.textContent ?? "";
          if (seen.at(-1) !== text) seen.push(text);
        }
      }).observe(document.body, { subtree: true, childList: true, characterData: true });
    });
    // Each launch ends with 0 once the primary queued its request; the window then applies them all.
    const results = await Promise.all(
      names.map((n) => launchAgain(OFFLINE_BOB_PORT, docs, { home, cwd: dir, args: [n] }, 30_000)),
    );
    expect(results.map((r) => r.code)).toEqual(names.map(() => 0));
    // Every request applied once, one after the other, in the order the window received them.
    const appliedLog = async () =>
      (
        await page.evaluate(
          () =>
            (
              globalThis as typeof globalThis & {
                __inkwellOpenApplied?: { n: number; show?: string }[];
              }
            ).__inkwellOpenApplied ?? [],
        )
      ).filter((e) => e.show !== undefined && names.includes(e.show));
    await expect
      .poll(async () => (await appliedLog()).length, { timeout: 15_000 })
      .toBe(names.length);
    const applied = await appliedLog();
    expect(applied.map((e) => e.show).sort()).toEqual([...names].sort());
    const ids = applied.map((e) => e.n);
    expect(ids, "applied in order").toEqual([...ids].sort((a, b) => a - b));
    expect(new Set(ids).size).toBe(names.length);
    await expect(shown(page)).toHaveText(applied.at(-1)?.show ?? "");
    // What the window said, whatever the bursts (timing decides how the launches group, UX-OI-1):
    // once every toast has gone, each was a summary of files shown, and the toasts together name
    // each file at most once; still exactly one application per file.
    await expect(page.locator(".toast")).toHaveCount(0, { timeout: 30_000 });
    const toasts = await page.evaluate(
      () => (globalThis as typeof globalThis & { __toastTexts?: string[] }).__toastTexts ?? [],
    );
    for (const text of toasts) expect(text).toMatch(/^[2-8] fichiers ouverts : « rafale-\d\.md »/);
    for (const n of names)
      expect(toasts.join(" ").split(`« ${n} »`).length - 1, `${n} named`).toBeLessThanOrEqual(1);
    expect((await appliedLog()).map((e) => e.show).sort()).toEqual([...names].sort());
  } finally {
    await stopInstance(primary);
  }
});
