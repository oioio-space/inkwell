import { type ChildProcess, spawn } from "node:child_process";
import { createHash } from "node:crypto";
import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  openSync,
  readdirSync,
  readFileSync,
  rmSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join, relative, resolve } from "node:path";
import AxeBuilder from "@axe-core/playwright";
import type { Browser, Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { docText, editor, open, pair, rename } from "./helpers";
import { ALICE_PORT, ALICE_URL, BOB_PORT, BOB_URL, SPARE_PORT, SPARE_URL } from "./ports";

// B4 (a person who joins receives every file) and B5 (the file manager), with real instances.
// The index, its rules and the tree are covered by unit tests; here are the things that need the
// real app: Go writing the folder, the blobs crossing the network, the keys and the pointer.

const docsRoot = process.env.INKWELL_E2E_DOCS ?? "";
const tag = () => Date.now().toString(36);

/** The project folder of an instance: the one that holds .inkwell. */
function projectDir(port: number, root = join(docsRoot, String(port))): string {
  const dirs = readdirSync(root).filter((d) => existsSync(join(root, d, ".inkwell")));
  expect(dirs, `one project folder under ${root}`).toHaveLength(1);
  return join(root, dirs[0] as string);
}

/** Every file under `dir`, relative, with the sha-256 of its content. */
function hashes(dir: string): Record<string, string> {
  const out: Record<string, string> = {};
  const walk = (d: string) => {
    for (const name of readdirSync(d)) {
      if (name === ".inkwell" || name.endsWith(".tmp")) continue;
      const full = join(d, name);
      if (statSync(full).isDirectory()) walk(full);
      else out[relative(dir, full)] = createHash("sha256").update(readFileSync(full)).digest("hex");
    }
  };
  walk(dir);
  return out;
}

const tree = (p: Page) => p.getByRole("tree", { name: "Arborescence du projet" });
const item = (p: Page, path: string) => p.locator(`[role="treeitem"][data-path="${path}"]`);

/** Creates a file by the menu and the dialog; the new file is shown in the editor. */
async function newFile(p: Page, name: string, where?: string) {
  if (where) {
    await item(p, where).click({ button: "right" });
    await p.getByRole("menuitem", { name: "Nouveau fichier ici" }).click();
  } else {
    await p.getByRole("button", { name: "Ajouter au projet" }).click();
    await p.getByRole("menuitem", { name: "Nouveau fichier" }).click();
  }
  const dialog = p.getByRole("dialog", { name: "Nouveau fichier" });
  await dialog.getByLabel("Nom du fichier").fill(name);
  await dialog.getByRole("button", { name: "Créer" }).click();
  await expect(dialog).toBeHidden();
}

async function newFolder(p: Page, name: string) {
  await p.getByRole("button", { name: "Ajouter au projet" }).click();
  await p.getByRole("menuitem", { name: "Nouveau dossier" }).click();
  const field = p.locator("input.rename");
  await expect(field).toBeFocused();
  await field.fill(name);
  await field.press("Enter");
  await expect(item(p, name)).toBeVisible();
}

const seen: string[] = [];

/** Alice and `other` end up in the same project. `other` joins with Alice's invitation. */
async function joinWith(alice: Page, other: Page, name: string) {
  if (!(await alice.getByRole("dialog").isVisible())) {
    await alice.getByRole("button", { name: "Partager" }).click();
  }
  const invitation = alice.getByLabel("Invitation", { exact: true });
  // The dialog still shows the invitation Bob used: wait for a new one.
  const stale = (await invitation.count()) > 0 ? await invitation.inputValue() : "";
  await expect(async () => {
    await alice.getByRole("button", { name: "Inviter" }).click();
    await expect(invitation).toHaveValue(/^inkwell1:/, { timeout: 1_000 });
    expect(await invitation.inputValue()).not.toBe(stale);
  }).toPass({ timeout: 60_000 });
  const text = await invitation.inputValue();
  await alice.getByRole("button", { name: "Fermer" }).click();

  await other.getByRole("button", { name: "Partager" }).click();
  await rename(other, name);
  await other.getByPlaceholder("Collez l'invitation ici").fill(text);
  other.on("console", (m) => seen.push(`${m.type()}: ${m.text().slice(0, 200)}`));
  await other.getByRole("button", { name: "Rejoindre" }).click();
  await expect(other.getByRole("dialog").getByRole("alert"), seen.join("\n")).toBeHidden({
    timeout: 20_000,
  });
  try {
    await expect(other.getByRole("status").first()).toHaveText(/Synchronisé/, { timeout: 60_000 });
  } catch (err) {
    const why = await other
      .getByRole("dialog")
      .innerText()
      .catch(() => "no dialog");
    throw new Error(`${String(err)}\n${why}`);
  }
  // The dialog closes by itself once the project is open.
  await expect(other.getByRole("dialog")).toBeHidden({ timeout: 60_000 });
}

/** Starts another inkwell-server on its own folders and waits until it answers. */
async function startInstance(port: number): Promise<{ child: ChildProcess; docs: string }> {
  const base = mkdtempSync(join(tmpdir(), "inkwell-files-"));
  const docs = join(base, "docs");
  mkdirSync(docs);
  const log = openSync(join(base, "server.log"), "w");
  const child = spawn(resolve(import.meta.dirname, "../../bin/inkwell-server"), [], {
    env: {
      ...process.env,
      INKWELL_SERVER_PORT: String(port),
      INKWELL_RENDEZVOUS: "off",
      INKWELL_HOME: join(base, "home"),
      INKWELL_DOCS: docs,
    },
    stdio: ["ignore", log, log],
  });
  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    try {
      if ((await fetch(`http://127.0.0.1:${port}/health`)).ok) return { child, docs };
    } catch {
      // not listening yet
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  child.kill("SIGKILL");
  throw new Error(`instance on port ${port} did not start`);
}

async function stopInstance(child: ChildProcess) {
  if (child.exitCode !== null) return;
  const exited = new Promise((r) => child.once("exit", r));
  child.kill("SIGTERM");
  await exited;
}

/**
 * The shared instances keep their project between specs, and the reference screenshots of the
 * shell show it: this spec puts it back as it found it, in the app and on disk.
 */
async function tidy(browser: Browser) {
  const alice = await open(browser, ALICE_URL);
  const rows = tree(alice).locator('[role="treeitem"]');
  // The tree fills a moment after the editor shows (the documents load one by one).
  await alice.waitForTimeout(2_500);
  // The list draws only what is in view: top-level files below a long folder wait for a second turn.
  for (let turn = 0; turn < 5; turn++) {
    const doomed = await rows.evaluateAll((els) =>
      els
        .map((e) => e as HTMLElement)
        .filter((e) => e.getAttribute("aria-level") === "1" && e.dataset.path !== "main.typ")
        .map((e) => e.dataset.path as string),
    );
    if (doomed.length === 0) break;
    await item(alice, doomed[0] as string).click();
    for (const path of doomed.slice(1)) {
      await item(alice, path).click({ modifiers: ["ControlOrMeta"] });
    }
    await item(alice, doomed[0] as string).press("Delete");
    await alice.waitForTimeout(500);
  }
  const trash = alice.getByRole("button", { name: /la corbeille \(\d+\)/ });
  if (await trash.isVisible()) {
    await trash.click();
    await alice.getByRole("button", { name: "Vider la corbeille" }).first().click();
    await alice.getByRole("dialog").getByRole("button", { name: "Vider la corbeille" }).click();
  }
  await expect(rows).toHaveCount(1);
  for (const port of [ALICE_PORT, BOB_PORT]) {
    const dir = projectDir(port);
    for (const name of readdirSync(dir)) {
      if (name !== ".inkwell" && name !== "main.typ")
        rmSync(join(dir, name), { recursive: true, force: true });
    }
  }
  await alice.context().close();
}

test.describe.configure({ mode: "serial" });
test.afterAll(async ({ browser }) => {
  await tidy(browser);
});

test("FM-01 PRJ-04 PRJ-05 FM-03 a file is made, seen by the other person, renamed under their hands", async ({
  browser,
}) => {
  const alice = await open(browser, ALICE_URL);
  const bob = await open(browser, BOB_URL, true);
  await pair(alice, bob);
  const id = tag();

  await newFolder(alice, `chapitres-${id}`);
  await newFile(alice, `notes-${id}.md`, `chapitres-${id}`);
  const path = `chapitres-${id}/notes-${id}.md`;

  // Everyone has it, with the Markdown icon; the author has it open, with the template text.
  await expect(item(bob, path)).toBeVisible({ timeout: 30_000 });
  await expect(item(bob, path).locator("svg")).toHaveAttribute("aria-label", "Markdown");
  await expect(item(alice, path)).toHaveAttribute("aria-current", "page");
  await expect.poll(() => docText(alice)).toBe("# Titre");

  // It is a real file in both folders.
  const disk = (port: number) => join(projectDir(port), path);
  await expect.poll(() => existsSync(disk(ALICE_PORT)), { timeout: 15_000 }).toBe(true);
  await expect.poll(() => existsSync(disk(BOB_PORT)), { timeout: 30_000 }).toBe(true);

  // Bob opens it and types; Alice renames it meanwhile: Bob's typing is not lost.
  await item(bob, path).click();
  await editor(bob).click();
  await bob.keyboard.press("ControlOrMeta+End");
  await bob.keyboard.type(" du texte de Bob");
  await item(alice, path).focus();
  await alice.keyboard.press("F2");
  const field = alice.locator("input.rename");
  await field.fill("a/b");
  await field.press("Enter");
  await expect(alice.getByRole("alert")).toContainText("caractère interdit");
  await expect(item(alice, path)).toBeVisible(); // nothing changed
  await field.fill(`intro-${id}.md`);
  await field.press("Enter");
  const renamed = `chapitres-${id}/intro-${id}.md`;
  await expect(item(bob, renamed)).toBeVisible({ timeout: 30_000 });
  await bob.keyboard.type(" et la suite");
  await item(alice, renamed).click();
  await expect
    .poll(() => docText(alice), { timeout: 15_000 })
    .toContain("du texte de Bob et la suite");
  // The folder follows: the same file moved, not written twice.
  await expect
    .poll(() => existsSync(join(projectDir(ALICE_PORT), renamed)), { timeout: 15_000 })
    .toBe(true);
  expect(existsSync(join(projectDir(ALICE_PORT), path))).toBe(false);
});

test("FM-05 FM-06 FM-07 FM-09 a writer asks, an admin answers, the trash keeps the file", async ({
  browser,
}) => {
  const alice = await open(browser, ALICE_URL);
  const bob = await open(browser, BOB_URL, true);
  await pair(alice, bob);
  const id = tag();
  const name = `a-supprimer-${id}.typ`;
  await newFile(alice, name);
  await expect(item(bob, name)).toBeVisible({ timeout: 30_000 });

  // Bob deletes: the file stays, struck through, and Alice is asked.
  await item(bob, name).click();
  await item(bob, name).press("Delete");
  await expect(item(bob, name)).toContainText("suppression demandée");
  await expect(item(bob, name)).toBeVisible();
  await expect(alice.getByText(`demande à supprimer ${name}`)).toBeVisible({ timeout: 30_000 });
  await expect(bob.getByRole("button", { name: "Accepter" })).toHaveCount(0);

  // Alice refuses: the request goes, the refusal shows on the file.
  await alice.getByRole("button", { name: "Refuser" }).click();
  await expect(item(bob, name)).toContainText("Suppression refusée par", { timeout: 30_000 });
  await expect(alice.getByText(`demande à supprimer ${name}`)).toBeHidden();

  // Bob asks again while Alice is editing the file: she accepts, and the file goes to the trash.
  await item(bob, name).press("Delete");
  await alice.getByRole("button", { name: "Accepter" }).click();
  await expect(item(alice, name)).toBeHidden();
  await expect(item(bob, name)).toBeHidden({ timeout: 30_000 });
  const showTrash = (p: Page) => p.getByRole("button", { name: /la corbeille \(\d+\)/ });
  await showTrash(alice).click();
  await expect(alice.getByRole("region", { name: "Corbeille" })).toContainText(name);
  await alice.getByRole("button", { name: `Restaurer ${name}` }).click();
  await expect(item(alice, name)).toBeVisible();
  await expect(item(bob, name)).toBeVisible({ timeout: 30_000 });

  // An admin deletes outright a file the other has open: the banner says who, and nothing is lost.
  await item(bob, name).click();
  await editor(bob).click();
  await item(alice, name).click();
  await item(alice, name).press("Delete");
  await expect(bob.getByText(`${name} a été mis à la corbeille par`)).toBeVisible({
    timeout: 30_000,
  });
  await bob.keyboard.type("encore");
  await expect(bob.getByRole("button", { name: "Restaurer" })).toHaveCount(0); // a writer does not restore
  await showTrash(alice).click();
  await alice.getByRole("button", { name: `Restaurer ${name}` }).click();
  await expect(bob.getByText(`${name} a été mis à la corbeille par`)).toBeHidden({
    timeout: 30_000,
  });

  // Emptying the trash names the files and keeps the history.
  await item(alice, name).press("Delete");
  await showTrash(alice).click();
  await alice.getByRole("button", { name: "Vider la corbeille" }).first().click();
  const dialog = alice.getByRole("dialog", { name: "Vider la corbeille ?" });
  await expect(dialog).toContainText(name);
  await dialog.getByRole("button", { name: "Vider la corbeille" }).click();
  await expect(alice.getByRole("region", { name: "Corbeille" })).toBeHidden();
});

test("FM-02 FM-21 PRJ-13 import by drop, by name search, and files that are in the folder", async ({
  browser,
}) => {
  const alice = await open(browser, ALICE_URL);
  const bob = await open(browser, BOB_URL, true);
  await pair(alice, bob);
  const id = tag();

  // A drop of a text and a binary file: the first is a document, the second a blob that Bob gets.
  const png = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0, 1, 2, 3, 4]);
  const dt = await alice.evaluateHandle(
    ([name, bytes]) => {
      const t = new DataTransfer();
      t.items.add(new File(["= Déposé\n"], `depose-${name}.typ`));
      t.items.add(new File([new Uint8Array(bytes as number[])], `image-${name}.png`));
      return t;
    },
    [id, [...png]] as const,
  );
  await tree(alice).dispatchEvent("drop", { dataTransfer: dt });
  await expect(item(alice, `depose-${id}.typ`)).toBeVisible();
  await expect(item(alice, `image-${id}.png`).locator("svg")).toHaveAttribute(
    "aria-label",
    "Image",
  );
  await expect(item(bob, `image-${id}.png`)).toBeVisible({ timeout: 30_000 });
  await expect
    .poll(() => existsSync(join(projectDir(BOB_PORT), `image-${id}.png`)), { timeout: 30_000 })
    .toBe(true);
  expect(readFileSync(join(projectDir(BOB_PORT), `image-${id}.png`)).equals(png)).toBe(true);

  // Ctrl+P: open by name, from the keyboard.
  await editor(alice).click();
  await alice.keyboard.press("ControlOrMeta+p");
  const finder = alice.getByRole("dialog", { name: "Aller au fichier" });
  await finder.getByRole("combobox").fill(`depose-${id}`);
  await alice.keyboard.press("Enter");
  await expect(item(alice, `depose-${id}.typ`)).toHaveAttribute("aria-current", "page");
  await expect.poll(() => docText(alice)).toBe("= Déposé");

  // A file put in the folder by another program is shown apart, and joins only when asked.
  const scan = join(projectDir(ALICE_PORT), `scan-${id}.png`);
  writeFileSync(scan, Buffer.from([1, 2, 3]));
  const apart = alice.getByRole("region", { name: "Dans le dossier, pas dans le projet" });
  await expect(apart).toContainText(`scan-${id}.png`, { timeout: 15_000 });
  await expect(item(alice, `scan-${id}.png`)).toHaveCount(0);
  // (A file purged from the trash stays in the folder too: Go never deletes, so it is listed.)
  await apart
    .getByRole("listitem")
    .filter({ hasText: `scan-${id}.png` })
    .getByRole("button", { name: "Ajouter au projet" })
    .click();
  await expect(item(alice, `scan-${id}.png`)).toBeVisible();
  await expect(item(bob, `scan-${id}.png`)).toBeVisible({ timeout: 30_000 });

  // The image is replaced on disk: the same file, a new content, sent to Bob.
  const next = Buffer.from([9, 9, 9, 9, 9]);
  rmSync(scan);
  writeFileSync(scan, next);
  await item(alice, `scan-${id}.png`).click({ button: "right" });
  await alice.getByRole("menuitem", { name: "Mettre à jour dans le projet" }).click();
  const bobCopy = join(projectDir(BOB_PORT), `scan-${id}.png`);
  await expect
    .poll(() => (existsSync(bobCopy) ? readFileSync(bobCopy).equals(next) : false), {
      timeout: 30_000,
    })
    .toBe(true);

  // Accessibility of the list with its menu open.
  await item(alice, `scan-${id}.png`).click({ button: "right" });
  const results = await new AxeBuilder({ page: alice })
    .include('[aria-label="Fichiers du projet"]')
    .include('[role="menu"]')
    .withTags(["wcag2a", "wcag2aa"])
    .analyze();
  expect(results.violations.map((v) => `${v.id}: ${v.nodes[0]?.html}`)).toEqual([]);
  await alice.keyboard.press("Escape");
});

