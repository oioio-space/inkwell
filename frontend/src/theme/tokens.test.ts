import { describe, expect, it } from "vitest";
import css from "./tokens.css?raw";

const luminance = (hex: string) => {
  const [r, g, b] = [1, 3, 5].map((i) => {
    const c = Number.parseInt(hex.slice(i, i + 2), 16) / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * (r ?? 0) + 0.7152 * (g ?? 0) + 0.0722 * (b ?? 0);
};
const contrast = (a: string, b: string) => {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return ((hi ?? 0) + 0.05) / ((lo ?? 0) + 0.05);
};

/** A semantic token written as `light-dark(#aaaaaa, #bbbbbb)`. */
function lightDark(name: string): { light: string; dark: string } {
  const m = css.match(new RegExp(`${name}:\\s*light-dark\\((#[0-9a-f]{6}),\\s*(#[0-9a-f]{6})\\)`));
  if (!m?.[1] || !m[2]) throw new Error(`${name} is not a light-dark() of two hex colours`);
  return { light: m[1], dark: m[2] };
}

const SYNTAX = [
  "heading",
  "function",
  "string",
  "label",
  "comment",
  "keyword",
  "number",
  "raw",
  "marker",
];

describe("syntax colours", () => {
  // The editor background is --p-light-1 (white) in light and --p-dark-5 in dark.
  const light = "#ffffff";
  const dark = css.match(/--p-dark-5:\s*(#[0-9a-f]{6})/)?.[1] ?? "";

  it("starts from the dark editor background of the design", () => {
    expect(dark).toBe("#1d1d20");
  });

  for (const name of SYNTAX) {
    it(`--hl-${name} reaches WCAG AA (4.5:1) in light and in dark`, () => {
      const c = lightDark(`--hl-${name}`);
      expect(contrast(c.light, light)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(c.dark, dark)).toBeGreaterThanOrEqual(4.5);
    });
  }

  it("keeps the mockup's hues where they already pass", () => {
    expect(lightDark("--hl-heading").light).toBe("#1c71d8");
    expect(lightDark("--hl-function").light).toBe("#c64600");
    expect(lightDark("--hl-label").light).toBe("#9141ac");
  });
});
