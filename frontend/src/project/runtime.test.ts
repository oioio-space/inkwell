import { next as A } from "@automerge/automerge";
import { Repo } from "@automerge/automerge-repo";
import { afterEach, describe, expect, it } from "vitest";
import { until } from "../net/testkit";
import { FakeGoStore } from "../store/fakeStore";
import { GoStorageAdapter } from "../store/GoStorageAdapter";
import { Op } from "../store/protocol";
import { NodeType } from "../store/tree";
import type { IncomingFile } from "./import";
import { migration } from "./migrate";
import type { IndexDoc } from "./model";
import { type BlobStatus, ProjectRuntime } from "./runtime.svelte";

const text =
  (s: string): IncomingFile["read"] =>
  async () =>
    new TextEncoder().encode(s);
const incoming = (path: string, body: string | Uint8Array): IncomingFile => ({
  path,
  size: typeof body === "string" ? body.length : body.length,
  read: typeof body === "string" ? text(body) : async () => body,
});

const running: ProjectRuntime[] = [];

interface Options {
  role?: "admin" | "writer" | "reader";
  blobStatus?: () => Promise<BlobStatus>;
  legacy?: boolean;
}

async function setup(o: Options = {}) {
  const store = new FakeGoStore();
  store.strictOwnership = true;
  const storage = new GoStorageAdapter(() => store.socket());
  const repo = new Repo({});
  const root = repo.create<IndexDoc>({ text: "" });
  const rt = new ProjectRuntime(root, {
    repo,
    storage,
    me: () => ({ id: "alice", name: "Alice" }),
    ...(o.blobStatus ? { blobStatus: o.blobStatus } : {}),
  });
  running.push(rt);
  rt.role = o.role ?? "admin";
  rt.start();
  await until(() => store.treeCalls > 0, 3000, "no tree sent");
  return { store, storage, repo, root, rt };
}

afterEach(() => {
  for (const rt of running.splice(0)) rt.stop();
});

describe("the tree reaches Go, and files are written where Go knows them", () => {
  it("PRJ-01 the migrated project sends its tree; the main file's text is the workspace's to write", async () => {
    const { store, root, rt } = await setup();
    expect(store.tree.map((e) => [e.path, e.type, e.main])).toEqual([
      ["main.typ", NodeType.text, true],
    ]);
    expect(store.tree[0]?.url).toBe(root.url);
    expect(rt.tree.mainId).toBeDefined();
    root.change((d) => A.splice(d, ["text"], 0, 0, "Bonjour"));
    await new Promise((r) => setTimeout(r, 600));
    // The root document is projected by the workspace, with its sync point (B1b).
    expect(store.files.has("main.typ")).toBe(false);
    expect(store.log.some(([op]) => op === Op.Tree)).toBe(true);
  });

  it("FM-01 a new file in a folder is a document, listed to Go, projected with its text", async () => {
    const { store, rt } = await setup();
    const dir = rt.index.createFolder(null, "chapitres");
    expect(dir.ok).toBe(true);
    const made = rt.index.createFile(
      (dir as { ids: string[] }).ids[0] as string,
      "notes.md",
      "# Titre\n",
    );
    expect(made.ok).toBe(true);
    await until(() => store.files.has("chapitres/notes.md"), 3000, "notes.md not written");
    expect(new TextDecoder().decode(store.files.get("chapitres/notes.md"))).toBe("# Titre\n");
    expect(store.tree.map((e) => e.path)).toEqual(["chapitres", "chapitres/notes.md", "main.typ"]);
    expect(store.referencedCalls).toBeGreaterThan(0);
  });

  it("PRJ-05 renaming a file keeps its document: typing afterwards lands at the new path", async () => {
    const { store, rt } = await setup();
    const made = rt.index.createFile(null, "ch1.typ", "un");
    const id = (made as { ids: string[] }).ids[0] as string;
    await until(() => store.files.has("ch1.typ"), 3000);
    const handle = rt.handleOf(id);
    expect(handle).toBeDefined();
    expect(rt.index.rename(id, "intro.typ").ok).toBe(true);
    await until(() => store.tree.some((e) => e.path === "intro.typ"), 3000);
    expect(rt.handleOf(id)).toBe(handle);
    handle?.change((d) => A.splice(d, ["text"], 2, 0, " deux"));
    await until(
      () => new TextDecoder().decode(store.files.get("intro.typ")) === "un deux",
      3000,
      "no write",
    );
  });

  it("PRJ-19 a burst of changes to the index is a few Tree requests, not one per change", async () => {
    const { store, rt } = await setup();
    const before = store.treeCalls;
    for (let i = 0; i < 100; i++) rt.index.createFile(null, `f${i}.typ`);
    await until(() => store.tree.length === 101, 5000, "tree incomplete");
    expect(store.treeCalls - before).toBeLessThan(10);
  });

  it("FM-09 PRJ-06 a file trashed while open keeps its document and tells who trashed it", async () => {
    const { store, rt } = await setup();
    const id = (rt.index.createFile(null, "live.typ", "a") as { ids: string[] }).ids[0] as string;
    await until(() => store.files.has("live.typ"), 3000);
    rt.openFile(id);
    rt.index.remove([id]);
    expect(rt.currentId).toBe(id);
    expect(rt.trashedCurrent).toMatchObject({ name: "live.typ", by: "alice" });
    expect(rt.currentPath).toBe("live.typ");
    // Typing goes on in the document: nothing is lost, and the file is restorable.
    rt.handleOf(id)?.change((d) => A.splice(d, ["text"], 1, 0, "bc"));
    rt.index.restore([id]);
    await until(
      () => new TextDecoder().decode(store.files.get("live.typ")) === "abc",
      3000,
      "not written",
    );
    expect(rt.trashedCurrent).toBeUndefined();
  });

  it("FM-12 a purged file that was open is closed", async () => {
    const { rt } = await setup();
    const id = (rt.index.createFile(null, "x.typ") as { ids: string[] }).ids[0] as string;
    rt.openFile(id);
    rt.index.remove([id]);
    rt.index.purge([id]);
    expect(rt.openIds).not.toContain(id);
    expect(rt.currentId).toBe(rt.tree.mainId);
  });
});

