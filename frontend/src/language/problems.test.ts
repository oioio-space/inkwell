import { webcrypto } from "node:crypto";
import { next as A } from "@automerge/automerge";
import { beforeEach, describe, expect, it, vi } from "vitest";

const sourceDoc = A.from({ text: "#bad" });
vi.mock("../workspace/workspace.svelte", () => ({
  workspace: {
    handle: { doc: () => sourceDoc },
    current: "main.typ",
    files: [{ path: "main.typ" }, { path: "chapter.typ" }],
    openFile: vi.fn(async () => {}),
  },
}));
vi.mock("../ui/editorSession.svelte", () => ({ session: {} }));
vi.mock("../ui/history/history.svelte", () => ({ history: {} }));

import type { EditorView } from "@codemirror/view";
import { session } from "../ui/editorSession.svelte";
import { Problems } from "./problems.svelte";

describe("ER-1 source reconciliation and LSP-STALE compiler diagnostics", () => {
  let model: Problems;
  beforeEach(() => {
    model = new Problems();
  });
  it("keeps unmatched compiler revisions visible and inactive", () => {
    model.compiler(
      [{ severity: "error", message: "old compilation", file: "main.typ", line: 1 }],
      1,
    );
    expect(model.entries[0]?.stale).toBe(true);
    expect(model.entries[0]?.anchor).toBeUndefined();
  });
  it("uses language diagnostics first, then restores compiler fallback after empty publication", () => {
    model.compiler(
      [{ severity: "error", message: "unknown variable: bad", file: "main.typ", line: 1 }],
      1,
    );
    model.publish("main.typ", 1, [
      {
        message: "unknown variable: bad",
        range: { start: { line: 0, character: 1 }, end: { line: 0, character: 4 } },
      },
    ]);
    expect(model.entries).toHaveLength(1);
    expect(model.entries[0]?.source).toBe("language");
    model.publish("main.typ", 2, []);
    expect(model.entries).toHaveLength(1);
    expect(model.entries[0]?.source).toBe("compiler");
  });
  it("activates only content-verified compiler spans and rejects a late mismatched compilation", async () => {
    vi.stubGlobal("crypto", webcrypto);
    const bytes = await crypto.subtle.digest("SHA-256", new TextEncoder().encode("#bad"));
    const hash = Array.from(new Uint8Array(bytes), (b) => b.toString(16).padStart(2, "0")).join("");
    await model.compilerVerified(
      [
        {
          severity: "error",
          message: "unknown variable: bad",
          file: "main.typ",
          line: 1,
          column: 2,
        },
      ],
      1,
      { "main.typ": hash },
    );
    expect(model.entries[0]?.stale).toBe(false);
    expect(model.entries[0]?.from).toBe(1);
    expect(model.entries[0]?.to).toBe(4);
    await model.compilerVerified(
      [
        {
          severity: "error",
          message: "unknown variable: old",
          file: "main.typ",
          line: 1,
          column: 2,
        },
      ],
      0,
      { "main.typ": "0".repeat(64) },
    );
    expect(model.entries[0]?.stale).toBe(true);
    expect(model.entries[0]?.anchor).toBeUndefined();
    vi.unstubAllGlobals();
  });
  it("refuses to navigate a projection changed after verification", async () => {
    vi.stubGlobal("crypto", webcrypto);
    const bytes = await crypto.subtle.digest("SHA-256", new TextEncoder().encode("#bad"));
    const hash = Array.from(new Uint8Array(bytes), (b) => b.toString(16).padStart(2, "0")).join("");
    const dispatch = vi.fn();
    session.view = {
      state: { doc: { toString: () => "#changed" } },
      dispatch,
    } as unknown as EditorView;
    const problem = {
      source: "compiler",
      file: "chapter.typ",
      severity: "error" as const,
      message: "unknown variable: bad",
      version: 1,
      from: 1,
      to: 4,
      stale: false,
      verifiedHash: hash,
    };
    model.sources = [problem];
    await model.jump(problem);
    expect(dispatch).not.toHaveBeenCalled();
    expect(model.entries[0]?.stale).toBe(true);
    session.view = undefined;
    vi.unstubAllGlobals();
  });
  it("retains unconverted positions of an unopened projection", () => {
    model.publish("chapter.typ", 0, [
      {
        message: "bad",
        range: { start: { line: 8, character: 4 }, end: { line: 8, character: 9 } },
      },
    ]);
    expect(model.entries[0]?.position?.start).toEqual({ line: 8, character: 4 });
  });
});
