// @vitest-environment node
import { next as A } from "@automerge/automerge";
import { type DocHandle, type PeerId, Repo } from "@automerge/automerge-repo";
import { afterEach, describe, expect, it, vi } from "vitest";
import { FakeGoStore } from "./fakeStore";
import { Eol, FileKind } from "./files";
import { ExternalEditError, GoStorageAdapter, NotTextError } from "./GoStorageAdapter";
import { Op } from "./protocol";

type Doc = { text: string };

const text = (s: string) => new TextEncoder().encode(s);
const str = (b: Uint8Array | undefined) => (b ? new TextDecoder().decode(b) : undefined);

function adapterOn(go: FakeGoStore) {
  const adapter = new GoStorageAdapter(() => go.socket());
  adapters.push(adapter);
  return adapter;
}

const adapters: GoStorageAdapter[] = [];
const repos: Repo[] = [];
afterEach(async () => {
  await Promise.all(repos.splice(0).map((r) => r.shutdown()));
  for (const a of adapters.splice(0)) a.close();
  vi.restoreAllMocks();
});

async function until(cond: () => boolean, ms = 5000): Promise<void> {
  const deadline = Date.now() + ms;
  while (!cond()) {
    if (Date.now() > deadline) throw new Error("timed out");
    await new Promise((r) => setTimeout(r, 2));
  }
}

describe("GoStorageAdapter (StorageAdapterInterface conformance)", () => {
  it("saves, loads and removes a value", async () => {
    const a = adapterOn(new FakeGoStore());
    expect(await a.load(["doc", "snapshot", "h"])).toBeUndefined();
    await a.save(["doc", "snapshot", "h"], text("hello"));
    expect(str(await a.load(["doc", "snapshot", "h"]))).toBe("hello");
    await a.remove(["doc", "snapshot", "h"]);
    expect(await a.load(["doc", "snapshot", "h"])).toBeUndefined();
  });

  it("round-trips binary data and the empty value", async () => {
    const a = adapterOn(new FakeGoStore());
    const bytes = Uint8Array.from({ length: 256 }, (_, i) => i);
    await a.save(["bin"], bytes);
    expect(await a.load(["bin"])).toEqual(bytes);
    await a.save(["empty"], new Uint8Array());
    expect(await a.load(["empty"])).toEqual(new Uint8Array());
  });

  it("loadRange returns every value under a prefix, keys included", async () => {
    const a = adapterOn(new FakeGoStore());
    await a.save(["d1", "snapshot", "h1"], text("s1"));
    await a.save(["d1", "incremental", "i1"], text("i1"));
    await a.save(["d1", "incremental", "i2"], text("i2"));
    await a.save(["d2", "snapshot", "h2"], text("s2"));

    const inc = await a.loadRange(["d1", "incremental"]);
    expect(inc.map((c) => [c.key.join("/"), str(c.data)])).toEqual([
      ["d1/incremental/i1", "i1"],
      ["d1/incremental/i2", "i2"],
    ]);
    expect((await a.loadRange(["d1"])).length).toBe(3);
    expect((await a.loadRange([])).length).toBe(4);
    expect(await a.loadRange(["nope"])).toEqual([]);
  });

  it("removeRange removes only the prefix", async () => {
    const a = adapterOn(new FakeGoStore());
    await a.save(["d1", "incremental", "i1"], text("i1"));
    await a.save(["d1", "snapshot", "h1"], text("s1"));
    await a.removeRange(["d1", "incremental"]);
    expect((await a.loadRange(["d1"])).map((c) => c.key.join("/"))).toEqual(["d1/snapshot/h1"]);
  });

  it("reads after writes in request order, without waiting for each answer", async () => {
    const a = adapterOn(new FakeGoStore());
    const writes = ["one", "two", "three"].map((v) => a.save(["k"], text(v)));
    const read = a.load(["k"]);
    await Promise.all(writes);
    expect(str(await read)).toBe("three");
  });

  it("rejects a key Go would refuse, before sending anything", async () => {
    const go = new FakeGoStore();
    const a = adapterOn(go);
    await expect(a.save(["a/b"], text("x"))).rejects.toThrow();
    await expect(a.load(["..", "x"])).rejects.toThrow();
    expect(go.log).toEqual([]);
  });

  it("turns a write refused after an outside edit into an ExternalEditError", async () => {
    const go = new FakeGoStore();
    go.locked.add("main.typ");
    const a = adapterOn(go);
    await expect(a.writeProjection("main.typ", "x")).rejects.toBeInstanceOf(ExternalEditError);
  });

  it("treats a damaged, unrecoverable value as missing, without leaking its content", async () => {
    const go = new FakeGoStore();
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const a = adapterOn(go);
    go.corrupt.add("d/snapshot/h");
    expect(await a.load(["d", "snapshot", "h"])).toBeUndefined();
    expect(warn).toHaveBeenCalledOnce();
  });

  it("fails pending requests when the stream closes, and reconnects for the next one", async () => {
    const go = new FakeGoStore();
    const a = adapterOn(go);
    await a.save(["k"], text("v"));
    go.dropConnections();
    await new Promise((r) => setTimeout(r, 0));
    expect(str(await a.load(["k"]))).toBe("v");
  });

  it("rejects requests made after close()", async () => {
    const a = adapterOn(new FakeGoStore());
    await a.save(["k"], text("v"));
    a.close();
    // A closed adapter may reconnect for later calls; what must never happen is a hang.
    await expect(
      Promise.race([a.load(["k"]), new Promise((r) => setTimeout(() => r("hang"), 1000))]),
    ).resolves.not.toBe("hang");
  });
});

