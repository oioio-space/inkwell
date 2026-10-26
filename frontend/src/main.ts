import { mount } from "svelte";
import "./theme/tokens.css";
import "./theme/adwaita.css";

const target = document.getElementById("app");
if (!target) throw new Error("missing #app mount point");

// `?view=public` is the audience window of a presentation: the slide alone, with no project, no
// editor and no stream but the one that joins it to the presenter's window.
const isPublic = new URLSearchParams(location.search).get("view") === "public";
const { default: Root } = await (isPublic
  ? import("./present/PublicWindow.svelte")
  : import("./App.svelte"));
mount(Root, { target });
