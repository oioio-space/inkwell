import en from "./en.json";
import fr from "./fr.json";

export type MessageKey = keyof typeof fr;
const catalogs: Record<string, Record<MessageKey, string>> = { fr, en };

/** The UI language: French by default, English when the system asks for it. */
export const lang: "fr" | "en" = navigator.language.toLowerCase().startsWith("en") ? "en" : "fr";

const plurals = new Intl.PluralRules(lang);
const numbers = new Intl.NumberFormat(lang);

/** Returns the message for key, with {placeholders} filled from vars. */
export function t(key: MessageKey, vars: Record<string, string | number> = {}): string {
  const template = catalogs[lang]?.[key] ?? fr[key];
  return template.replace(/\{(\w+)\}/g, (_, name: string) => String(vars[name] ?? `{${name}}`));
}

const megabytes = new Intl.NumberFormat(lang, { maximumFractionDigits: 1 });

/** A text of this many characters, as the "Mo" a message shows (1 Mo = 2^20 characters). */
export function megabytesOf(chars: number): string {
  return megabytes.format(chars / 2 ** 20);
}

/** The right plural of a message: `key.one` or `key.other`, with {count} filled in. */
export function tn(
  base: string,
  count: number,
  vars: Record<string, string | number> = {},
): string {
  const form = plurals.select(count) === "one" ? "one" : "other";
  return t(`${base}.${form}` as MessageKey, { ...vars, count: numbers.format(count) });
}