describe("reception", () => {
  it("PRJ-02 counts complete documents and verified blobs: « 12 sur 40 »", async () => {
    let stored = 2;
    const { rt } = await setup({
      blobStatus: async () => ({
        Files: 4,
        Stored: stored,
        Bytes: 4000,
        StoredBytes: 2000,
        Limit: 2 ** 31,
        NeedsConfirmation: false,
      }),
    });
    for (let i = 0; i < 4; i++) {
      rt.index.createMany([
        {
          type: "blob",
          name: `f${i}.png`,
          parent: null,
          hash: `blake3:${String(i).repeat(64)}`,
          size: 1000,
        },
      ]);
    }
    await until(() => rt.ready >= 1 && rt.progress.total === 5, 3000, "not counted");
    await rt.refreshProgress();
    expect(rt.progress).toMatchObject({ done: 3, total: 5 }); // main.typ + 2 of the 4 blobs
    stored = 4;
    await rt.refreshProgress();
    expect(rt.progress).toMatchObject({ done: 5, total: 5 });
  });

  it("PRJ-11 a project over its quota waits for the person", async () => {
    const { rt } = await setup({
      blobStatus: async () => ({
        Files: 1,
        Stored: 0,
        Bytes: 3e9,
        StoredBytes: 0,
        Limit: 2 * 1024 ** 3,
        NeedsConfirmation: true,
        Refused: ["huge.bin"],
      }),
    });
    await rt.refreshProgress();
    expect(rt.progress.needsConfirmation).toBe(true);
    expect(rt.progress.refused).toEqual(["huge.bin"]);
  });
});

