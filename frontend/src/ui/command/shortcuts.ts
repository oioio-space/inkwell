/**
 * What the shortcuts window lists (spec §4.26, rule P-9), from the registry and keytables.ts only:
 * one section per category, a row per command with a chord in force on this system (every chord
 * of it, as the system shows it), then « Écrire », the editor's and the grid's labelled keys. A
 * chord the system or the editor handles, and one of hers, says so.
 */
import {
  activeOf,
  commandRegistry,
  commandSystem,
  labelOf,
  textOf,
} from "../../extensions/commands";
import { renderChord } from "../../extensions/keys";
import { OWNED_KEYS } from "../../extensions/keytables";
import { CATEGORIES } from "../../extensions/manifest";
import { lang, type MessageKey, t } from "../../i18n";

export interface ShortcutRow {
  readonly id: string;
  readonly label: string;
  readonly keywords: readonly string[];
  /** As this system shows them, each once. */
  readonly chords: readonly string[];
  /** « géré par le système », « géré par l'éditeur », « personnalisé ». */
  readonly marks: readonly string[];
}

export interface ShortcutSection {
  readonly id: string;
  readonly title: string;
  readonly rows: readonly ShortcutRow[];
}

export function shortcutSections(): ShortcutSection[] {
  const r = commandRegistry();
  const system = commandSystem();
  const show = (chord: string) => renderChord(chord, system, lang);
  const sections: ShortcutSection[] = CATEGORIES.map((category) => ({
    id: category,
    title: t(`cmd.category.${category}` as MessageKey),
    rows: (r?.commands ?? [])
      .filter((c) => c.category === category)
      .flatMap((c): ShortcutRow[] => {
        const active = activeOf(c.id);
        if (active.length === 0) return [];
        const marks = new Set<string>();
        for (const a of active) {
          if (a.delegated === "system") marks.add(t("shortcuts.system"));
          if (a.delegated === "widget") marks.add(t("shortcuts.editor"));
          if (a.mine) marks.add(t("shortcuts.mine"));
        }
        return [
          {
            id: c.id,
            label: labelOf(c),
            keywords: c.keywords.map((k) => textOf(c, k)),
            chords: [...new Set(active.map((a) => show(a.chord)))],
            marks: [...marks],
          },
        ];
      }),
  }));
  // The writing keys: one row per label (the editor's and the grid's « Tout sélectionner » merge).
  const writing = new Map<string, Set<string>>();
  for (const k of OWNED_KEYS) {
    if (k.label === undefined || k[system].length === 0) continue;
    const chords = writing.get(k.label) ?? new Set<string>();
    for (const chord of k[system]) chords.add(show(chord));
    writing.set(k.label, chords);
  }
  sections.push({
    id: "write",
    title: t("shortcuts.write"),
    rows: [...writing].map(([key, chords]) => ({
      id: key,
      label: t(key as MessageKey),
      keywords: [],
      chords: [...chords],
      marks: [t("shortcuts.editor")],
    })),
  });
  return sections.filter((s) => s.rows.length > 0);
}
