import { EditorSelection } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { PreviewService } from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { t } from "../i18n";
import { session } from "../ui/editorSession.svelte";
import { notice } from "../ui/notice.svelte";
import { workspace } from "../workspace/workspace.svelte";
import { preview } from "./preview.svelte";
import type { Diagnostic } from "./protocol";
import { lineAt, pairUp, placeOf } from "./sync";

/** The file the editor shows: today the project's main document. */
const shownFile = () => workspace.current;

/** Puts the caret at `line` (and `column`, in characters) of the editor and shows it. */
export function placeCaret(line: number, column = 1): boolean {
  const view = session.view;
  if (!view) return false;
  const doc = view.state.doc;
  const l = doc.line(Math.min(Math.max(1, line), doc.lines));
  const before = Array.from(l.text).slice(0, Math.max(0, column - 1));
  const pos = l.from + before.join("").length;
  view.dispatch({
    selection: EditorSelection.cursor(pos),
    effects: EditorView.scrollIntoView(pos, { y: "center" }),
  });
  view.focus();
  return true;
}

/** Jumps from a problem of the list to its place in the source. */
export function jumpToProblem(d: Diagnostic): void {
  if (d.line === undefined) return;
  if (d.file !== undefined && d.file !== shownFile()) {
    notice.show(t("preview.jump.otherFile", { file: d.file, line: d.line }));
    return;
  }
  placeCaret(d.line, d.column ?? 1);
}

/** Jumps from the caret to the page that shows the paragraph or heading it is in. */
export async function jumpFromCursor(): Promise<void> {
  const state = session.state;
  if (!state) return;
  try {
    const anchors = await preview.anchors();
    const line = state.doc.lineAt(state.selection.main.head).number;
    const place = placeOf(pairUp(state.doc.toString(), anchors), line);
    if (!place) {
      notice.show(t("preview.jump.unknown"));
      return;
    }
    preview.mark(place.page, place.y);
  } catch {
    notice.show(t("preview.jump.unknown"));
  }
}

/** Jumps from a click on a page to the line of the paragraph or heading that was clicked. */
export async function jumpFromPage(page: number, _x: number, y: number): Promise<void> {
  const state = session.state;
  if (!state) return;
  try {
    const anchors = await preview.anchors();
    const line = lineAt(pairUp(state.doc.toString(), anchors), page, y);
    if (line === undefined || !placeCaret(line)) notice.show(t("preview.jump.unknown"));
  } catch {
    notice.show(t("preview.jump.unknown"));
  }
}

/** "Exporter en PDF": asks where to save, writes the PDF, and says what happened. */
export async function exportPdf(): Promise<void> {
  try {
    // The PDF is made from the project folder: what was typed a moment ago must be there first.
    await workspace.flushProjection();
    const r = await PreviewService.ExportPDF();
    if (r.Cancelled) return;
    if (r.Reason === "errors") notice.show(t("export.errors"), "alert");
    else if (r.Reason === "unavailable") notice.show(t("export.unavailable"), "alert");
    else if (r.Reason) notice.show(t("export.failed"), "alert");
    else notice.show(t("export.done", { path: r.Path }));
  } catch {
    notice.show(t("export.failed"), "alert");
  }
}
