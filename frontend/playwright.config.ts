import { resolve } from "node:path";
import { defineConfig, devices } from "@playwright/test";

// Two real inkwell instances in Wails server mode (`-tags server`): same Go code, same
// frontend, the Stream bridge becomes a WebSocket. Built by `mise run e2e`.
const browser = process.env.INKWELL_E2E_BROWSER ?? (process.env.CI ? "webkit" : "chromium");

const instance = (port: number) => ({
  command: `../bin/inkwell-server`,
  // Each instance has its own identity directory, or both would be the same peer.
  env: {
    INKWELL_SERVER_PORT: String(port),
    INKWELL_HOME: resolve(import.meta.dirname, "../.task/e2e", String(port)),
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
      ? { name: "webkit", use: { ...devices["Desktop Safari"] } }
      : { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  ],
  webServer: [instance(18081), instance(18082)],
});
