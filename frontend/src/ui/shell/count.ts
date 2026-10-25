import { type CountUnit, formatCount } from "../../editor/stats/stats";
import { lang, t } from "../../i18n";

const english = new Intl.NumberFormat("en");

/** "1 284 mots" in French (stats' own formatter), "1,284 words" in English. */
export function count(n: number, unit: CountUnit): string {
  if (lang === "fr") return formatCount(n, unit);
  return `${english.format(n)} ${t(`count.${unit}`)}${n === 1 ? "" : "s"}`;
}
