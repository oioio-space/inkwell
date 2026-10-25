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
  plugins: [svelte(), wails("./bindings"), devCsp()],
  test: {
    environment: "happy-dom",
    include: ["src/**/*.test.ts"],
  },
});