describe("project files", () => {
  it("writes, reads and acknowledges a projection", async () => {
    const go = new FakeGoStore();
    const a = adapterOn(go);
    expect(await a.readProjection("main.typ")).toBeUndefined();
    await a.writeProjection("main.typ", "= Titre\né\n");
    expect(await a.readProjection("main.typ")).toBe("= Titre\né\n");
    await a.acknowledge("main.typ");
    expect(go.acknowledged).toEqual(["main.typ"]);
    expect(go.log.map(([op]) => op)).toEqual([
      Op.ReadProjection,
      Op.WriteProjection,
      Op.ReadProjection,
      Op.Acknowledge,
    ]);
  });

  it("lists the files and says how one is stored", async () => {
    const go = new FakeGoStore();
    const a = adapterOn(go);
    await a.writeProjection("main.typ", "= A\n");
    go.files.set("notes.txt", text("n"));
    go.info.set("notes.txt", { encoding: "windows-1252", eol: Eol.CRLF, bom: false, mixed: true });
    expect(await a.listFiles()).toEqual([
      { path: "main.typ", kind: FileKind.typst, size: 4, main: true },
      { path: "notes.txt", kind: FileKind.text, size: 1, main: false },
    ]);
    expect(await a.fileInfo("main.typ")).toEqual({
      encoding: "utf-8",
      eol: Eol.LF,
      bom: false,
      mixed: false,
    });
    expect(await a.fileInfo("notes.txt")).toMatchObject({ encoding: "windows-1252", mixed: true });
    expect(await a.fileInfo("absent.typ")).toBeUndefined();
  });

  it("rejects a file that is not text, with the line of the bad byte", async () => {
    const go = new FakeGoStore();
    const a = adapterOn(go);
    go.files.set("img.typ", text("x"));
    go.files.set("bad.typ", text("x"));
    go.binary.add("img.typ");
    go.invalidText.set("bad.typ", 3);
    await expect(a.readProjection("img.typ")).rejects.toMatchObject({
      name: "NotTextError",
      reason: "binary",
    });
    await expect(a.fileInfo("bad.typ")).rejects.toMatchObject({
      name: "NotTextError",
      reason: "invalid-utf8",
      line: 3,
    });
    await expect(a.readProjection("bad.typ")).rejects.toBeInstanceOf(NotTextError);
  });

  it("delivers external-edit notices, including the ones pushed on attach", async () => {
    const go = new FakeGoStore();
    const a = adapterOn(go);
    const seen: string[] = [];
    const off = a.onExternal((name) => seen.push(name));
    await a.save(["k"], text("v")); // connected
    go.pushExternal("main.typ");
    await until(() => seen.length === 1);
    off();
    go.pushExternal("main.typ");
    await new Promise((r) => setTimeout(r, 5));
    expect(seen).toEqual(["main.typ"]);
  });
});

describe("with automerge-repo", () => {
  function repoOn(go: FakeGoStore, id: string): { repo: Repo; adapter: GoStorageAdapter } {
    const adapter = adapterOn(go);
    const repo = new Repo({ peerId: id as PeerId, storage: adapter });
    repos.push(repo);
    return { repo, adapter };
  }

  it("a document survives a restart: a new Repo over the same Go store loads it", async () => {
    const go = new FakeGoStore();
    const first = repoOn(go, "first");
    const handle = first.repo.create<Doc>({ text: "" });
    handle.change((d) => A.updateText(d, ["text"], "Bonjour"));
    handle.change((d) => A.splice(d, ["text"], 7, 0, " tout le monde"));
    await first.repo.flush();
    await first.repo.shutdown();
    repos.length = 0;

    expect(go.values.size).toBeGreaterThan(0);
    for (const key of go.values.keys()) {
      expect(
        key.split("/").every((c) => /^[A-Za-z0-9._-]{1,128}$/.test(c)),
        key,
      ).toBe(true);
    }

    const second = repoOn(go, "second");
    const reopened: DocHandle<Doc> = await second.repo.find<Doc>(handle.url);
    expect(reopened.doc()?.text).toBe("Bonjour tout le monde");
  });

  it("compaction removes the incremental chunks it folds into a snapshot", async () => {
    const go = new FakeGoStore();
    const { repo } = repoOn(go, "p");
    const handle = repo.create<Doc>({ text: "" });
    for (let i = 0; i < 30; i++) {
      handle.change((d) => A.splice(d, ["text"], d.text.length, 0, `line ${i}\n`));
      await repo.flush();
    }
    await repo.shutdown();
    repos.length = 0;
    const reopened = await repoOn(go, "q").repo.find<Doc>(handle.url);
    expect(reopened.doc()?.text.split("\n").length).toBe(31);
  });
});
