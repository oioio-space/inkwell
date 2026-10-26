/**
 * A page of the preview is an SVG made by Typst from a document that anyone in the project may
 * write. It is never put in the page as markup: it is shown as an image (`<img src="data:…">`),
 * which the browser renders without running scripts, loading anything or reaching the page's
 * DOM, and the content security policy allows `data:` images only. This module adds a second,
 * independent line: anything that is not a plain SVG document, or that holds an active element,
 * is refused.
 */

export interface PageImage {
  /** A `data:` URL for an `<img>`. */
  src: string;
  /** The size of the page in points (the unit of Typst and of the positions Go reports). */
  width: number;
  height: number;
}

const MAX_BYTES = 64 << 20;
// Typst writes glyphs and shapes as paths; none of these can appear in what it produces.
const ACTIVE =
  /<\s*(script|foreignObject|iframe|object|embed|animate|set)\b|\son[a-z]+\s*=|javascript:/i;
const ROOT = /^\s*(?:<\?xml[^>]*\?>\s*)?<svg\b([^>]*)>/;

function points(value: string | undefined): number | undefined {
  const m = value?.match(/^\s*([0-9]*\.?[0-9]+)\s*(pt)?\s*$/);
  const n = m ? Number(m[1]) : Number.NaN;
  return Number.isFinite(n) && n > 0 && n < 100_000 ? n : undefined;
}

function attr(tag: string, name: string): string | undefined {
  return tag.match(new RegExp(`\\s${name}\\s*=\\s*"([^"]*)"`))?.[1];
}

/** The size in points of an SVG page, from its root element; undefined if it has none. */
export function pageSize(svg: string): { width: number; height: number } | undefined {
  const tag = svg.match(ROOT)?.[1];
  if (tag === undefined) return undefined;
  const width = points(attr(tag, "width"));
  const height = points(attr(tag, "height"));
  if (width && height) return { width, height };
  const box = attr(tag, "viewBox")
    ?.trim()
    .split(/[\s,]+/);
  if (box?.length === 4) {
    const w = points(box[2]);
    const h = points(box[3]);
    if (w && h) return { width: w, height: h };
  }
  return undefined;
}

/** Turns the SVG of a page into an image to show, or undefined when it must not be shown. */
export function pageImage(svg: string): PageImage | undefined {
  if (svg.length > MAX_BYTES || ACTIVE.test(svg)) return undefined;
  const size = pageSize(svg);
  if (!size) return undefined;
  return { src: `data:image/svg+xml;base64,${toBase64(svg)}`, ...size };
}

function toBase64(text: string): string {
  const bytes = new TextEncoder().encode(text);
  let binary = "";
  const step = 0x8000;
  for (let i = 0; i < bytes.length; i += step) {
    binary += String.fromCharCode(...bytes.subarray(i, i + step));
  }
  return btoa(binary);
}
