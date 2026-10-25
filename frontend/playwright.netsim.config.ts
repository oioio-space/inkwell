import { defineConfig, devices } from "@playwright/test";

// Used by scripts/netsim/run.sh only: the instances already run, inside network
// namespaces, and are bridged to 127.0.0.1:18081 (alice) and :18082 (bob). The network
// layer is Go, so one browser engine is enough; Chromium also runs as the (namespace)
// root that the script runs as.
export default defineConfig({
  testDir: "e2e",
  testMatch: "netsim.spec.ts",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 300_000,
  reporter: process.env.CI ? "github" : "list",
  use: { trace: "retain-on-failure" },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