/** 40 files of every kind in a folder, as a real project would be. */
function makeProject(): { dir: string; root: string } {
  const root = mkdtempSync(join(tmpdir(), "inkwell-prj01-"));
  const dir = join(root, "projet-test");
  const put = (rel: string, data: string | Buffer) => {
    mkdirSync(join(dir, rel, ".."), { recursive: true });
    writeFileSync(join(dir, rel), data);
  };
  const noise = (n: number, seed: number) =>
    Buffer.from(Array.from({ length: n }, (_, i) => (i * 31 + seed) & 0xff));
  for (let i = 1; i <= 12; i++)
    put(
      `rapport/ch${String(i).padStart(2, "0")}.typ`,
      `= Chapitre ${i}\n\nTexte du chapitre ${i}.\n`,
    );
  for (let i = 1; i <= 3; i++) put(`rapport/notes/n${i}.md`, `# Note ${i}\n`);
  put("data/table.csv", "a,b\n1,2\n");
  put("data/biblio.bib", "@book{x, title={T}}\n");
  put("data/config.json", '{"k": 1}\n');
  for (let i = 1; i <= 10; i++)
    put(
      `figures/fig${String(i).padStart(2, "0")}.png`,
      Buffer.concat([Buffer.from([0x89, 0x50, 0x4e, 0x47, 0]), noise(2000 * i, i)]),
    );
  for (let i = 1; i <= 2; i++)
    put(`docs/doc${i}.pdf`, Buffer.concat([Buffer.from("%PDF-1.7\n\0"), noise(5000, 40 + i)]));
  for (let i = 1; i <= 2; i++)
    put(`fonts/Test-${i}.ttf`, Buffer.concat([Buffer.from([0, 1, 0, 0, 0]), noise(30000, 70 + i)]));
  for (let i = 1; i <= 8; i++) put(`annexes/a${i}.typ`, `= Annexe ${i}\n`);
  return { dir, root };
}

