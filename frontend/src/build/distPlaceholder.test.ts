// @vitest-environment node
import { existsSync, mkdirSync, mkdtempSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { build } from "vite";
import { afterEach, describe, expect, it } from "vitest";
import { DIST_PLACEHOLDER, distPlaceholder } from "./distPlaceholder";

const dirs: string[] = [];
afterEach(() => {
  for (const dir of dirs.splice(0)) rmSync(dir, { recursive: true, force: true });
});

// A real (tiny) Vite build: the output directory is emptied first, as in every frontend build.
async function buildInto(withPlugin: boolean): Promise<string> {
  const root = mkdtempSync(join(tmpdir(), "dist-placeholder-"));
  dirs.push(root);
  writeFileSync(join(root, "index.html"), '<script type="module" src="./main.js"></script>');
  writeFileSync(join(root, "main.js"), "document.title = 'x';\n");
  const outDir = join(root, "out");
  mkdirSync(outDir);
  writeFileSync(join(outDir, DIST_PLACEHOLDER), "");
  writeFileSync(join(outDir, "stale.js"), "");
  await build({
    root,
    configFile: false,
    logLevel: "silent",
    plugins: withPlugin ? [distPlaceholder()] : [],
    build: { outDir, emptyOutDir: true },
  });
  return outDir;
}

describe("distPlaceholder (the tracked frontend/dist/.gitkeep survives every build, block PF1)", () => {
  it("the build empties its output directory, placeholder included, without the plugin", async () => {
    const out = await buildInto(false);
    expect(existsSync(join(out, DIST_PLACEHOLDER))).toBe(false);
  });

  it("the build writes an empty placeholder back, and still empties the rest", async () => {
    const out = await buildInto(true);
    expect(readdirSync(out)).toContain(DIST_PLACEHOLDER);
    expect(readdirSync(out)).not.toContain("stale.js");
    expect(readdirSync(out)).toContain("index.html");
  });

  it("the application's own config keeps it, as the reference build of PERF-START runs it", async () => {
    const out = mkdtempSync(join(tmpdir(), "dist-placeholder-app-"));
    dirs.push(out);
    await build({
      configFile: fileURLToPath(new URL("../../vite.config.ts", import.meta.url)),
      configLoader: "runner",
      mode: "production",
      logLevel: "silent",
      build: { outDir: out, emptyOutDir: true },
    });
    expect(readdirSync(out)).toEqual(expect.arrayContaining([DIST_PLACEHOLDER, "index.html"]));
  }, 60_000);
});
