import { next as A } from "@automerge/automerge";
import { type ActionKind, type Heads, parseAction } from "./types";

/** One change of the document, in the order this replica applied them (causal order). */
export interface Step {
  hash: string;
  actor: string;
  /** Milliseconds, as the author's device said (a claim: never used to order anything). */
  time: number;
  deps: Heads;
  message: string | null;
}

/** Several steps of one author with no pause over 10 minutes, or one action of Inkwell's own. */
export interface Session {
  /** The hash of the first step: stable while the session grows. */
  id: string;
  actor: string;
  /** First and last step, inclusive. */
  from: number;
  to: number;
  /** Times, on the author's clock. */
  start: number;
  end: number;
  count: number;
  /** The heads before the session and after it. */
  before: Heads;
  after: Heads;
  /** Set for a revert, an undo, a merge: an entry of its own. */
  action?: { kind: ActionKind; label: string };
  /** Nothing but a device saying who it is: no entry to show. */
  claimOnly: boolean;
  /** The device's clock disagrees with causality (X-20): the time is shown as its device's say. */
  skewed: boolean;
}

/** The gap of quiet that ends a session. */
export const SESSION_GAP_MS = 10 * 60_000;
/** A clock this far ahead of ours is not believed. */
const FUTURE_MS = 5 * 60_000;
/** A step this much earlier than what it follows is out of order. */
const BACKWARD_MS = 60_000;

/** The steps of a document, cheap: only the metadata of each change is read. */
export function stepsOf(doc: A.Doc<unknown>): Step[] {
  const meta = A.getChangesMetaSince(doc, []);
  const steps: Step[] = new Array(meta.length);
  for (let i = 0; i < meta.length; i++) {
    const c = meta[i] as A.ChangeMetadata;
    steps[i] = {
      hash: c.hash,
      actor: c.actor,
      time: c.time * 1000,
      deps: c.deps,
      message: c.message,
    };
  }
  return steps;
}

/**
 * Groups steps into sessions: one author, no pause over ten minutes, and an action of Inkwell's
 * (revert, undo, merge) always on its own. The order is the document's causal order, so a device
 * with a wrong clock cannot reorder anything.
 */
export function sessionsOf(
  steps: readonly Step[],
  now = Date.now(),
  personOf: (actor: string) => string = (actor) => actor,
): Session[] {
  const out: Session[] = [];
  const timeOf = new Map<string, number>();
  // The heads after each step, kept as a set; only the session boundaries are written out.
  const heads = new Set<string>();
  const snapshot = (): Heads => [...heads].sort();
  let cur: Session | undefined;
  let curAction = false;
  for (let i = 0; i < steps.length; i++) {
    const s = steps[i] as Step;
    const parsed = parseAction(s.message);
    // Saying who one is changes no text: it is part of whatever the person does, and an entry of
    // its own only when it is all they did (then the entry is hidden).
    const claim = parsed?.kind === "claim";
    const action = claim ? undefined : parsed;
    const skew =
      s.time > now + FUTURE_MS || s.deps.some((d) => (timeOf.get(d) ?? 0) - s.time > BACKWARD_MS);
    timeOf.set(s.hash, s.time);
    const joins =
      cur !== undefined &&
      !action &&
      !curAction &&
      personOf(cur.actor) === personOf(s.actor) &&
      Math.max(0, s.time - cur.end) <= SESSION_GAP_MS;
    if (joins && cur) {
      cur.to = i;
      cur.end = Math.max(cur.end, s.time);
      cur.count++;
      cur.skewed ||= skew;
      cur.claimOnly &&= claim;
    } else {
      const edge = snapshot();
      if (cur) cur.after = edge;
      cur = {
        id: s.hash,
        actor: s.actor,
        from: i,
        to: i,
        start: s.time,
        end: s.time,
        count: 1,
        before: edge,
        after: edge,
        skewed: skew,
        claimOnly: claim,
        ...(action ? { action } : {}),
      };
      curAction = action !== undefined;
      out.push(cur);
    }
    for (const d of s.deps) heads.delete(d);
    heads.add(s.hash);
  }
  if (cur) cur.after = snapshot();
  return out;
}

/**
 * Which person an actor is, for the grouping: the device it says it is (`meta.actors`), so a
 * person who reloads the page (a new actor each time) stays in the same session.
 */
export function personOfDoc(doc: A.Doc<unknown>): (actor: string) => string {
  const claims =
    (doc as { meta?: { actors?: Record<string, { peer: string }> } }).meta?.actors ?? {};
  return (actor) => claims[actor]?.peer ?? actor;
}

/** The sessions of a document, oldest first. */
export function history(doc: A.Doc<unknown>, now = Date.now()): Session[] {
  return sessionsOf(stepsOf(doc), now, personOfDoc(doc));
}

/** The text of the document at some heads (empty heads: the empty document). */
export function textAt(doc: A.Doc<{ text: string }>, heads: Heads): string {
  return heads.length === 0 ? "" : A.view(doc, heads).text;
}

/** The set of steps that are in the past of `heads` (inclusive). */
export function pastOf(steps: readonly Step[], heads: Heads): Set<string> {
  const byHash = new Map(steps.map((s) => [s.hash, s] as const));
  const seen = new Set<string>();
  const todo = [...heads];
  for (let h = todo.pop(); h !== undefined; h = todo.pop()) {
    if (seen.has(h)) continue;
    seen.add(h);
    const s = byHash.get(h);
    if (s) todo.push(...s.deps);
  }
  return seen;
}

/** How many sessions, from the oldest, lie entirely in the past of `heads`. */
export function sessionsBefore(
  sessions: readonly Session[],
  steps: readonly Step[],
  heads: Heads,
): number {
  const past = pastOf(steps, heads);
  let n = 0;
  while (n < sessions.length) {
    const s = sessions[n] as Session;
    let inside = true;
    for (let i = s.from; i <= s.to && inside; i++) inside = past.has((steps[i] as Step).hash);
    if (!inside) break;
    n++;
  }
  return n;
}
