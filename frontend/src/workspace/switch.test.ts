import { describe, expect, it, vi } from "vitest";
import { workspace } from "./workspace.svelte";

// The Wails runtime dials its host on import; switching files never needs it.
vi.mock("@wailsio/runtime", () => ({ Stream: vi.fn() }));
vi.mock("../../bindings/github.com/oioio-space/inkwell/internal/app/index", () => ({
  Projects: {},
  Session: {},
}));

// EX0c3 step 2 (plan §4, the EX0c1-E2 item): a MutationObserver on the status bar showed the
// caret, the words and the language vanish for 11 to 14 ms and come back on a switch from
// main.typ to a note, while `.cm-content` was gone: `openFile` named the new file and dropped the
// old one before reading the new one, so for the time of the read no file was open.
describe("switching files", () => {
  it("EX0c3 a switch to another file changes the current file and its content together, never through no file", async () => {
    let release: () => void = () => {};
    const read = new Promise<void>((r) => {
      release = r;
    });
    const fetch = vi.spyOn(globalThis, "fetch").mockImplementation(async (_url, init) => {
      if (init?.method === "HEAD")
        return new Response(null, { status: 200, headers: { ETag: '"1"' } });
      await read;
      return new Response("# Notes\n", { status: 200 });
    });
    const info = vi.spyOn(workspace, "refreshInfo").mockResolvedValue();
    try {
      workspace.files = [
        { path: "main.typ", size: 0, main: true },
        { path: "notes.md", size: 8, main: false },
      ];
      await workspace.openFile("main.typ");
      const switching = workspace.openFile("notes.md");
      await vi.waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
      // The read is under way: the shell still shows main.typ, whole.
      expect(workspace.current).toBe("main.typ");
      expect(workspace.opened).toBeUndefined();
      release();
      await switching;
      expect(workspace.current).toBe("notes.md");
      expect(workspace.opened?.text).toBe("# Notes\n");
    } finally {
      fetch.mockRestore();
      info.mockRestore();
      await workspace.openFile("main.typ");
      workspace.files = [];
    }
  });
});
