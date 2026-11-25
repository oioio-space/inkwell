/**
 * The preview's zoom (EX0c1): one state for the preview pane and its commands
 * (`typst.preview-zoom-out`, `-in`, `typst.preview-fit`), so its tools are locations of the
 * registry. `fit` follows the pane's width; a number is a fixed zoom.
 */
import { clampZoom, stepZoom } from "./layout";

export const previewZoom = $state({ mode: "fit" as "fit" | number, current: 1 });

/** One step out or in from the zoom shown now. */
export function stepPreviewZoom(direction: 1 | -1): void {
  previewZoom.mode = clampZoom(stepZoom(previewZoom.current, direction));
}

/** Back to the pane's width. */
export function fitPreview(): void {
  previewZoom.mode = "fit";
}
