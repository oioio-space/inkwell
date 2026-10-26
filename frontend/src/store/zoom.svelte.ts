import { safeGet, safeSet } from "./safeStorage";

export const ZOOM_MIN = 50;
export const ZOOM_MAX = 300;
export const ZOOM_DEFAULT = 100;
const STEP = 10;
const KEY = "inkwell.zoom";

/** The nearest allowed zoom: whole percents from 50 to 300. */
export const clampZoom = (pct: number): number =>
  Number.isFinite(pct) ? Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, Math.round(pct))) : ZOOM_DEFAULT;

/**
 * The text zoom of each project: a setting of this device, never sent to anyone (ZOOM-02), kept
 * per project. It applies to the editor, the differences view and, once it has a hook, the sheet.
 */
class Zoom {
  #levels = $state<Record<string, number>>({});

  constructor() {
    try {
      const raw = safeGet(KEY);
      const parsed: unknown = raw ? JSON.parse(raw) : {};
      if (parsed && typeof parsed === "object") {
        for (const [project, v] of Object.entries(parsed)) {
          if (typeof v === "number") this.#levels[project] = clampZoom(v);
        }
      }
    } catch {
      // Not remembered: back to 100 %.
    }
  }

  level(project: string): number {
    return this.#levels[project] ?? ZOOM_DEFAULT;
  }

  set(project: string, pct: number): void {
    this.#levels[project] = clampZoom(pct);
    safeSet(KEY, JSON.stringify(this.#levels));
  }

  /** One step bigger (+10 points); stays at 300. */
  in(project: string): void {
    this.set(project, this.level(project) + STEP);
  }

  /** One step smaller (-10 points); stays at 50. */
  out(project: string): void {
    this.set(project, this.level(project) - STEP);
  }

  reset(project: string): void {
    this.set(project, ZOOM_DEFAULT);
  }
}

export const zoom = new Zoom();
export type { Zoom };
