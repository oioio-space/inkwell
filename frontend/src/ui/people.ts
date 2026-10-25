/** CSS custom properties of the 8 Okabe-Ito person colours (src/theme/tokens.css). */
export const PERSON_COLORS = [
  "var(--person-1)",
  "var(--person-2)",
  "var(--person-3)",
  "var(--person-4)",
  "var(--person-5)",
  "var(--person-6)",
  "var(--person-7)",
  "var(--person-8)",
] as const;

export type PersonColor = (typeof PERSON_COLORS)[number];

/**
 * Returns the colour shown for a peer. It is derived from the peer id (FNV-1a), so
 * every participant sees the same colour for the same person without coordination.
 * Colour is never the only cue: the name is always shown too (docs/ux-principles.md).
 */
export function personColor(peerId: string): PersonColor {
  return PERSON_COLORS[personIndex(peerId) - 1] as PersonColor;
}

/** The 1-based palette slot (1–8) of a peer, for CSS classes like `cm-who-3`. */
export function personIndex(peerId: string): number {
  let hash = 0x811c9dc5;
  for (let i = 0; i < peerId.length; i++) {
    hash ^= peerId.charCodeAt(i);
    hash = Math.imul(hash, 0x01000193);
  }
  return ((hash >>> 0) % PERSON_COLORS.length) + 1;
}
