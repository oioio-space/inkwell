import { t } from "../i18n";
import type { GoStorageAdapter } from "../store/GoStorageAdapter";
import { notice } from "../ui/notice.svelte";

/** The longest delay a timer takes (2^31 - 1 ms): the notice of a stopped page stays until closed. */
const STAYS = 2 ** 31 - 1;

/**
 * Says once that a newer page took the store (ADR 0020, RT1 rule C-2): this page has stopped, and every request it
 * makes from now on fails without reconnecting. Returns an unsubscribe.
 */
export function watchSuperseded(storage: GoStorageAdapter): () => void {
  return storage.onSuperseded(() => notice.show(t("store.openElsewhere"), "alert", STAYS));
}
