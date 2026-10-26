import { afterEach, describe, expect, it, vi } from "vitest";
import { FakeGoStore, fakeHash } from "../store/fakeStore";
import { GoStorageAdapter, TooLargeError } from "../store/GoStorageAdapter";
import { NodeType } from "../store/tree";
import { materialize } from "./materialize";
import * as ops from "./ops";
import { TreeSync, toTreeEntries } from "./sync";
import { alice, apply, ctx, HASH, newProject, textUrl, view } from "./testkit";

afterEach(() => vi.useRealTimers());

function project() {
  const doc = newProject();
  return apply(
    doc,
    ops.create(doc, view(doc), ctx(alice), [
      { type: "dir", name: "figs", parent: null },
      { type: "blob", name: "a.png", parent: { spec: 0 }, hash: HASH, size: 7 },
      { type: "text", name: "t.typ", parent: { spec: 0 }, url: textUrl() },
    ]),
  );
}

describe("what Go is told", () => {
  it("maps the materialized tree to Tree entries: folders, texts with their url, blobs with hash and size", () => {
    const entries = toTreeEntries(view(project()));
    expect(entries.map((e) => [e.path, e.type, e.main, e.size])).toEqual([
      ["figs", NodeType.dir, false, 0],
      ["figs/a.png", NodeType.blob, false, 7],
      ["figs/t.typ", NodeType.text, false, 0],
      ["main.typ", NodeType.text, true, 0],
    ]);
    expect(entries[1]?.hash).toHaveLength(32);
    expect(entries[2]?.url).toMatch(/^automerge:/);
  });

  it("PRJ-19 a burst of changes is one Tree and one Referenced request, and none when nothing changed", async () => {
    vi.useFakeTimers();
    const tree = vi.fn(async (_: unknown[]) => 0);
    const refs = vi.fn(async (_: Uint8Array[]) => {});
    const sync = new TreeSync({ setTree: tree, setReferenced: refs }, { delayMs: 50 });
    const m = view(project());
    for (let i = 0; i < 50; i++) sync.update(m);
    await vi.advanceTimersByTimeAsync(60);
    expect(tree).toHaveBeenCalledTimes(1);
    expect(refs).toHaveBeenCalledTimes(1);
    expect(refs.mock.calls[0]?.[0]).toHaveLength(1);

    sync.update(view(project()));
    await vi.advanceTimersByTimeAsync(60);
    // Another project with other urls: sent. The same tree again: not.
    const calls = tree.mock.calls.length;
    sync.update(m);
    sync.update(m);
    await vi.advanceTimersByTimeAsync(60);
    expect(tree.mock.calls.length).toBeLessThanOrEqual(calls + 1);
    const settled = tree.mock.calls.length;
    sync.update(m);
    await vi.advanceTimersByTimeAsync(60);
    expect(tree.mock.calls.length).toBe(settled);
    sync.stop();
  });

  it("tells the caller how many entries Go refused, and tries again after a failure", async () => {
    vi.useFakeTimers();
    const results: number[] = [];
    const errors: unknown[] = [];
    let fail = true;
    const sync = new TreeSync(
      {
        setTree: async () => {
          if (fail) throw new Error("store closed");
          return 2;
        },
        setReferenced: async () => {},
      },
      { delayMs: 10, onResult: (n) => results.push(n), onError: (e) => errors.push(e) },
    );
    const m = view(project());
    sync.update(m);
    await vi.advanceTimersByTimeAsync(20);
    expect(errors).toHaveLength(1);
    fail = false;
    sync.update(m);
    await vi.advanceTimersByTimeAsync(20);
    expect(results).toEqual([2]);
    sync.stop();
  });

  it("an empty project is a valid, small tree", () => {
    expect(toTreeEntries(materialize(undefined))).toEqual([]);
  });
});

describe("the store stream's new operations, through the adapter", () => {
  it("Tree, Referenced and PutBlob round-trip with the fake Go side", async () => {
    const store = new FakeGoStore();
    const adapter = new GoStorageAdapter(() => store.socket());
    store.rejectOnTree = 3;
    expect(await adapter.setTree(toTreeEntries(view(project())))).toBe(3);
    expect(store.tree.map((e) => e.path)).toEqual(["figs", "figs/a.png", "figs/t.typ", "main.typ"]);
    await adapter.setReferenced([fakeHash(Uint8Array.of(1))]);
    expect(store.referenced).toHaveLength(1);

    store.disk.set("figs/a.png", Uint8Array.of(1, 2, 3));
    const info = await adapter.putBlob("figs/a.png");
    expect(info.size).toBe(3);
    expect(info.hash).toEqual(fakeHash(Uint8Array.of(1, 2, 3)));
    // Bytes brought from the webview are written first, and replace nothing.
    const fresh = await adapter.putBlob("b.bin", Uint8Array.of(9, 9));
    expect(fresh.size).toBe(2);
    await expect(adapter.putBlob("b.bin", Uint8Array.of(1))).rejects.toThrow(/file exists/);
    store.blobLimit = 4;
    store.disk.set("big.bin", new Uint8Array(10));
    await expect(adapter.putBlob("big.bin")).rejects.toMatchObject({ limit: 4 });
    await expect(adapter.putBlob("big.bin")).rejects.toBeInstanceOf(TooLargeError);
  });

  it("files in folders are addressed by path, one component per folder", async () => {
    const store = new FakeGoStore();
    const adapter = new GoStorageAdapter(() => store.socket());
    await adapter.writeProjection("chapitres/ch1.typ", "= Un\n");
    expect(store.files.has("chapitres/ch1.typ")).toBe(true);
    expect(await adapter.readProjection("chapitres/ch1.typ")).toBe("= Un\n");
    // A hostile path never leaves the webview.
    await expect(adapter.writeProjection("../x", "")).rejects.toThrow();
    await expect(adapter.writeProjection("a//b", "")).rejects.toThrow();
  });
});
