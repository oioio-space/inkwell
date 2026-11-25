/**
 * The palette's commands, folded once per registry and language (rule P-1): a built-in's label
 * and keywords are i18n keys read in that language, another extension's are literal and match in
 * every language. The palette's own command is not listed.
 */
import type { Registry } from "../../extensions";
import type { Command } from "../../extensions/registry";
import { type MessageKey, textIn } from "../../i18n";
import { type Prepared, prepare } from "./rank";

export type Language = "fr" | "en";

const PALETTE = "app.palette";
const cache = new WeakMap<Registry, Map<Language, Prepared<Command>>>();

const textOf = (c: Command, text: string, language: Language) =>
  c.origin === "builtin" ? textIn(language, text as MessageKey) : text;

export function paletteEntries(r: Registry, language: Language): Prepared<Command> {
  const byLanguage = cache.get(r) ?? new Map<Language, Prepared<Command>>();
  cache.set(r, byLanguage);
  let p = byLanguage.get(language);
  if (!p) {
    p = prepare(
      r.commands.filter((c) => c.id !== PALETTE),
      (c) => textOf(c, c.label, language),
      (c) => c.keywords.map((k) => textOf(c, k, language)),
      language,
    );
    byLanguage.set(language, p);
  }
  return p;
}
