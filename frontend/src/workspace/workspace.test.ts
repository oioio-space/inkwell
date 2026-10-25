import { afterEach, describe, expect, it, vi } from "vitest";
import { statusOf, workspace } from "./workspace.svelte";

// The Wails runtime dials its host on import; the status logic never needs it.
vi.mock("@wailsio/runtime", () => ({ Stream: vi.fn() }));
vi.mock("../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  Projects: {},
  Session: {},
}));

afterEach(() => {
  workspace.peers = [];
  workspace.joining = false;
});

describe("status", () => {
  it("follows who is connected: any peer means in sync, whatever else is going on", () => {
    expect(statusOf(0, false)).toBe("alone");
    expect(statusOf(0, true)).toBe("connecting");
    expect(statusOf(1, false)).toBe("synced");
    expect(statusOf(2, true)).toBe("synced");
  });

  it("cannot say 'alone' while a peer is listed, however the events arrived", () => {
    expect(workspace.status).toBe("alone");
    workspace.peers = ["bob"];
    expect(workspace.status).toBe("synced");
    // The adapter drops and re-announces a peer that starts over: the status follows the list.
    workspace.peers = [];
    expect(workspace.status).toBe("alone");
    workspace.joining = true;
    expect(workspace.status).toBe("connecting");
    workspace.peers = ["bob"];
    expect(workspace.status).toBe("synced");
    workspace.joining = false;
    expect(workspace.status).toBe("synced");
    workspace.peers = [];
    expect(workspace.status).toBe("alone");
  });
});
