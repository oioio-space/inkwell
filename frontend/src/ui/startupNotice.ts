import { Session } from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { t } from "../i18n";
import { notice } from "./notice.svelte";

/** The sentences a start can owe to the person, by catalogue key, with how they are announced. */
const SENTENCES = {
  "foreign.takeOver.done": "status",
  "foreign.takeOver.failed": "alert",
  "join.failed": "alert",
  "join.return.failed": "alert",
} as const;

/** What the restart page adds to the address when it comes back without a restart, and its sentence. */
const REFUSED = {
  takeOver: "foreign.takeOver.failed",
  join: "join.failed",
  return: "join.return.failed",
} as const;

/**
 * Says, once, how the project change that restarted Inkwell ended (PC1, rules N-9, N-12 and
 * J-4), then gives the focus to the text. Go chooses the sentence from what this process image
 * loaded (`Session.StartupNotice`, answered once); when Go has nothing to say and the restart page
 * came back without a restart (`?takeOver=refused`, `?join=refused`, `?return=refused`), nothing changed and the page
 * says so. The mark is removed from the address, so reloading the page does not say it again.
 * (A join that ended well is said by the page that completed it: `Workspace.openStart`.)
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
  let refused = "";
  for (const [mark, sentence] of Object.entries(REFUSED)) {
    if (url.searchParams.get(mark) !== "refused") continue;
    refused ||= sentence;
    url.searchParams.delete(mark);
    replace(url.pathname + url.search + url.hash);
  }
  const said = await Session.StartupNotice().catch(() => "");
  const key = said || refused;
  if (!Object.hasOwn(SENTENCES, key)) return;
  const known = key as keyof typeof SENTENCES;
  notice.queue(t(known), SENTENCES[known]);
  await ready.then(focus, () => undefined);
}
