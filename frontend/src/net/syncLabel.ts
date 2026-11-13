import { t } from "../i18n";
import type { GlobalSyncState } from "./syncIndicator";

type Shown = Exclude<GlobalSyncState, "offline"> | "alone" | "connecting" | "suspended";

/**
 * The wording of the header and status bar for a state; « Lien lent » is added to a state that is
 * not synced after 2 s of pending age (D-SI7).
 */
export function statusLabel(state: Shown, sending: number, slow: boolean): string {
  const text =
    state === "sending" && sending < 1
      ? t("status.sending.none")
      : t(`status.${state}`, { n: sending });
  const pending = state === "sending" || state === "behind" || state === "checking";
  return slow && pending ? `${text} · ${t("status.slow")}` : text;
}
