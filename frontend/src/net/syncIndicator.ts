import { next as A } from "@automerge/automerge";

/**
 * The honest sync indicator (W33, block SI1). Spec: docs/plans/2026-11-08-netsim-impaired-plan.md,
 * "Honest sync indicator"; model: docs/protocol/tla/SyncIndicator.tla. A pure function of what this
 * device can observe: who is announced, the heads of the last sync message heard from each peer for
 * each document, its own documents, and the Links notification. Never of the peer count.
 */

/** The state of one peer, or of the whole project, as this device can tell. */
export type PeerSyncState = "offline" | "checking" | "behind" | "sending" | "synced";
export type GlobalSyncState = PeerSyncState;

/** How long the true state must have been « synced », without a break, before the header says so. */
export const SyncedHoldMs = 1000;
/** The pending age of the oldest message after which the link is called slow (ADR 0028 D8). */
export const SlowAfterMs = 2000;

/** A document shared with a peer: my copy (undefined while I do not hold it yet) and their last heads. */
export interface ObservedDoc {
  mine: A.Doc<unknown> | undefined;
  /** The heads of the last sync message heard from the peer; null while none was heard. */
  theirs: readonly string[] | null;
}

/** Everything the indicator may read about one peer. */
export interface Observed {
  announced: boolean;
  docs: ReadonlyMap<string, ObservedDoc>;
  /** From the Links notification: informational, it never decides « Synchronisé ». */
  pendingMsgs: number;
  oldestPendingMs: number;
}

const sameHeads = (a: readonly string[], b: readonly string[]) =>
  a.length === b.length && [...a].sort().join() === [...b].sort().join();

/** The state of one peer: the first that holds of offline, checking, behind, sending. */
export function peerSyncState(o: Observed): PeerSyncState {
  if (!o.announced) return "offline";
  const docs = [...o.docs.values()];
  if (docs.length === 0 || docs.some((d) => d.mine === undefined || d.theirs === null))
    return "checking";
  const known = docs as { mine: A.Doc<unknown>; theirs: readonly string[] }[];
  if (known.some((d) => !A.hasHeads(d.mine, [...d.theirs]))) return "behind";
  if (known.some((d) => !sameHeads(A.getHeads(d.mine), d.theirs))) return "sending";
  return "synced";
}

/** The worst state among the connected peers (behind > sending > checking); never synced with none. */
export function globalSyncState(states: readonly PeerSyncState[]): GlobalSyncState {
  const connected = states.filter((s) => s !== "offline");
  if (connected.length === 0) return "offline";
  for (const worst of ["behind", "sending", "checking"] as const) {
    if (connected.includes(worst)) return worst;
  }
  return "synced";
}

/** n of « Envoi en cours (n) »: the connected people who do not have all my changes yet (D-SI1). */
export function sendingCount(observed: Iterable<Observed>): number {
  return [...observed].filter((o) => peerSyncState(o) === "sending").length;
}

/** Whether a connected link has held a message for more than {@link SlowAfterMs} (D-SI7). */
export function isSlow(observed: readonly Observed[]): boolean {
  return observed.some((o) => o.announced && o.oldestPendingMs > SlowAfterMs);
}

/** How long a label stays on screen before another non-synced label replaces it (DS-3). */
export const MinShowMs = 1000;

/** What the header shows, and since when. */
export interface Display {
  label: GlobalSyncState;
  /** n of « Envoi en cours (n) », frozen with the label; it only grows while the label stays. */
  n: number;
  /** When the label was put on screen (ms). */
  since: number;
  /** When the true state became synced, without a break; null if it is not synced. */
  syncedSince: number | null;
}

export const initialDisplay: Display = {
  label: "offline",
  n: 0,
  since: Number.NEGATIVE_INFINITY,
  syncedSince: null,
};

/**
 * The display rule DS-1..DS-5 (spec amendment SI1-B), pure: (state, truth, n, now) gives the new
 * state. « Synchronisé » shows once the truth has been synced for SyncedHoldMs and leaves at once;
 * every other change of label waits until the label has been shown MinShowMs, and the latest truth
 * wins; n is taken with its label and only grows while the label stays.
 */
