import { mkdirSync, mkdtempSync, readdirSync, rmSync } from "node:fs";
import { join, resolve } from "node:path";
import { defineConfig, devices } from "@playwright/test";
import { ALICE_PORT, BOB_PORT } from "./e2e/ports";

// Each run gets fresh project folders (one per instance), shared with the workers through the
// environment: the specs read what the instances write there.
const e2eDir = resolve(import.meta.dirname, "../.task/e2e");
if (!process.env.INKWELL_E2E_DOCS) {
  mkdirSync(e2eDir, { recursive: true });
  for (const old of readdirSync(e2eDir)) {
    if (old.startsWith("docs-")) rmSync(join(e2eDir, old), { recursive: true, force: true });
  }
  process.env.INKWELL_E2E_DOCS = mkdtempSync(join(e2eDir, "docs-"));
}
const docsRoot = process.env.INKWELL_E2E_DOCS;

// Two real inkwell instances in Wails server mode (`-tags server`): same Go code, same
// frontend, the Stream bridge becomes a WebSocket. Built by `mise run e2e`.
const browser = process.env.INKWELL_E2E_BROWSER ?? (process.env.CI ? "webkit" : "chromium");

const instance = (port: number) => ({
  command: `../bin/inkwell-server`,
  // Each instance has its own identity directory, or both would be the same peer.
  env: {
    INKWELL_SERVER_PORT: String(port),
    // No short codes: the tests never reach the public code server.
    INKWELL_RENDEZVOUS: "off",
    INKWELL_HOME: resolve(e2eDir, String(port)),
    INKWELL_DOCS: join(docsRoot, String(port)),
  },
  url: `http://127.0.0.1:${port}/health`,
  reuseExistingServer: false,
  timeout: 30_000,
});

export default defineConfig({
  testDir: "e2e",
  testIgnore: "netsim.spec.ts", // needs network namespaces: `mise run e2e:netsim`
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? "github" : "list",
  use: { trace: "retain-on-failure" },
  // WebKit is the engine behind WebKitGTK, which the Linux desktop app uses; CI (Ubuntu)
  // runs it. Playwright's WebKit build does not run on Fedora, so local runs default to
  // Chromium — override with INKWELL_E2E_BROWSER=webkit.
  projects: [
    browser === "webkit"
      ? {
          name: "webkit",
          use: {
            ...devices["Desktop Safari"],
            // `mise run e2e:webkit`: WebKit runs in the Playwright container, the tests here.
            ...(process.env.INKWELL_E2E_WS
              ? { connectOptions: { wsEndpoint: process.env.INKWELL_E2E_WS } }
              : {}),
          },
        }
      : { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  ],
  webServer: [instance(ALICE_PORT), instance(BOB_PORT)],
});
