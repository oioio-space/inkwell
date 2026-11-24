import { describe, expect, it } from "vitest";
import { viewerProps } from "./context";
import { builtinSources } from "./loader";
import { buildRegistry } from "./registry";

const registry = buildRegistry(builtinSources(), () => Promise.resolve({}));
const opening = registry.open("notes.csv", { content: "text" });
const native = {
  opening,
  path: "dir/notes.csv",
  lang: "plain" as const,
  me: { id: "me", name: "Me" },
  big: { text: "a,b\n1,2\n" },
  readOnlyReason: "read-only here",
  onshowtext: () => {},
  readImage: async () => undefined,
};

describe("what a viewer receives (ADR 0027, « Never exposed »; code review, finding 4)", () => {
  it("only a viewer declaring ui.native gets the editor's handles; any other gets the facade", async () => {
    const grid = opening.viewer;
    expect(grid.capabilities).toContain("ui.native");
    expect(viewerProps(grid, native)).toBe(native);
    const plain = { ...grid, capabilities: [] };
    const props = viewerProps(plain, native);
    expect(Object.keys(props)).toEqual(["ctx"]);
    if (!("ctx" in props)) return;
    const ctx = props.ctx;
    expect([ctx.name, ctx.path, ctx.size, ctx.readOnlyReason]).toEqual([
      "notes.csv",
      "dir/notes.csv",
      8,
      "read-only here",
    ]);
    expect(new TextDecoder().decode(await ctx.read({ from: 0, to: 3 }))).toBe("a,b");
    expect(await ctx.propose({ from: 0, to: 0, insert: "x" })).toBe(false);
    for (const key of ["handle", "big", "readImage", "me", "opening"])
      expect(key in ctx).toBe(false);
    expect(Object.isFrozen(ctx)).toBe(true);
  });
});
