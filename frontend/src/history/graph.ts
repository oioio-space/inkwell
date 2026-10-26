import type { Session } from "./log";
import type { Heads, Variant, Version } from "./types";

export type RowKind = "now" | "session" | "version" | "variant" | "merge";

/** One line of the history: its node, and what the lines around it do (drawn per row, so the list can be virtual). */
export interface Row {
  key: string;
  kind: RowKind;
  /** The lane of the node: 0 is the main document, 1 and more are variants. */
  lane: number;
  /** Position in the causal order: the index of the last step it covers. */
  pos: number;
  session?: Session;
  version?: Version;
  variant?: Variant;
  /** Lanes crossing this row from top to bottom (a variant still open between its start and its merge). */
  through: number[];
  /** A variant branches off here: its lane arrives from above and joins this node (the starting point). */
  forkFrom?: number;
  /** A variant joins this node from below (the merge, drawn coming back to the main line). */
  mergeFrom?: number;
  /** A variant's node: its lane continues below it, to the starting point, and above it when merged. */
  down?: boolean;
  up?: boolean;
}

export interface GraphInput {
  /** The hash of each change, in order (the order the sessions were cut from). */
  order: readonly string[];
  sessions: readonly Session[];
  versions: readonly Version[];
  variants: readonly Variant[];
  /** Show the variants that were set aside. */
  showAbandoned?: boolean;
}

/** The position of a point of history: the last step it includes. */
function posOf(index: ReadonlyMap<string, number>, heads: Heads, fallback: number): number {
  let pos = -1;
  for (const h of heads) {
    const i = index.get(h);
    if (i === undefined) return fallback;
    pos = Math.max(pos, i);
  }
  return pos;
}

/**
 * The rows of the history graph, newest first. The order is the document's causal order, never
 * the clocks (X-20). A variant is drawn as a branch at the point it was started from, and its
 * merge as a line coming back to the main one.
 */
export function buildGraph({
  order,
  sessions,
  versions,
  variants,
  showAbandoned = false,
}: GraphInput): Row[] {
  const index = new Map(order.map((h, i) => [h, i] as const));
  const last = order.length - 1;
  type Item = Row & { order: number };
  const items: Item[] = [];
  // Order within one position: the session, then the version named there, then the variants
  // started there (above, so each branch is drawn next to its starting point), then merges.
  for (const s of sessions) {
    items.push({
      key: `s:${s.id}`,
      kind: "session",
      lane: 0,
      pos: s.to,
      order: s.to + 0,
      session: s,
      through: [],
    });
  }
  for (const [i, v] of versions.entries()) {
    const pos = posOf(index, v.heads, last);
    items.push({
      key: `v:${i}`,
      kind: "version",
      lane: 0,
      pos,
      order: pos + 0.1,
      version: v,
      through: [],
    });
  }
  let lane = 0;
  const visible = variants.filter((v) => showAbandoned || v.state !== "abandoned");
  const laneOf = new Map<string, number>();
  for (const v of visible) laneOf.set(v.url, ++lane);
  for (const v of visible) {
    const l = laneOf.get(v.url) as number;
    const from = posOf(index, v.from, last);
    items.push({
      key: `b:${v.url}`,
      kind: "variant",
      lane: l,
      pos: from,
      order: from + 0.2 + l / 100,
      variant: v,
      through: [],
      down: true,
      up: v.state === "merged",
    });
    if (v.state === "merged") {
      const at = v.mergedHeads ? posOf(index, v.mergedHeads, last) : last;
      items.push({
        key: `m:${v.url}`,
        kind: "merge",
        lane: 0,
        pos: at,
        order: at + 0.3 + l / 100,
        variant: v,
        mergeFrom: l,
        through: [],
      });
    }
  }
  items.sort((x, y) => x.order - y.order);

  // Lanes crossing the rows between a variant's node and its merge; the starting point.
  const rowOf = new Map(items.map((it, i) => [it.key, i] as const));
  for (const v of visible) {
    const l = laneOf.get(v.url) as number;
    const start = rowOf.get(`b:${v.url}`) as number;
    const end = rowOf.get(`m:${v.url}`);
    if (end !== undefined) for (let i = start + 1; i < end; i++) (items[i] as Item).through.push(l);
    // The row just below the variant's node is where it comes from: the node of the main line.
    const origin = items.findLastIndex((it, i) => i < start && it.lane === 0);
    if (origin >= 0) (items[origin] as Item).forkFrom = l;
  }

  const rows: Row[] = items.map(({ order: _o, ...row }) => row);
  rows.reverse();
  rows.unshift({ key: "now", kind: "now", lane: 0, pos: last + 1, through: [] });
  return rows;
}
