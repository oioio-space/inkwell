import { lang, t } from "../i18n";

const weekday = new Intl.DateTimeFormat(lang, { weekday: "long" });
const dayMonth = new Intl.DateTimeFormat(lang, { day: "numeric", month: "short" });
const dayMonthYear = new Intl.DateTimeFormat(lang, {
  day: "numeric",
  month: "short",
  year: "numeric",
});

const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();

/** The time of day, 14:10 (no leading zero on the hour: 9:30). */
export function clock(ms: number): string {
  const d = new Date(ms);
  return `${d.getHours()}:${String(d.getMinutes()).padStart(2, "0")}`;
}

/**
 * A moment in plain words: « aujourd'hui 14:10 », « hier 17:42 », « lundi 9:30 » within the last
 * week, then « 12 oct. 14:10 » (with the year when it is not this one).
 */
export function when(ms: number, now = Date.now()): string {
  const days = Math.round((startOfDay(new Date(now)) - startOfDay(new Date(ms))) / 86_400_000);
  const time = clock(ms);
  if (days === 0) return t("when.today", { time });
  if (days === 1) return t("when.yesterday", { time });
  if (days > 1 && days < 7) return `${weekday.format(ms)} ${time}`;
  const sameYear = new Date(ms).getFullYear() === new Date(now).getFullYear();
  return `${(sameYear ? dayMonth : dayMonthYear).format(ms)} ${time}`;
}

/** The day alone, for headings: « aujourd'hui », « hier », « lundi », « 12 oct. ». */
export function whenDay(ms: number, now = Date.now()): string {
  const days = Math.round((startOfDay(new Date(now)) - startOfDay(new Date(ms))) / 86_400_000);
  if (days === 0) return t("when.today.day");
  if (days === 1) return t("when.yesterday.day");
  if (days > 1 && days < 7) return weekday.format(ms);
  return (
    new Date(ms).getFullYear() === new Date(now).getFullYear() ? dayMonth : dayMonthYear
  ).format(ms);
}
