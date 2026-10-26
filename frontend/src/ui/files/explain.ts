import type { MessageKey } from "../../i18n";
import { lang, t } from "../../i18n";
import type { OpError } from "../../project/ops";

const NAME_REASONS = new Set([
  "empty",
  "too_long",
  "bad_char",
  "dot",
  "trailing",
  "device",
  "reserved",
]);

/** Why an operation was refused, in words (FM-03): the rule that was broken, never a code. */
export function explain(error: OpError): string {
  if (error.code === "invalid_name") {
    const reason = error.reason ?? "";
    return t(
      (NAME_REASONS.has(reason)
        ? `files.error.name.${reason}`
        : "files.error.name.other") as MessageKey,
    );
  }
  return t(`files.error.${error.code}` as MessageKey, { name: error.name ?? "" });
}

const bytes = new Intl.NumberFormat(lang, { maximumFractionDigits: 1 });

/** A size in bytes the way the person reads it: "3,2 Mo" in French, "3.2 MB" in English. */
export function formatSize(n: number): string {
  const units = lang === "fr" ? ["o", "Ko", "Mo", "Go", "To"] : ["B", "KB", "MB", "GB", "TB"];
  let value = n;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${i === 0 ? String(value) : bytes.format(value)} ${units[i]}`;
}
