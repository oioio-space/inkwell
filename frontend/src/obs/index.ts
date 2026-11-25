/**
 * The application's statistics session: one for the page, bound to Go's `StatsService`, started
 * by the first built-in that counts something (ext/activity). Only ext/activity imports this
 * module; it reaches no network, transport or CRDT code (gate OBS1 `local-only`).
 */
import { StatsService } from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { type MetricInfo, StatsSession } from "./session";

export type { MetricInfo, StatsBackend } from "./session";
export { FLUSH_EVERY, StatsSession } from "./session";

let session: StatsSession | undefined;

/**
 * The page's session, started on the first call: it declares `metrics` to Go, those `merge`
 * keeps once Go's own descriptors are merged in.
 */
export function statsSession(
  metrics: readonly MetricInfo[],
  merge?: (go: readonly unknown[]) => readonly MetricInfo[],
): StatsSession {
  if (!session) {
    session = new StatsSession(StatsService, metrics, {
      page: window,
      ...(merge ? { merge } : {}),
    });
    session.start();
  }
  return session;
}
