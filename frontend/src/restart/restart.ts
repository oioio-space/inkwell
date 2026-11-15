import { t } from "../i18n";

/**
 * The restart page (plan 2026-11-15-pc1a-take-over.md, rule N-9; shared rule N-15): what is left of
 * the application once the page has flushed, closed its repo and navigated away. It holds no repo,
 * adapter, editor or storage handle: it asks Go for the take-over, once, and waits for another
 * process image to answer.
 */

/** What the page needs from outside: Go's two calls, the navigation, and time. */
export interface RestartDeps {
  /** Takes the mark of the page that left after the confirmation (`takeLeaveMark`): true once. */
  confirmed(): boolean;
  /** `Session.TakeOver()`. */
  takeOver(): Promise<{ restarting: boolean }>;
  /** `Session.StartupID()`: the random identity of the process image that answers. */
  startupId(): Promise<string>;
  /** Replaces this page by the one at `url`. */
  load(url: string): void;
  wait(ms: number): Promise<void>;
}

/** The application, loaded again by a new process image. */
export const APP_URL = "./";
/** The application, loaded again by the same image: the take-over did not happen. */
export const REFUSED_URL = "./?takeOver=refused";
/** How often the startup id is asked while Inkwell restarts. */
export const POLL_MS = 250;
/**
 * How long the page says that Inkwell restarts before it gives up (UX-PC1a-2). A restart measured
 * 0.5 to 1.4 s from the confirmation; its longest step is the termination barrier, which fails
 * after 5 s (rule N-14): three times that is past any restart that can still succeed.
 */
export const GIVE_UP_MS = 15_000;

/**
 * Shows « Inkwell redémarre… » in `target`, asks Go for the take-over once, and only when the
 * page that left was confirmed ({@link RestartDeps.confirmed}), then loads the
 * application: at {@link APP_URL} only once an image with another startup id answers (the id is
 * the only proof of a restart: the pid and `/health` stay the same), at {@link REFUSED_URL} when
 * Go says it does not restart, or when the request failed and the same image still answers.
 *
 * Past {@link GIVE_UP_MS} without either, the restart failed (rule N-12: Inkwell closed and could
 * not start again, or it never closed): the page says so and what to do, and goes on asking, so
 * that it loads the application once the person has opened Inkwell again.
 */
export async function startRestart(target: HTMLElement, deps: RestartDeps): Promise<void> {
  // Opened or reloaded without the confirmation: nothing is asked, the application comes back.
  if (!deps.confirmed()) return deps.load(APP_URL);
  const status = document.createElement("p");
  status.setAttribute("role", "status");
  status.textContent = t("foreign.takeOver.busy");
  target.replaceChildren(status);
  let loading = false; // the application is being loaded: this page has nothing more to say
  const load = (url: string) => {
    loading = true;
    deps.load(url);
  };
  const giveUp = () => {
    if (loading) return;
    const failed = document.createElement("p");
    failed.setAttribute("role", "alert");
    failed.textContent = t("change.restartFailed");
    target.replaceChildren(failed);
  };
  void deps.wait(GIVE_UP_MS).then(giveUp);

  // An id that cannot be read is no id: whichever image answers next is then another one.
  const before = await deps.startupId().catch(() => "");
  let restarting: boolean;
  try {
    restarting = (await deps.takeOver()).restarting;
    if (!restarting) return load(REFUSED_URL);
  } catch {
    // Refused, or the reply was lost with the old image: the next answer tells which.
    restarting = false;
  }
  for (;;) {
    await deps.wait(POLL_MS);
    let id: string;
    try {
      id = await deps.startupId();
    } catch {
      continue; // between two images nothing answers
    }
    if (id !== before) return load(APP_URL);
    if (!restarting) return load(REFUSED_URL);
  }
}
