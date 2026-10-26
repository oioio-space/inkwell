import { register } from "./registry";

register({
  id: "markdown",
  label: "kind.markdown",
  icon: "markdown",
  extensions: [".md", ".markdown"],
  mime: ["text/markdown"],
  formatProfile: "markdown",
  wordCount: "markdown",
  preview: "markdown",
  open: () => import("./editors/text"),
});
