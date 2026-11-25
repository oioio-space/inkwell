// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import fr from "../i18n/fr.json";
import { notice } from "../ui/notice.svelte";
import { watchSuperseded } from "../workspace/superseded";
import { FakeGoStore } from "./fakeStore";
import { GoStorageAdapter, StoreSupersededError } from "./GoStorageAdapter";
import { Op } from "./protocol";

/**
 * RT1, the store stream's half in the page (plan 2026-11-25-rt1-root-kept.md): a read cut by a stream close is sent
 * again, once, and a write never is (A-2); a page told it is superseded stays down and says so once (C-2).
 */

const text = (s: string) => new TextEncoder().encode(s);
const str = (b: Uint8Array | undefined) => (b ? new TextDecoder().decode(b) : undefined);
const tick = () => new Promise((r) => setTimeout(r, 0));

const adapters: GoStorageAdapter[] = [];
afterEach(() => {
  for (const a of adapters.splice(0)) a.close();
  vi.restoreAllMocks();
});

/** An adapter on `go`, counting the streams it opens. */
function adapterOn(go: FakeGoStore): { a: GoStorageAdapter; opened: () => number } {
  let n = 0;
  const a = new GoStorageAdapter(() => {
    n++;
    return go.socket();
  });
  adapters.push(a);
  return { a, opened: () => n };
}

/** The requests of `op` Go answered (a held one was read, never answered). */
const sent = (go: FakeGoStore, op: number) =>
  go.log.filter(([o, k]) => o === op && k !== "held").length;

describe("RT1 a cut stream", () => {
  it("RT1 A-2 a read cut by a stream close is sent again, once, on the next stream", async () => {
    const go = new FakeGoStore();
    const { a, opened } = adapterOn(go);
    await a.save(["d", "snapshot", "h"], text("kept"));
    go.held = true; // Go reads the request, then the stream ends before the answer
    const load = a.load(["d", "snapshot", "h"]);
    const range = a.loadRange(["d"]);
    await tick();
    go.held = false;
    go.dropConnections();
    expect(str(await load)).toBe("kept");
    expect((await range).map((c) => str(c.data))).toEqual(["kept"]);
    expect(opened()).toBe(2);
    // Each read went out twice: once held, once on the new stream; never a third time.
    expect(go.log.filter(([, k]) => k === "held").length).toBe(2);
    expect(sent(go, Op.Load)).toBe(1);
    expect(sent(go, Op.LoadRange)).toBe(1);

    // A read cut a second time is not sent a third time: it fails.
    go.held = true;
    const twice = a.load(["d", "snapshot", "h"]);
    await tick();
    go.dropConnections();
    await tick();
    go.dropConnections();
    await expect(twice).rejects.toThrow("store stream closed");
  });

  it("RT1 A-2 a write cut by a stream close is never sent again", async () => {
    const go = new FakeGoStore();
    const { a } = adapterOn(go);
    const lost = vi.fn();
    a.onLeaving(lost);
    await a.load(["warm"]);
    go.held = true;
    const save = a.save(["d", "snapshot", "h"], text("typed"));
    const remove = a.remove(["d", "snapshot", "old"]);
    await tick();
    go.held = false;
    go.dropConnections();
    await expect(save).rejects.toThrow();
    await expect(remove).rejects.toThrow();
    await a.load(["after"]); // a new stream is up, and nothing was sent again on it
    expect(sent(go, Op.Save)).toBe(0);
    expect(sent(go, Op.Remove)).toBe(0);
    expect(go.values.has("d/snapshot/h")).toBe(false);
    expect(lost).toHaveBeenCalled();
  });
});

describe("RT1 a superseded page", () => {
  it("RT1 C-2 a superseded adapter fails every request and never reconnects", async () => {
    const go = new FakeGoStore();
    const { a, opened } = adapterOn(go);
    await a.save(["k"], text("v"));
    go.held = true;
    const pending = a.load(["k"]);
    await tick();
    go.held = false;
    go.supersede();
    await expect(pending).rejects.toBeInstanceOf(StoreSupersededError);
    for (const later of [a.load(["k"]), a.listFiles(), a.loadRange([])]) {
      await expect(later).rejects.toBeInstanceOf(StoreSupersededError);
    }
    await expect(a.save(["k"], text("w"))).rejects.toThrow("store superseded");
    await tick();
    expect(opened()).toBe(1);
    expect(str(go.values.get("k"))).toBe("v");
  });

  it("RT1 C-2 a superseded page says once that Inkwell is open elsewhere", async () => {
    const go = new FakeGoStore();
    const { a } = adapterOn(go);
    const show = vi.spyOn(notice, "show");
    watchSuperseded(a);
    await a.load(["k"]);
    expect(show).not.toHaveBeenCalled();
    go.supersede();
    await tick();
    for (let i = 0; i < 3; i++) await a.listFiles().catch(() => {});
    expect(show.mock.calls.map(([text, role]) => [text, role])).toEqual([
      [fr["store.openElsewhere"], "alert"],
    ]);
  });
});
