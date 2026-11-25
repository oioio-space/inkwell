/**
 * Typst's commands (ext/typst): presenting a document's pages, exporting it, jumping from the
 * source to the preview, the preview's zoom, and the help on the function at the caret (in the target editor).
 */
import type { EditorView } from "@codemirror/view";
import type { CommandModule } from "../../../extensions/commands";
import { requestSignature } from "../../../language/extension";
import { fitPreview, stepPreviewZoom } from "../../../preview/zoom.svelte";

export const commands: CommandModule["commands"] = {
  "typst.present": ({ shell }) => shell.present("start"),
  "typst.present-here": ({ shell }) => shell.present("here"),
  "typst.present-public": ({ shell }) => shell.present("public"),
  "typst.export": ({ shell }) => shell.exportPdf(),
  "typst.jump-to-preview": ({ shell }) => shell.jumpToPreview(),
  "typst.preview-zoom-out": () => stepPreviewZoom(-1),
  "typst.preview-zoom-in": () => stepPreviewZoom(1),
  "typst.preview-fit": () => fitPreview(),
  "typst.signature-help": ({ view }) => {
    if (view) void requestSignature(view as EditorView, true);
    return view !== undefined;
  },
};
