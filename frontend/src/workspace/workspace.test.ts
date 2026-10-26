import { afterEach, describe, expect, it, vi } from "vitest";
import { Session } from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
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

describe("names (UX-S7-3)", () => {
  it("keeps the name of a person who is no longer present", () => {
    workspace.hearNames([{ peerId: "paul", name: "Paul" }]);
    expect(workspace.nameOf("paul")).toBe("Paul");
    workspace.hearNames([]); // Paul went quiet: the list of people present no longer has him
    expect(workspace.names.paul).toBeUndefined();
    expect(workspace.nameOf("paul")).toBe("Paul");
    expect(workspace.knownName("nobody")).toBeUndefined();
    expect(workspace.nameOf("nobody")).toBe("Personne nobo");
  });
});

describe("invitation", () => {
  it("is marked used when a new person connects, and a fresh one replaces it", async () => {
    const invite = vi
      .fn()
      .mockResolvedValueOnce({ text: "first" })
      .mockResolvedValueOnce({ text: "second" });
    (Session as unknown as { Invite: typeof invite }).Invite = invite;
    workspace.peers = ["carol"];
    await workspace.host();
    expect(workspace.invitation).toBe("first");
    expect(workspace.invitationUsed).toBe(false);

    workspace.peerUp("carol"); // already there: not the person invited
    expect(workspace.invitationUsed).toBe(false);
    workspace.peerUp("bob");
    expect(workspace.invitationUsed).toBe(true);

    await workspace.host();
    expect(workspace.invitation).toBe("second");
    expect(workspace.invitationUsed).toBe(false);
    workspace.invitation = "";
  });
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