describe("files from outside", () => {
  it("PRJ-04 PRJ-13 a file in the folder joins the project only when asked; a replaced one gets a new hash", async () => {
    const { store, rt } = await setup();
    store.disk.set("figs/a.png", Uint8Array.of(1, 2, 3));
    await rt.refreshUntracked();
    expect(rt.untracked.map((f) => f.path)).toEqual(["figs/a.png"]);
    expect(rt.tree.byPath.has("figs/a.png")).toBe(false);

    const out = await rt.addFromDisk("figs/a.png");
    expect(out.ok).toBe(true);
    const blob = rt.tree.byPath.get("figs/a.png");
    expect(blob?.type).toBe("blob");
    await until(() => store.tree.some((e) => e.path === "figs/a.png"), 3000);
    expect(rt.untracked).toEqual([]);

    const before = blob?.hash;
    store.disk.set("figs/a.png", Uint8Array.of(9, 9, 9, 9));
    store.files.delete("figs/a.png");
    const updated = await rt.updateFromDisk(blob?.id as string);
    expect(updated.ok).toBe(true);
    expect(rt.tree.byPath.get("figs/a.png")?.hash).not.toBe(before);
    expect(rt.tree.byPath.get("figs/a.png")?.id).toBe(blob?.id);
  });

  it("FM-02 importing mixed files rebuilds the folders, makes documents of texts and blobs of the rest", async () => {
    const { store, rt } = await setup();
    const res = await rt.importFiles(
      [
        incoming("rapport/ch1.typ", "= Un\n"),
        incoming("rapport/notes/CON.txt", "x"),
        incoming("rapport/figs/a.png", Uint8Array.of(0x89, 0x50, 0x4e, 0x47, 0, 1)),
        incoming("rapport/plan.pdf", Uint8Array.of(0x25, 0x50, 0x44, 0x46, 0, 2)),
        incoming("main.typ", "= Autre\n"),
      ],
      null,
    );
    expect(res.outcome.ok).toBe(true);
    expect(res.plan.files).toBe(5);
    expect(res.plan.renamed).toContainEqual(["CON.txt", "_CON.txt"]);
    expect(res.plan.renamed).toContainEqual(["main.typ", "main (2).typ"]);
    const paths = rt.tree.entries.map((e) => e.path);
    expect(paths).toEqual([
      "main (2).typ",
      "main.typ",
      "rapport",
      "rapport/ch1.typ",
      "rapport/figs",
      "rapport/figs/a.png",
      "rapport/notes",
      "rapport/notes/_CON.txt",
      "rapport/plan.pdf",
    ]);
    expect(rt.tree.byPath.get("rapport/figs/a.png")?.type).toBe("blob");
    expect(rt.tree.byPath.get("rapport/ch1.typ")?.type).toBe("text");
    await until(() => store.files.has("rapport/ch1.typ"), 3000, "text not projected");
    expect(store.files.has("rapport/figs/a.png")).toBe(true);
  });

  it("PRJ-11 a file over 100 MB is reported, not added", async () => {
    const { rt } = await setup();
    const res = await rt.importFiles(
      [{ path: "big.bin", size: 101 * 1024 * 1024, read: async () => new Uint8Array() }],
      null,
    );
    expect(res.plan.skipped).toEqual([
      { path: "big.bin", reason: "too_large", limit: 100 * 1024 * 1024 },
    ]);
    expect(rt.tree.byPath.has("big.bin")).toBe(false);
  });
});

describe("PRJ-12 FM-13 a reader", () => {
  it("sees the project but changes nothing", async () => {
    const { rt } = await setup({ role: "reader" });
    expect(rt.index.createFile(null, "x.typ")).toMatchObject({
      ok: false,
      error: { code: "forbidden" },
    });
    expect(rt.index.rename(rt.tree.mainId as string, "y.typ")).toMatchObject({
      ok: false,
      error: { code: "forbidden" },
    });
    expect(rt.tree.entries.map((e) => e.path)).toEqual(["main.typ"]);
  });
});

describe("migration of a project that has only text", () => {
  it("keeps the text, and the main node points to the root document", async () => {
    const store = new FakeGoStore();
    const storage = new GoStorageAdapter(() => store.socket());
    const repo = new Repo({});
    const root = repo.create<IndexDoc>({ text: "ancien contenu" });
    expect(migration(root.doc() as IndexDoc, root.url, "alice", 0)).toBeDefined();
    const rt = new ProjectRuntime(root, {
      repo,
      storage,
      me: () => ({ id: "alice", name: "Alice" }),
    });
    running.push(rt);
    rt.start();
    expect(rt.tree.byPath.get("main.typ")?.url).toBe(root.url);
    expect(root.doc()?.text).toBe("ancien contenu");
    expect(migration(root.doc() as IndexDoc, root.url, "alice", 0)).toBeUndefined();
  });
});
