import { register } from "./registry";

register({
  id: "typst",
  label: "kind.typst",
  icon: "typst",
  extensions: [".typ", ".typst"],
  mime: ["text/x-typst"],
  formatProfile: "typst",
  wordCount: "typst",
  preview: "typst",
  open: () => import("./editors/text"),
});
