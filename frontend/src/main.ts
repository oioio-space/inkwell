import { mount } from "svelte";
import { preloadStartup } from "./extensions/loader";
import "./theme/tokens.css";
import "./theme/adwaita.css";

const target = document.getElementById("app");
if (!target) throw new Error("missing #app mount point");

// `?view=public` is the audience window of a presentation: the slide alone, with no project, no
// editor and no stream but the one that joins it to the presenter's window.
const isPublic = new URLSearchParams(location.search).get("view") === "public";
if (import.meta.env.VITE_PERF_NATIVE === "1" && !isPublic) {
  const { installNativeEarly } = await import("./perf/nativeEarly");
  installNativeEarly(true);
}
if (!isPublic) preloadStartup();
// One loader per root, never `cond ? import(a) : import(b)`: Vite wraps each import() in a preload
// call whose dependency list (the chunk's CSS) is filled in after the minifier ran, and the
// minifier folds the two identical-looking calls of a conditional into one, which then carries
// App's list for both branches (the public window got App's CSS and not its own, 2026-11-24).
const roots = {
  public: () => import("./present/PublicWindow.svelte"),
  app: () => import("./App.svelte"),
};
const { default: Root } = await roots[isPublic ? "public" : "app"]();
mount(Root, { target });