export function displayStep(
  d: Display,
  truth: GlobalSyncState,
  count: number,
  now: number,
): Display {
  const n = truth === "sending" ? count : 0; // only « Envoi en cours » has a count
  const syncedSince = truth === "synced" ? (d.syncedSince ?? now) : null;
  const next = { ...d, syncedSince };
  if (truth !== "synced") {
    if (d.label === "synced" || truth === d.label || now - d.since >= MinShowMs) {
      return truth === d.label
        ? { ...next, n: Math.max(d.n, n) }
        : { ...next, label: truth, n, since: now };
    }
    return next;
  }
  if (d.label === "synced") return next;
  const held = now - (syncedSince as number) >= SyncedHoldMs;
  if (held && now - d.since >= MinShowMs) return { ...next, label: "synced", n: 0, since: now };
  return next;
}

/** When {@link displayStep} next has something to do if nothing else changes (ms), if ever. */
function nextDue(d: Display, truth: GlobalSyncState, now: number): number | undefined {
  const due: number[] = [];
  if (truth !== d.label) due.push(d.since + MinShowMs);
  if (truth === "synced" && d.label !== "synced" && d.syncedSince !== null)
    due.push(d.syncedSince + SyncedHoldMs);
  return due.filter((t) => t > now).sort((a, b) => a - b)[0];
}

/** Runs {@link displayStep} against the clock, telling `onChange` whenever the label or n moves. */
export class SyncDisplay {
  #d = initialDisplay;
  #timer: ReturnType<typeof setTimeout> | undefined;
  #truth: GlobalSyncState = "offline";
  #n = 0;

  constructor(private readonly onChange: (label: GlobalSyncState, n: number) => void) {}

  get shown(): GlobalSyncState {
    return this.#d.label;
  }

  get shownN(): number {
    return this.#d.n;
  }

  /** Feeds the true state and n (the connected people who lack some of my changes). */
  update(truth: GlobalSyncState, n = 0): void {
    this.#truth = truth;
    this.#n = n;
    this.#run();
  }

  /** Starts over (the workspace closes): nothing pending, nothing shown. */
  reset(): void {
    clearTimeout(this.#timer);
    this.#timer = undefined;
    this.#d = initialDisplay;
    this.#truth = "offline";
    this.#n = 0;
    this.onChange("offline", 0);
  }

  #run(): void {
    clearTimeout(this.#timer);
    const now = Date.now();
    const before = this.#d;
    this.#d = displayStep(before, this.#truth, this.#n, now);
    if (this.#d.label !== before.label || this.#d.n !== before.n)
      this.onChange(this.#d.label, this.#d.n);
    const due = nextDue(this.#d, this.#truth, now);
    this.#timer = due === undefined ? undefined : setTimeout(() => this.#run(), due - now);
  }
}

/** What the adapter has seen of its peers. */
export interface HeadsSource {
  announcedPeers(): readonly string[];
  /** Per document shared with the peer: the heads of its last sync message, null while none came. */
  heardHeads(peer: string): ReadonlyMap<string, readonly string[] | null>;
}

/** Gathers the observables of every announced peer. */
export function observePeers(
  source: HeadsSource,
  docOf: (id: string) => A.Doc<unknown> | undefined,
  linkOf: (peer: string) => { pendingMsgs: number; oldestPendingMs: number } | undefined,
  inScope: (id: string) => boolean = () => true,
): Map<string, Observed> {
  const out = new Map<string, Observed>();
  for (const peer of source.announcedPeers()) {
    const docs = new Map<string, ObservedDoc>();
    // Only the documents of the current admitted scope are shared with the peer (SI1-A).
    for (const [id, theirs] of source.heardHeads(peer)) {
      if (inScope(id)) docs.set(id, { mine: docOf(id), theirs });
    }
    const link = linkOf(peer);
    out.set(peer, {
      announced: true,
      docs,
      pendingMsgs: link?.pendingMsgs ?? 0,
      oldestPendingMs: link?.oldestPendingMs ?? 0,
    });
  }
  return out;
}
