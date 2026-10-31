import type { AutomergeUrl, PeerId } from "@automerge/automerge-repo";
import { afterEach, expect, it, vi } from "vitest";
import { Session } from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { workspace } from "./workspace.svelte";

const state = vi.hoisted(() => ({
  listeners: new Set<(url: AutomergeUrl, peer: PeerId) => void>(),
}));
vi.mock("@wailsio/runtime", () => ({ Stream: vi.fn() }));
vi.mock("../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  Session: { LocalPeer: async () => "me", Join: async () => "host" },
  Projects: { Current: async () => ({ RootURL: "" }) },
}));
vi.mock("@automerge/automerge-repo", async (original) => ({
  ...(await original<typeof import("@automerge/automerge-repo")>()),
  Repo: class {
    create() {
      return { url: "root" };
    }
    async shutdown() {}
  },
}));
vi.mock("../net/WailsNetworkAdapter", () => ({
  WailsNetworkAdapter: class {
    on() {}
    onRoles() {}
    onSealed() {}
    disconnect() {}
    projectOf() {
      return undefined;
    }
    onProject(cb: (url: AutomergeUrl, peer: PeerId) => void) {
      state.listeners.add(cb);
      return () => state.listeners.delete(cb);
    }
  },
}));
vi.mock("../store/GoStorageAdapter", () => ({
  GoStorageAdapter: class {
    onExternal() {}
    close() {}
  },
}));
afterEach(async () => {
  await workspace.stop();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

it.each(["success", "timeout", "stop"])("removes project wait listeners on %s", async (outcome) => {
  vi.useFakeTimers();
  vi.spyOn(workspace, "open").mockResolvedValue();
  await workspace.start();
  const joined = workspace.join("invitation");
  await vi.waitFor(() => expect(state.listeners.size).toBe(1));
  expect(state.listeners.size).toBe(1);
  if (outcome === "success") {
    for (const cb of state.listeners) cb("root" as AutomergeUrl, "host" as PeerId);
  } else if (outcome === "timeout") {
    await vi.advanceTimersByTimeAsync(10_000);
  } else {
    await workspace.stop();
  }
  await joined;
  expect(state.listeners.size).toBe(0);
  expect(vi.getTimerCount()).toBe(0);
});

it("does not resume startup after teardown while LocalPeer is pending", async () => {
  let finish!: (peer: string) => void;
  vi.spyOn(Session, "LocalPeer").mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }) as ReturnType<typeof Session.LocalPeer>,
  );
  const open = vi.spyOn(workspace, "open").mockResolvedValue();
  const started = workspace.start();
  await workspace.stop();
  finish("me");
  await started;
  expect(open).not.toHaveBeenCalled();
  expect(workspace.repo).toBeUndefined();
  expect(state.listeners.size).toBe(0);
});

it("can stop and start again without retaining project waits", async () => {
  vi.useFakeTimers();
  vi.spyOn(workspace, "open").mockResolvedValue();
  for (let i = 0; i < 2; i++) {
    await workspace.start();
    const joined = workspace.join("invitation");
    await vi.waitFor(() => expect(state.listeners.size).toBe(1));
    await workspace.stop();
    await joined;
    expect(state.listeners.size).toBe(0);
    expect(vi.getTimerCount()).toBe(0);
  }
});
