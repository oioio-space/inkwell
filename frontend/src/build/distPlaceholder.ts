import type { Plugin } from "vite";

/** The placeholder git tracks in frontend/dist, so that `//go:embed all:frontend/dist` (inkwell.go) resolves on
 * a fresh checkout, before any frontend build. */
export const DIST_PLACEHOLDER = ".gitkeep";

/** Emits the empty placeholder with the bundle: Vite empties the output directory before writing, so every build
 * (`pnpm run build`, a raw `vite build`, a side directory swapped in later) would otherwise delete a tracked file
 * and dirty the tree (block PF1). */
export const distPlaceholder = (): Plugin => ({
  name: "inkwell-dist-placeholder",
  apply: "build",
  generateBundle() {
    this.emitFile({ type: "asset", fileName: DIST_PLACEHOLDER, source: "" });
  },
});
