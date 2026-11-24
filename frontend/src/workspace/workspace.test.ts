import { afterEach, describe, expect, it, vi } from "vitest";
import { Session } from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { notice } from "../ui/notice.svelte";
import { viewChoice } from "./viewChoice.svelte";
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
      { path: "main.typ", size: 0, main: true },
      { path: "notes.bin", size: 1, main: false },
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

describe("how a file opens (ADR 0027 rule 6)", () => {
  it("bytes Go does not take as text open as the file card; a blob type is not read as text", async () => {
    const fetch = vi.spyOn(globalThis, "fetch").mockImplementation(async (url, init) => {
      if (init?.method === "HEAD")
        return new Response(null, { status: 200, headers: { ETag: '"1"' } });
      return String(url).startsWith("/project-text")
        ? new Response(null, { status: 415 })
        : new Response("data:image/png;base64,", { status: 200 });
    });
    workspace.files = [
      { path: "main.typ", size: 0, main: true },
      { path: "blob.bin", size: 3, main: false },
      { path: "data.csv", size: 3, main: false },
      { path: "fig.png", size: 3, main: false },
    ];
    try {
      await workspace.openFile("blob.bin");
      expect([workspace.opened?.opening.type.id, workspace.opened?.opening.viewer.id]).toEqual([
        "binary",
        "files.card",
      ]);
      await workspace.openFile("data.csv");
      expect(workspace.opened?.opening.notice).toBe("not-text");
      await workspace.openFile("fig.png");
      expect(workspace.opened?.opening.viewer.id).toBe("images.view");
      expect(
        fetch.mock.calls.some(
          ([u, i]) =>
            String(u).includes("fig.png") &&
            !i?.method?.includes("HEAD") &&
            String(u).startsWith("/project-text"),
        ),
      ).toBe(false);
    } finally {
      fetch.mockRestore();
      await workspace.openFile("main.typ");
      workspace.files = [];
    }
  });
});

describe("what a file is, once read (EX0 code review, findings 1, 9 and 11)", () => {
  it("a text too big shows the info card; an unknown name read as text gets the text type", async () => {
    const fetch = vi.spyOn(globalThis, "fetch").mockImplementation(async (url, init) => {
      if (init?.method === "HEAD")
        return new Response(null, { status: 200, headers: { ETag: '"1"' } });
      if (String(url).includes("huge.log")) return new Response(null, { status: 413 });
      return new Response("Copyright", { status: 200 });
    });
    workspace.files = [
      { path: "main.typ", size: 0, main: true },
      { path: "huge.log", size: 3, main: false },
      { path: "LICENSE", size: 3, main: false },
    ];
    try {
      await workspace.openFile("huge.log");
      expect(workspace.opened?.failed).toBeUndefined();
      expect(workspace.opened?.large).toBe(true);
      expect(workspace.opened?.opening.viewer.id).toBe("files.card");
      expect(workspace.fileTypeOf("LICENSE")).toBeUndefined();
      await workspace.openFile("LICENSE");
      expect(workspace.fileTypeOf("LICENSE")?.id).toBe("text");
      expect(await workspace.readFile("LICENSE")).toBe("Copyright");
      expect(await workspace.readFile("huge.log")).toBeUndefined();
    } finally {
      fetch.mockRestore();
      await workspace.openFile("main.typ");
      workspace.files = [];
    }
  });

  it("the type the person chose for a file is the one everything reads", () => {
    expect(workspace.fileTypeOf("notes.txt")?.id).toBe("text");
    viewChoice.openAsTable(workspace.project, "notes.txt");
    expect(workspace.fileTypeOf("notes.txt")?.id).toBe("csv");
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
        { path: "main.typ", size: 0, main: true },
        { path: "notes.md", size: 3, main: false },
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
  it("PC1 the invitation shown follows what Go says of it, and a fresh one replaces it", async () => {
    const invite = vi
      .fn()
      .mockResolvedValueOnce({ text: "first" })
      .mockResolvedValueOnce({ text: "second" });
    let held = "open";
    const state = vi.fn(async () => held);
    Object.assign(Session, { Invite: invite, InvitationState: state });
    await workspace.host();
    expect(workspace.invitation).toBe("first");
    expect(workspace.invitationState).toBe("open");

    // J-8: someone redeemed it; the person may never connect (a cancelled join).
    held = "used";
    await workspace.refreshInvitation();
    expect(state).toHaveBeenLastCalledWith("first");
    expect(workspace.invitationState, "J-8 a used invitation is not offered").toBe("used");

    // A fresh invitation is open; a new connection makes the page ask Go again.
    held = "open";
    await workspace.host();
    expect([workspace.invitation, workspace.invitationState]).toEqual(["second", "open"]);
    held = "expired";
    workspace.peerUp("bob");
    await vi.waitFor(() => expect(workspace.invitationState).toBe("expired"));
    workspace.invitation = "";
    workspace.invitationState = "open";
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
