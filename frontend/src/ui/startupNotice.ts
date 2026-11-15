import { Session } from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { t } from "../i18n";
import { notice } from "./notice.svelte";

/** The sentences a start can owe to the person, by catalogue key, with how they are announced. */
const SENTENCES = {
  "foreign.takeOver.done": "status",
  "foreign.takeOver.failed": "alert",
} as const;

/**
 * Says, once, how the take-over that restarted Inkwell ended (PC1a, rules N-9 and N-12), then
 * gives the focus to the text. Go chooses the sentence from the member list this process image
 * loaded (`Session.StartupNotice`, answered once); when Go has nothing to say and the restart page
 * came back without a restart (`?takeOver=refused`), nothing changed and the page says so. The
 * mark is removed from the address, so reloading the page does not say it again.
 *
 * The sentence does not wait for the document: after a failed request the bridge may be closed for
 * good (rule N-16) and the document never opens.
 *
 * @param focus gives the focus to the text.
 * @param ready ends once the document is open: the focus waits for it, the sentence does not.
 */
export async function showStartupNotice(
  focus: () => void,
  ready: Promise<unknown> = Promise.resolve(),
  url: URL = new URL(location.href),
  replace: (clean: string) => void = (clean) => history.replaceState(null, "", clean),
): Promise<void> {
  const refused = url.searchParams.get("takeOver") === "refused";
  if (refused) {
    url.searchParams.delete("takeOver");
    replace(url.pathname + url.search + url.hash);
  }
  const said = await Session.StartupNotice().catch(() => "");
  const key = said || (refused ? "foreign.takeOver.failed" : "");
  if (!Object.hasOwn(SENTENCES, key)) return;
  const known = key as keyof typeof SENTENCES;
  notice.queue(t(known), SENTENCES[known]);
  await ready.then(focus, () => undefined);
}
