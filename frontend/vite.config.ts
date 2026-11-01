import { fileURLToPath } from "node:url";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import wails from "@wailsio/runtime/plugins/vite";
import type { Plugin } from "vite";
import { defineConfig } from "vitest/config";

// Production keeps the strict CSP from index.html; only the dev server relaxes it for
// Vite's HMR socket and injected <style> tags.
const devCsp = (): Plugin => ({
  name: "inkwell-dev-csp",
  apply: "serve",
  transformIndexHtml: (html) =>
    html
      .replace("connect-src 'self'", "connect-src 'self' ws://127.0.0.1:* http://127.0.0.1:*")
      .replace("style-src 'self' 'nonce-__CSP_NONCE__'", "style-src 'self' 'unsafe-inline'"),
});

export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  build: { target: "es2023" },
  // Workers are ES modules too: the Automerge wasm loads with a top-level await.
  worker: { format: "es" },
  plugins: [svelte(), wails("./bindings"), devCsp()],
  // Component tests mount Svelte components in happy-dom: they need the browser build. Outside
  // tests, leave Vite's default conditions alone (an empty list would drop "browser").
  resolve: {
    alias: {
      "@inkwell/protocol": fileURLToPath(
        new URL("./packages/protocol/src/index.ts", import.meta.url),
      ),
    },
    ...(process.env.VITEST ? { conditions: ["browser"] } : {}),
  },
  cacheDir: "./test-results/.vite",
  test: {
    execArgv: ["--expose-gc"],
    // Worktrees may share node_modules, but cached transforms contain absolute binding paths.
    // Keep them local so mocks and imports resolve to the same worktree's modules.
    fsModuleCachePath: fileURLToPath(new URL("./test-results/.vitest-cache", import.meta.url)),
    environment: "happy-dom",
    include: ["src/**/*.test.ts", "packages/protocol/**/*.test.ts"],
    setupFiles: ["src/test-setup.ts"],
    // tokens.test.ts reads the stylesheet as text.
    css: { include: [/tokens\.css/] },
  },
});
