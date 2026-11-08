import { LOSS_UNKNOWN, type PeerLink } from "./envelope";

/** A link as the people popover and the e2e specs read it (plan section 7, contract v1). */
export type LinkView = Omit<PeerLink, "quality" | "rttMs" | "lossPermille" | "lossScope"> & {
  /** Null while nothing was measured. */
  quality: Exclude<PeerLink["quality"], "unknown"> | null;
  /** Null while no round trip was measured. */
  rttMs: number | null;
  /** Percent, null while too few packets were sent to tell. */
  lossPct: number | null;
  lossScope: "path" | "firstHop" | null;
  /** Sync messages written to this peer since this device started (webview count, plan section 7). */
  syncSent: number;
  /** Sync messages accepted from this peer since this device started. */
  syncRecv: number;
};

/** Converts the wire units to the ones of {@link LinkView}. */
export function toLinkView(l: PeerLink): LinkView {
  const { quality, rttMs, lossPermille, lossScope, ...rest } = l;
  const known = lossPermille !== LOSS_UNKNOWN;
  return {
    ...rest,
    syncSent: 0,
    syncRecv: 0,
    quality: quality === "unknown" ? null : quality,
    rttMs: rttMs === 0 ? null : rttMs,
    lossPct: known ? lossPermille / 10 : null,
    lossScope: known && lossScope !== "none" ? lossScope : null,
  };
}

/** The state of the link to each peer, as Go last reported it (Links, kind 20). */
class Links {
  list = $state.raw<readonly LinkView[]>([]);
  /** When the list was received (ms): Go resends only on a change, so ages are as of then. */
  #at = 0;
  #byPeer = new Map<string, LinkView>();
  /** Where the sync message counts come from (the adapter); they change between two reports. */
  syncCounts: (peer: string) => { sent: number; recv: number } = () => ({ sent: 0, recv: 0 });

  /** Replaces the whole list: the notification carries the last value only. */
  set(links: readonly PeerLink[], now = Date.now()): void {
    this.list = links.map(toLinkView);
    this.#byPeer = new Map(this.list.map((l) => [l.peer, l]));
    this.#at = now;
  }

  /** The list with the ages brought up to now. */
  view(now = Date.now()): LinkView[] {
    const dt = Math.max(0, now - this.#at);
    return this.list.map((l) => ({
      ...l,
      syncSent: this.syncCounts(l.peer).sent,
      syncRecv: this.syncCounts(l.peer).recv,
      pathAgeMs: l.pathAgeMs + dt,
      rttAgeMs: l.rttMs === null ? l.rttAgeMs : l.rttAgeMs + dt,
      upMs: l.upMs > 0 ? l.upMs + dt : 0,
      oldestPendingMs: l.oldestPendingMs > 0 ? l.oldestPendingMs + dt : 0,
    }));
  }

  /** The link to peer, if Go reported one. */
  of(peer: string): LinkView | undefined {
    return this.#byPeer.get(peer);
  }

  clear(): void {
    this.set([]);
  }
}

export const links = new Links();