test("PRJ-01 X-17 X-19 a person who joins receives the 40 files, byte for byte, twice in a row", async ({
  browser,
}) => {
  test.setTimeout(240_000);
  const { dir, root } = makeProject();
  try {
    const alice = await open(browser, ALICE_URL);
    const bob = await open(browser, BOB_URL, true);
    await pair(alice, bob);

    // Alice brings the whole folder.
    const before = hashes(dir);
    expect(Object.keys(before)).toHaveLength(40);
    await alice.getByTestId("folder-picker").setInputFiles(dir);
    await expect(alice.getByText(/40 fichier\(s\) ajouté\(s\)/)).toBeVisible({ timeout: 60_000 });
    const onlyProject = (all: Record<string, string>) =>
      Object.fromEntries(Object.entries(all).filter(([k]) => k.startsWith("projet-test")));
    const expected = Object.fromEntries(
      Object.entries(before).map(([k, v]) => [`projet-test/${k}`, v]),
    );
    await expect
      .poll(() => onlyProject(hashes(projectDir(ALICE_PORT))), { timeout: 60_000 })
      .toEqual(expected);

    // C: a fresh instance, with nothing, joins through Alice (a real join).
    for (const [via, viaPage, label] of [
      ["alice", alice, "Charlie"],
      ["alice again, with Bob there", alice, "Dora"],
    ] as const) {
      const spare = await startInstance(SPARE_PORT);
      try {
        const c = await open(browser, SPARE_URL);
        await joinWith(viaPage, c, label);
        // The 40 files are in C's folder, the same paths and the same bytes; nothing half
        // written is ever visible there (temporary files are skipped by `hashes`, and a verified
        // file is put in place whole).
        // (The chapter the first visitor edited has their sentence in it by now.)
        const chapter = "projet-test/rapport/ch01.typ";
        const wanted = { ...expected };
        delete wanted[chapter];
        await expect
          .poll(
            () => {
              const got = onlyProject(hashes(projectDir(SPARE_PORT, spare.docs)));
              delete got[chapter];
              return got;
            },
            {
              timeout: 120_000,
              message: `files received through ${via}\n${[...new Set(seen.map((l) => l.replace(/doc \w+/, "doc")))].slice(0, 20).join("\n")}`,
            },
          )
          .toEqual(wanted);
        await expect
          .poll(() => existsSync(join(projectDir(SPARE_PORT, spare.docs), chapter)), {
            timeout: 60_000,
          })
          .toBe(true);
        await expect(c.locator(".progress progress")).toHaveCount(0, { timeout: 60_000 });
        // And it is a working project: C opens a received chapter by name, types, the others see it.
        await c.keyboard.press("ControlOrMeta+p");
        await c.getByRole("combobox").fill("ch01");
        await c.keyboard.press("Enter");
        await expect.poll(() => docText(c)).toContain("Chapitre 1");
        await editor(c).click();
        await c.keyboard.press("ControlOrMeta+End");
        await c.keyboard.type(` vu par ${label}`);
        await alice.keyboard.press("ControlOrMeta+p");
        await alice.getByRole("combobox").fill("ch01");
        await alice.keyboard.press("Enter");
        await expect.poll(() => docText(alice), { timeout: 30_000 }).toContain(`vu par ${label}`);
        await c.context().close();
      } finally {
        await stopInstance(spare.child);
        if (process.env.KEEP_SPARE) console.log(`spare kept in ${join(spare.docs, "..")}`);
        else rmSync(join(spare.docs, ".."), { recursive: true, force: true });
      }
    }
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
