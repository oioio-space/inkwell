// @vitest-environment node
import { Repo } from "@automerge/automerge-repo";
import { afterEach, expect, it, vi } from "vitest";
import { observeDocText, observeHeads, observeLinks, observeVariantHeads } from "./observe";

const observed = globalThis as typeof globalThis & {
  __inkwellVariantHeads?: (name: string) => string[];
  __inkwellDocText?: () => string | undefined;
  __inkwellLinks?: () => unknown[];
  __inkwellHeads?: () => string[];
};
afterEach(() => {
  vi.unstubAllEnvs();
  delete observed.__inkwellVariantHeads;
  delete observed.__inkwellDocText;
  delete observed.__inkwellLinks;
  delete observed.__inkwellHeads;
});
it("production e2e builds can inspect heads without opening or fetching variants", async () => {
  vi.stubEnv("MODE", "production");
  vi.stubEnv("VITE_E2E_OBSERVE", "1");
  const repo = new Repo();
  try {
    const variant = repo.create({ text: "Carol" });
    const root = repo.create({ meta: { variants: [{ name: "v-partie", url: variant.url }] } });
    const find = vi.spyOn(repo, "find");
    observeVariantHeads(
      () => root,
      () => repo,
    );
    expect(observed.__inkwellVariantHeads).toBeTypeOf("function");
    expect(observed.__inkwellVariantHeads?.("v-partie")).toEqual([...variant.heads()].sort());
    expect(observed.__inkwellVariantHeads?.("missing")).toEqual([]);
    expect(find).not.toHaveBeenCalled();
  } finally {
    await repo.shutdown();
  }
});
it("ordinary production builds expose no observer", () => {
  vi.stubEnv("MODE", "production");
  vi.stubEnv("VITE_E2E_OBSERVE", "");
  observeVariantHeads(
    () => undefined,
    () => undefined,
  );
  expect(observed.__inkwellVariantHeads).toBeUndefined();
});
it("e2e builds expose the document text and the links, other builds nothing", () => {
  vi.stubEnv("MODE", "production");
  vi.stubEnv("VITE_E2E_OBSERVE", "");
  observeDocText(() => "x");
  observeLinks(() => []);
  expect(observed.__inkwellDocText).toBeUndefined();
  expect(observed.__inkwellLinks).toBeUndefined();
  vi.stubEnv("VITE_E2E_OBSERVE", "1");
  observeDocText(() => "x");
  observeLinks(() => []);
  expect(observed.__inkwellDocText?.()).toBe("x");
  expect(observed.__inkwellLinks?.()).toEqual([]);
});
it("e2e builds expose the heads of the document shown, sorted; other builds nothing", () => {
  vi.stubEnv("MODE", "production");
  vi.stubEnv("VITE_E2E_OBSERVE", "");
  observeHeads(() => ["b", "a"]);
  expect(observed.__inkwellHeads).toBeUndefined();
  vi.stubEnv("VITE_E2E_OBSERVE", "1");
  observeHeads(() => ["b", "a"]);
  expect(observed.__inkwellHeads?.()).toEqual(["a", "b"]);
  observeHeads(() => undefined);
  expect(observed.__inkwellHeads?.()).toEqual([]);
});
