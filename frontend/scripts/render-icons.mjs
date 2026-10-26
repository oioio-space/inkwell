// Renders the app icon sources (build/icons/*.svg) to the PNG sizes the desktop needs, with
// Chromium so gradients and strokes match the SVG exactly. Run with `mise run icons`.
// Two trees: `icons` for releases, `icons-nightly` for pre-releases (-rc.N), which the packages
// pick by version (GNOME: a pre-release app has its own, striped icon; ADR 0025).
import { mkdirSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { chromium } from "@playwright/test";

const root = resolve(import.meta.dirname, "../..");
const id = "io.github.oioio_space.Inkwell";
const sizes = [16, 24, 32, 48, 64, 128, 256, 512];
const sources = [
  { svg: "inkwell.svg", tree: "icons", extra: [{ size: 1024, out: "build/appicon.png" }] },
  { svg: "inkwell-nightly.svg", tree: "icons-nightly", extra: [] },
];

const browser = await chromium.launch();
const page = await browser.newPage();
for (const { svg: file, tree, extra } of sources) {
  const svg = readFileSync(resolve(root, "build/icons", file), "utf8");
  const targets = [
    ...sizes.map((s) => ({
      size: s,
      out: `build/linux/${tree}/hicolor/${s}x${s}/apps/${id}.png`,
    })),
    ...extra,
  ];
  for (const { size, out } of targets) {
    const path = resolve(root, out);
    mkdirSync(dirname(path), { recursive: true });
    await page.setViewportSize({ width: size, height: size });
    await page.setContent(
      `<body style="margin:0">${svg.replace("<svg ", `<svg width="${size}" height="${size}" `)}</body>`,
    );
    await page.screenshot({
      path,
      omitBackground: true,
      clip: { x: 0, y: 0, width: size, height: size },
    });
  }
}
await browser.close();
