import { Browser } from "@wailsio/runtime";

/** Schemes the system may open: web pages and a new mail, never a file or an app. */
const SCHEMES = new Set(["http:", "https:", "mailto:"]);

/**
 * openExternal hands url to the system (default browser or mail client) only when the person
 * asked for it: event must be a click the browser itself dispatched (isTrusted) and still being
 * handled (eventPhase), so neither a script nor document or peer content can open anything, and
 * url must be http, https or mailto. It is the only module allowed to reach Browser.OpenURL
 * (external.test.ts). Returns whether the link was handed over.
 */
export function openExternal(
  event: Event,
  url: string,
  open: (url: string) => unknown = Browser.OpenURL,
): boolean {
  if (!event.isTrusted || event.type !== "click" || event.eventPhase === Event.NONE) return false;
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    return false;
  }
  if (!SCHEMES.has(parsed.protocol)) return false;
  void open(parsed.href);
  return true;
}
