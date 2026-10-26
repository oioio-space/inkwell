/**
 * The file types the system may open in its own program. A file comes from another person, so
 * only types made to be looked at qualify: documents, images, fonts. Programs, scripts,
 * shortcuts, macro documents and SVG (which can carry script) are shown in the folder instead.
 * The same list stands in internal/app/projects.go, which enforces it; this one only chooses
 * the wording of the button.
 */
const VIEWABLE = new Set([
  ".pdf",
  ".png",
  ".jpg",
  ".jpeg",
  ".gif",
  ".webp",
  ".bmp",
  ".tif",
  ".tiff",
  ".ico",
  ".avif",
  ".ttf",
  ".otf",
  ".woff",
  ".woff2",
  ".docx",
  ".odt",
  ".xlsx",
  ".ods",
  ".pptx",
  ".odp",
]);

/** True if the system may open this file in its program for it (see above). */
export function isViewable(name: string): boolean {
  const dot = name.lastIndexOf(".");
  return dot > 0 && VIEWABLE.has(name.slice(dot).toLowerCase());
}
