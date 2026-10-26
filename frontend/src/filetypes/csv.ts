import { register } from "./registry";
import type { EditorModule } from "./types";

// The sheet (B9) lives in ui/sheet. Until that module exists the tables open as text; when it
// does, nothing here changes: the glob finds it. TODO(B9): the sheet module is expected at
// ui/sheet/module.ts and must default-export an EditorModule.
const sheets = import.meta.glob<{ default: EditorModule }>("../ui/sheet/module.ts");
const open = async (): Promise<{ default: EditorModule }> => {
  const load = sheets["../ui/sheet/module.ts"];
  return load ? load() : import("./editors/text");
};

register({
  id: "csv",
  label: "kind.csv",
  icon: "csv",
  extensions: [".csv"],
  mime: ["text/csv"],
  wordCount: "none",
  dialect: { delimiter: "," },
  open,
});

register({
  id: "tsv",
  label: "kind.tsv",
  icon: "tsv",
  extensions: [".tsv", ".tab"],
  mime: ["text/tab-separated-values"],
  wordCount: "none",
  dialect: { delimiter: "\t" },
  open,
});
