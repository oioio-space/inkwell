// @vitest-environment node
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

describe("amSync does not reach into another package's build output", () => {
  it("no import from automerge-codemirror/dist (the patch translation is vendored in amPatches.ts)", () => {
    for (const file of ["amSync.ts", "amPatches.ts"]) {
      const source = readFileSync(new URL(`./${file}`, import.meta.url), "utf8");
      expect(source, file).not.toMatch(/automerge-codemirror\/dist/);
    }
  });
});
