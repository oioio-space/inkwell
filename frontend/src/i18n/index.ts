import en from "./en.json";
import fr from "./fr.json";

export type MessageKey = keyof typeof fr;
const catalogs: Record<string, Record<MessageKey, string>> = { fr, en };

/** The UI language: French by default, English when the system asks for it. */
export const lang: "fr" | "en" = navigator.language.toLowerCase().startsWith("en") ? "en" : "fr";

/** Returns the message for key, with {placeholders} filled from vars. */
export function t(key: MessageKey, vars: Record<string, string | number> = {}): string {
  const template = catalogs[lang]?.[key] ?? fr[key];
  return template.replace(/\{(\w+)\}/g, (_, name: string) => String(vars[name] ?? `{${name}}`));
}
