import { afterEach, describe, expect, it, vi } from "vitest";
import { Session } from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { FileKind } from "../store/files";
import { notice } from "../ui/notice.svelte";
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

describe("openFile", () => {
  it("refreshes the file info when going back to the main document", async () => {
    const refresh = vi.spyOn(workspace, "refreshInfo").mockResolvedValue();
    const fetch = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(null));
    workspace.files = [
      { path: "main.typ", kind: FileKind.typst, size: 0, main: true },
      { path: "notes.bin", kind: FileKind.other, size: 1, main: false },
    ];
    await workspace.openFile("notes.bin");
    refresh.mockClear();
    await workspace.openFile("main.typ");
    expect(workspace.current).toBe("main.typ");
    expect(refresh).toHaveBeenCalledTimes(1);
    refresh.mockRestore();
    fetch.mockRestore();
    workspace.files = [];
  });
});

describe("an open file changed outside Inkwell", () => {
  it("reloads on a new version, and goes back to main.typ with a toast once gone", async () => {
    vi.useFakeTimers();
    const disk = { version: '"1"', text: "one", gone: false };
    const fetch = vi.spyOn(globalThis, "fetch").mockImplementation(async (_url, init) => {
      if (disk.gone) return new Response(null, { status: 404 });
      if (init?.method === "HEAD")
        return new Response(null, { status: 200, headers: { ETag: disk.version } });
      return new Response(disk.text, { status: 200 });
    });
    try {
      workspace.files = [
        { path: "main.typ", kind: FileKind.typst, size: 0, main: true },
        { path: "notes.md", kind: FileKind.markdown, size: 3, main: false },
      ];
      await workspace.openFile("notes.md");
      expect(workspace.opened?.text).toBe("one");
      await vi.advanceTimersByTimeAsync(2500);
      expect(workspace.opened?.text).toBe("one"); // same version: not read again
      Object.assign(disk, { version: '"2"', text: "two" });
      await vi.advanceTimersByTimeAsync(2500);
      expect(workspace.opened?.text).toBe("two");
      disk.gone = true;
      await vi.advanceTimersByTimeAsync(2500);
      expect(workspace.current).toBe("main.typ");
      expect(notice.text).toContain("notes.md");
      const calls = fetch.mock.calls.length;
      await vi.advanceTimersByTimeAsync(5000);
      expect(fetch.mock.calls.length).toBe(calls); // main.typ is not polled
    } finally {
      fetch.mockRestore();
      notice.hide();
      vi.useRealTimers();
      workspace.files = [];
    }
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
  it("follows who is connected and what the indicator shows, never the peer count alone", () => {
    expect(statusOf("offline", 0, false)).toBe("alone");
    expect(statusOf("offline", 0, true)).toBe("connecting");
    // Someone is connected: the header says what the indicator shows, never "synced" by default.
    expect(statusOf("offline", 1, false)).toBe("checking");
    expect(statusOf("sending", 1, false)).toBe("sending");
    expect(statusOf("synced", 2, true)).toBe("synced");
  });

  it("cannot say 'alone' while a peer is listed, however the events arrived", () => {
    expect(workspace.status).toBe("alone");
    workspace.peers = ["bob"];
    expect(workspace.status).toBe("checking");
    // The adapter drops and re-announces a peer that starts over: the status follows the list.
    workspace.peers = [];
    expect(workspace.status).toBe("alone");
    workspace.joining = true;
    expect(workspace.status).toBe("connecting");
    workspace.peers = ["bob"];
    expect(workspace.status).toBe("checking");
    workspace.joining = false;
    expect(workspace.status).toBe("checking");
    workspace.peers = [];
    expect(workspace.status).toBe("alone");
  });
});
