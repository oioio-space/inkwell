import { type MessageKey, t } from "../i18n";
/** Ordered and deterministic: the original message stays available as plain text. */
export function explain(message: string): { summary: string; action: string } {
  const patterns: readonly [RegExp, MessageKey, MessageKey][] = [
    [/unknown variable/i, "writing.unknown", "writing.import"],
    [/unclosed|expected closing/i, "writing.unclosed", "writing.close"],
    [/file not found/i, "writing.missing", "writing.path"],
    [/expected comma/i, "writing.comma", "writing.commas"],
    [/unknown font/i, "writing.font", "writing.fontFix"],
    [/expected.*found/i, "writing.expected", "writing.signs"],
  ];
  for (const [pattern, summary, action] of patterns)
    if (pattern.test(message)) return { summary: t(summary), action: t(action) };
  return { summary: message.split("\n")[0] ?? message, action: t("writing.details") };
}
