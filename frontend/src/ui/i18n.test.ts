import { describe, expect, it } from "vitest";
import en from "../i18n/en.json";
import fr from "../i18n/fr.json";

const sources = import.meta.glob(["../**/*.svelte", "../**/*.ts", "!../**/*.test.ts"], {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;

const svelte = Object.entries(sources).filter(([path]) => path.endsWith(".svelte"));

/** The markup of a component: no script, no style, no comments. */
const markup = (source: string) =>
  source
    .replace(/<script[\s\S]*?<\/script>/g, "")
    .replace(/<style[\s\S]*?<\/style>/g, "")
    .replace(/<!--[\s\S]*?-->/g, "");

/** Text and literal accessible names written straight into a component's markup. */
function hardText(source: string): string[] {
  const html = markup(source);
  const found: string[] = [];
  // Text between tags that is not a {expression}: strip {…} (nested one level) first.
  const bare = html.replace(/\{[^{}]*(\{[^{}]*\}[^{}]*)*\}/g, "\uE000");
  for (const m of bare.matchAll(/>([^<>\uE000]*\p{L}[^<>\uE000]*)</gu)) {
    const text = m[1]?.trim();
    if (text) found.push(`text "${text}"`);
  }
  for (const m of html.matchAll(/\b(aria-label|title|placeholder|alt)="([^"{}]*\p{L}[^"{}]*)"/gu)) {
    found.push(`${m[1]}="${m[2]}"`);
  }
  return found;
}

describe("interface texts", () => {
  it("UI-10 the detector sees text written in the markup", () => {
    expect(hardText('<p>Bonjour</p><button aria-label="Fermer">{t("x")}</button>')).toEqual([
      'text "Bonjour"',
      'aria-label="Fermer"',
    ]);
    expect(hardText('<p>{t("a")} · {n}</p><span class="x">{label}</span>')).toEqual([]);
    expect(svelte.length).toBeGreaterThan(15);
  });

  it("UI-10 has no text in the markup: everything goes through t()", () => {
    const found = svelte.flatMap(([path, source]) => hardText(source).map((f) => `${path}: ${f}`));
    expect(found).toEqual([]);
  });

  it("UI-10 every literal key used in the code exists in both catalogs", () => {
    const missing: string[] = [];
    for (const [path, source] of Object.entries(sources)) {
      for (const m of source.matchAll(/\bt(?:n)?\(\s*"([A-Za-z][\w.]*)"/g)) {
        const key = m[1] ?? "";
        const known = key in fr || `${key}.other` in fr;
        if (!known || !(key in en || `${key}.other` in en)) missing.push(`${path}: ${key}`);
      }
      for (const m of source.matchAll(/\blabel:\s*"((?:fmt|heading)\.[\w.]+)"/g)) {
        if (!((m[1] ?? "") in fr)) missing.push(`${path}: ${m[1]}`);
      }
    }
    expect(missing).toEqual([]);
  });

  it("UI-10 the catalogs have the same keys, and French says « personne », never « pair »", () => {
    expect(Object.keys(en).sort()).toEqual(Object.keys(fr).sort());
    for (const [key, text] of Object.entries(fr)) {
      expect(text, key).not.toMatch(/\bpairs?\b/i);
    }
    for (const [key, text] of Object.entries(en)) {
      expect(text, key).not.toMatch(/\bpeers?\b/i);
    }
  });

  it("UI-10 every plural has both forms, and each message keeps its placeholders in both languages", () => {
    for (const catalog of [fr, en] as Record<string, string>[]) {
      for (const key of Object.keys(catalog)) {
        if (key.endsWith(".one")) expect(`${key.slice(0, -4)}.other` in catalog, key).toBe(true);
      }
    }
    const holes = (s: string) => [...s.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort();
    for (const [key, text] of Object.entries(fr)) {
      expect(holes((en as Record<string, string>)[key] ?? ""), key).toEqual(holes(text));
    }
  });
});
