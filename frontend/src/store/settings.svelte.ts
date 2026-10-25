import { safeGet, safeSet } from "./safeStorage";

/** When Inkwell writes the file on disk: after a pause in typing, or only on Ctrl+S. */
export type SaveMode = "auto" | "manual";
/** What happens when a project file changes on disk: ask, take the change, or keep ours. */
export type ExternalPolicy = "ask" | "import" | "keep";

export const SAVE_DELAYS = [1, 2, 5, 10] as const;
export type SaveDelay = (typeof SAVE_DELAYS)[number];

const PREFS_KEY = "inkwell.settings";
const LAYOUT_KEY = "inkwell.layout";

/** Layout choices remembered per project: what the person closed. */
interface Layout {
  toolbarHidden?: boolean;
  sidebarHidden?: boolean;
}

function readJSON(key: string): Record<string, unknown> {
  const raw = safeGet(key);
  if (!raw) return {};
  try {
    const value: unknown = JSON.parse(raw);
    return value && typeof value === "object" && !Array.isArray(value)
      ? (value as Record<string, unknown>)
      : {};
  } catch {
    return {};
  }
}

const pick = <T extends string | number>(value: unknown, allowed: readonly T[], fallback: T): T =>
  allowed.includes(value as T) ? (value as T) : fallback;

/**
 * Preferences and per-project layout, remembered in the webview. Their behaviour (saving,
 * outside changes) is wired by the persistence layer; this store only keeps the choices.
 */
class Settings {
  saveMode = $state<SaveMode>("auto");
  /** Seconds of quiet before an automatic save. */
  saveDelay = $state<SaveDelay>(2);
  externalPolicy = $state<ExternalPolicy>("ask");
  #layout = $state<Record<string, Layout>>({});

  constructor() {
    const prefs = readJSON(PREFS_KEY);
    this.saveMode = pick(prefs.saveMode, ["auto", "manual"], "auto");
    this.saveDelay = pick(prefs.saveDelay, SAVE_DELAYS, 2);
    this.externalPolicy = pick(prefs.externalPolicy, ["ask", "import", "keep"], "ask");
    const layout = readJSON(LAYOUT_KEY);
    for (const [project, value] of Object.entries(layout)) {
      if (value && typeof value === "object") this.#layout[project] = value as Layout;
    }
  }

  setSaveMode(mode: SaveMode): void {
    this.saveMode = mode;
    this.#savePrefs();
  }

  setSaveDelay(seconds: SaveDelay): void {
    this.saveDelay = seconds;
    this.#savePrefs();
  }

  setExternalPolicy(policy: ExternalPolicy): void {
    this.externalPolicy = policy;
    this.#savePrefs();
  }

  toolbarVisible(project: string): boolean {
    return !this.#layout[project]?.toolbarHidden;
  }

  setToolbarVisible(project: string, visible: boolean): void {
    this.#setLayout(project, { toolbarHidden: !visible });
  }

  sidebarVisible(project: string): boolean {
    return !this.#layout[project]?.sidebarHidden;
  }

  setSidebarVisible(project: string, visible: boolean): void {
    this.#setLayout(project, { sidebarHidden: !visible });
  }

  #setLayout(project: string, patch: Layout): void {
    this.#layout[project] = { ...this.#layout[project], ...patch };
    safeSet(LAYOUT_KEY, JSON.stringify(this.#layout));
  }

  #savePrefs(): void {
    safeSet(
      PREFS_KEY,
      JSON.stringify({
        saveMode: this.saveMode,
        saveDelay: this.saveDelay,
        externalPolicy: this.externalPolicy,
      }),
    );
  }
}

export const settings = new Settings();
export type { Settings };
