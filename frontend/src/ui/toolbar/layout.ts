/** Widths of the formatting bar's parts, in px (see theme/adwaita.css and FormatBar.svelte). */
export const BAR = {
  /** Side padding of the bar. */
  padding: 16,
  heading: 132,
  button: 34,
  gap: 2,
  groupGap: 14,
  insert: 104,
} as const;

/** How a bar's items take room: each button, the gap between two, and between two groups. */
export interface BarMetrics {
  readonly button: number;
  readonly gap: number;
  readonly groupGap: number;
}

/**
 * How many of the leading items fit in `room` px; the rest go to the "⋯" menu. `groups` is each
 * item's group number, in order: a change of group adds a wider gap. When something overflows,
 * room for the "⋯" button itself is kept. Any bar of a location with `overflow: "width"` (EX0c2).
 */
export function fitCount(groups: readonly number[], room: number, m: BarMetrics = BAR): number {
  const countFitting = (reserve: number): number => {
    let used = reserve;
    let n = 0;
    for (const [i, group] of groups.entries()) {
      const before = i > 0 && groups[i - 1] !== group ? m.groupGap : m.gap;
      used += before + m.button;
      if (used > room) break;
      n++;
    }
    return n;
  };
  const all = countFitting(0);
  return all === groups.length ? all : countFitting(m.gap + m.button);
}

/** How many of the formatting bar's items fit on a bar `width` px wide, around its fixed parts. */
export function visibleCount(groups: readonly number[], width: number): number {
  const fixed = BAR.padding + BAR.heading + BAR.groupGap + BAR.groupGap + BAR.insert;
  return fitCount(groups, width - fixed);
}
