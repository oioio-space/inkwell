/**
 * PERF-PALETTE's fixture (EX0b2-E2, architecture §15.4): 1 000 data-only commands in eight
 * extensions of another origin, added to the registry in e2e builds only (`VITE_E2E_OBSERVE`).
 * Criterion `chunks` checks that no production file holds it.
 */
import type { ManifestSource } from "./manifest";

/** Each extension's id with its dot: the prefix of its commands. A manifest holds 128 at most. */
const PREFIXES = [
  "e2e-many.",
  "e2e-many-b.",
  "e2e-many-c.",
  "e2e-many-d.",
  "e2e-many-e.",
  "e2e-many-f.",
  "e2e-many-g.",
  "e2e-many-h.",
];
const PER_EXTENSION = 125;
const WORDS = [
  "afficher",
  "panneau",
  "page",
  "mise",
  "en",
  "forme",
  "de",
  "la",
  "gauche",
  "droite",
  "texte",
  "tableau",
  "colonne",
  "ligne",
  "aperçu",
  "document",
  "exporter",
  "insérer",
  "réglage",
  "partager",
];
const CATEGORIES = ["document", "edit", "format", "view", "navigate", "present", "app"];

export function manySources(): ManifestSource[] {
  let seed = 7;
  const next = () => {
    seed = (Math.imul(seed, 1103515245) + 12345) >>> 0;
    return seed >>> 16;
  };
  return PREFIXES.map((prefix, e) => {
    const commands = Array.from({ length: PER_EXTENSION }, (_, i) => {
      const n = 2 + (next() % 4);
      const words = Array.from({ length: n }, () => WORDS[next() % WORDS.length] as string);
      const label = words.join(" ");
      return {
        id: `${prefix}c${i}`,
        label: label.charAt(0).toUpperCase() + label.slice(1),
        keywords: [WORDS[next() % WORDS.length]],
        category: CATEGORIES[(e * PER_EXTENSION + i) % CATEGORIES.length],
      };
    });
    const id = prefix.slice(0, -1);
    return {
      origin: "plugin" as const,
      folder: id,
      text: JSON.stringify({ id, version: "1.0.0", contributes: { commands } }),
      files: new Set<string>(),
    };
  });
}
