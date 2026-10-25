// Renders the app icon sources (build/icons/*.svg) to the PNG sizes the desktop needs, with
// Chromium so gradients and strokes match the SVG exactly. Run with `mise run icons`.
import { mkdirSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { chromium } from "@playwright/test";

const root = resolve(import.meta.dirname, "../..");
const id = "io.github.oioio_space.Inkwell";
const svg = readFileSync(resolve(root, "build/icons/inkwell.svg"), "utf8");
const targets = [
  ...[16, 24, 32, 48, 64, 128, 256, 512].map((s) => ({
    size: s,
    out: `build/linux/icons/hicolor/${s}x${s}/apps/${id}.png`,
  })),
  { size: 1024, out: "build/appicon.png" },
];

const browser = await chromium.launch();
const page = await browser.newPage();
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
await browser.close();
