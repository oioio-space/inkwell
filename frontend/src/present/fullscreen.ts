/** The part of `document` the full screen toggle uses. */
export interface FullscreenHost {
  readonly fullscreenElement: Element | null;
  readonly documentElement: { requestFullscreen(): Promise<void> };
  exitFullscreen(): Promise<void>;
}

/**
 * Full screen for the public window, the way a browser does it: the window stays where the person
 * put it (on the projector's screen, say) and fills it. Esc leaves, which the browser handles itself.
 * Returns whether the window is full screen afterwards; a refusal (no permission) leaves it as it was.
 */
export async function toggleFullscreen(doc: FullscreenHost): Promise<boolean> {
  try {
    if (doc.fullscreenElement) {
      await doc.exitFullscreen();
      return false;
    }
    await doc.documentElement.requestFullscreen();
    return true;
  } catch {
    return doc.fullscreenElement !== null;
  }
}
