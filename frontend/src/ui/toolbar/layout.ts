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

/**
 * How many of the leading items fit on a bar `width` px wide; the rest go to the "⋯" menu.
 * `groups` is each item's group number, in order: a change of group adds a wider gap. When
 * something overflows, room for the "⋯" button itself is kept.
 */
export function visibleCount(groups: readonly number[], width: number): number {
  const fixed = BAR.padding + BAR.heading + BAR.groupGap + BAR.groupGap + BAR.insert;
  const room = width - fixed;
  const countFitting = (reserve: number): number => {
    let used = reserve;
    let n = 0;
    for (const [i, group] of groups.entries()) {
      const before = i > 0 && groups[i - 1] !== group ? BAR.groupGap : BAR.gap;
      used += before + BAR.button;
      if (used > room) break;
      n++;
    }
    return n;
  };
  const all = countFitting(0);
  return all === groups.length ? all : countFitting(BAR.gap + BAR.button);
}
