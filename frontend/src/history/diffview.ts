import type { Session } from "./log";
import { type Blame, firstSpan, type Span } from "./track";
import { diffWithMoves, type Op, type OpKind } from "./words";

/** A stretch of a diff, with whoever wrote (or removed) it. */
export interface Frag {
  kind: OpKind;
  text: string;
  inner?: [number, number];
  moved?: number;
  /** The actor that wrote an added stretch or removed a deleted one; undefined when not known. */
  actor?: string;
  /** The session (index in the analysis) it comes from, to say when. */
  s?: number;
  /** Offset in the new text for `eq` and `add` stretches. */
  b: number;
}

/** A run of changes between two unchanged stretches: what F7 steps through. */
export interface Hunk {
  /** Indexes in the frags list, first and last. */
  from: number;
  to: number;
  actors: string[];
}

const normal = (s: string) => s.replace(/\s+/g, " ").trim();

/**
 * Puts a name on each changed stretch. An added stretch is split along the sessions that wrote
 * it (two people in one paragraph each keep their words, DIF-04); a removed one is matched to the
 * removal that took that text away. A stretch nobody can be found for stays without an author:
 * better than a wrong one.
 */
export function attribute(ops: readonly Op[], blame: Blame | undefined): Frag[] {
  const out: Frag[] = [];
  const actorOf = (s: number) => blame?.sessions[s]?.actor;
  for (const op of ops) {
    const base: Frag = { kind: op.kind, text: op.text, b: op.b };
    if (op.inner) base.inner = op.inner;
    if (op.moved !== undefined) base.moved = op.moved;
    if (op.kind === "eq" || !blame) {
      out.push(base);
      continue;
    }
    if (op.kind === "add") {
      let at = op.b;
      const first = out.length;
      const end = op.b + op.text.length;
      for (let i = firstSpan(blame.spans, at); i < blame.spans.length; i++) {
        const sp = blame.spans[i] as Span;
        if (sp.from >= end) break;
        const from = Math.max(sp.from, at);
        const to = Math.min(sp.to, end);
        const frag: Frag = { ...base, text: op.text.slice(from - op.b, to - op.b), b: from };
        delete frag.inner;
        const actor = actorOf(sp.s);
        if (actor) {
          frag.actor = actor;
          frag.s = sp.s;
        }
        out.push(frag);
        at = to;
      }
      if (at < end) out.push({ ...base, text: op.text.slice(at - op.b), b: at });
      const only = out[first];
      if (out.length - first === 1 && only && op.inner) only.inner = op.inner;
      continue;
    }
    const want = normal(op.text);
    const hit = blame.deletions.findLast((d) => normal(d.text).includes(want) && want !== "");
    const actor = hit ? actorOf(hit.by) : undefined;
    out.push(actor && hit ? { ...base, actor, s: hit.by } : base);
  }
  return out;
}

/** The runs of changes, in order. */
export function hunksOf(frags: readonly Frag[]): Hunk[] {
  const out: Hunk[] = [];
  let cur: Hunk | undefined;
  frags.forEach((f, i) => {
    if (f.kind === "eq") {
      cur = undefined;
      return;
    }
    if (!cur) {
      cur = { from: i, to: i, actors: [] };
      out.push(cur);
    }
    cur.to = i;
    if (f.actor && !cur.actors.includes(f.actor)) cur.actors.push(f.actor);
  });
  return out;
}

/** A change shown whole: what was there, what is there, who did it. */
export interface EntryDiff {
  before: string;
  after: string;
  ops: Op[];
  frags: Frag[];
  hunks: Hunk[];
}

/** The diff of one session: every stretch carries the session's author. */
export function entryDiff(before: string, after: string, session: Session): EntryDiff {
  const ops = diffWithMoves(before, after);
  const frags = ops.map<Frag>((o) => {
    const f: Frag = { kind: o.kind, text: o.text, b: o.b };
    if (o.inner) f.inner = o.inner;
    if (o.moved !== undefined) f.moved = o.moved;
    if (o.kind !== "eq") f.actor = session.actor;
    return f;
  });
  return { before, after, ops, frags, hunks: hunksOf(frags) };
}

/** The diff between two moments, attributed from who wrote the text up to now (or nobody). */
export function compareTexts(before: string, after: string, blame: Blame | undefined): EntryDiff {
  const ops = diffWithMoves(before, after);
  const frags = attribute(ops, blame);
  return { before, after, ops, frags, hunks: hunksOf(frags) };
}
