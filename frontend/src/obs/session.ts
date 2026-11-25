/**
 * The webview's side of the local statistics (plan docs/plans/2026-11-24-obs1-observability.md
 * §2): what the built-in extensions count is summed here, in memory, and handed to Go once a
 * minute and when the page goes (as editor/amSync.ts does). Nothing on the typing path crosses
 * the bridge (Z-2), and nothing leaves the device: Go only keeps it, under rule V.
 */
import type {
  Delta,
  Descriptor,
} from "../../bindings/github.com/oioio-space/inkwell/internal/obs/models";

/** How often the deltas are handed to Go. */
export const FLUSH_EVERY = 60_000;

/** What the session needs from Go: three calls of `StatsService`. */
export interface StatsBackend {
  Descriptors(): PromiseLike<Descriptor[] | null>;
  Declare(ds: Descriptor[]): PromiseLike<void>;
  Flush(batch: Delta[]): PromiseLike<void>;
}

/** A metric as the registry holds it. */
export interface MetricInfo {
  readonly id: string;
  readonly kind: string;
  readonly unit: string;
  readonly scope: string;
  readonly label: string;
  readonly max: number;
  readonly values: readonly string[];
}

export interface SessionOptions {
  every?: number;
  /**
   * Merges Go's own descriptors into the metrics (the registry's `withGoMetrics`, M-3) and
   * returns the ones the webview keeps: a metric refused for a clash is not declared nor sent.
   */
  merge?: (go: readonly unknown[]) => readonly MetricInfo[];
  /** The page, whose `pagehide` flushes; none in a test that does not need it. */
  page?: Pick<Window, "addEventListener" | "removeEventListener">;
}

interface Pending {
  m: string;
  doc: string;
  v: number;
  n: number;
}

/** Sums the deltas of declared metrics and hands them to Go at each flush. */
export class StatsSession {
  #metrics: ReadonlyMap<string, MetricInfo>;
  readonly #pending = new Map<string, Pending>();
  #declared: PromiseLike<void> | undefined;
  #timer: ReturnType<typeof setInterval> | undefined;

  constructor(
    private readonly backend: StatsBackend,
    metrics: readonly MetricInfo[],
    private readonly options: SessionOptions = {},
  ) {
    this.#metrics = new Map(metrics.map((m) => [m.id, m]));
  }

  /**
   * Merges Go's metrics into the view, declares the webview's to Go, once, and flushes every
   * minute and when the page goes.
   */
  start(): void {
    if (this.#declared) return;
    const merge = this.options.merge;
    this.#declared = Promise.resolve(merge ? this.backend.Descriptors() : null)
      .then((go) => {
        if (merge) {
          const kept = new Set(merge(go ?? []).map((m) => m.id));
          this.#metrics = new Map([...this.#metrics].filter(([id]) => kept.has(id)));
        }
        return this.backend.Declare(this.#declarations());
      })
      .then(undefined, () => console.warn("statistics: the metrics were refused"));
    this.#timer = setInterval(() => void this.flush(), this.options.every ?? FLUSH_EVERY);
    this.options.page?.addEventListener("pagehide", this.#onHide);
  }

  #declarations(): Descriptor[] {
    return [...this.#metrics.values()].map((m) => ({
      id: m.id,
      kind: m.kind,
      unit: m.unit,
      scope: m.scope,
      label: m.label,
      max: m.max,
      ...(m.values.length > 0 ? { values: [...m.values] } : {}),
    }));
  }

  readonly #onHide = () => void this.flush();

  stop(): void {
    clearInterval(this.#timer);
    this.options.page?.removeEventListener("pagehide", this.#onHide);
  }

  /** Adds v to a declared metric, for the document `doc` if it is a document metric. */
  add(id: string, v: number, doc = ""): void {
    this.#sum(id, v, 0, doc);
  }

  /** Adds one measured time of ms milliseconds to a declared duration. */
  duration(id: string, ms: number, doc = ""): void {
    this.#sum(id, Math.round(ms), 1, doc);
  }

  #sum(id: string, v: number, n: number, doc: string): void {
    const m = this.#metrics.get(id);
    if (!m || !Number.isSafeInteger(v) || v < 0 || (v === 0 && n === 0)) return;
    if ((m.scope === "document") !== (doc !== "")) return;
    const key = `${id}\n${doc}`;
    const p = this.#pending.get(key);
    if (p) {
      p.v += v;
      p.n += n;
    } else this.#pending.set(key, { m: id, doc, v, n });
  }

  /** Hands what was summed since the last flush to Go, in one call; nothing if there is none. */
  async flush(): Promise<void> {
    if (this.#pending.size === 0) return;
    const pending = [...this.#pending.values()];
    this.#pending.clear();
    await this.#declared;
    const batch: Delta[] = [];
    for (const p of pending) {
      const max = this.#metrics.get(p.m)?.max;
      if (max === undefined) continue; // refused at the merge
      // A sum above the bound of one delta goes as several.
      for (let v = p.v, n = p.n, first = true; first || v > 0; first = false) {
        const part = Math.min(v, max);
        batch.push({ m: p.m, v: part, ...(p.doc ? { doc: p.doc } : {}), ...(n ? { n } : {}) });
        v -= part;
        n = 0;
      }
    }
    if (batch.length === 0) return;
    try {
      await this.backend.Flush(batch);
    } catch {
      console.warn("statistics: a batch was refused"); // statistics are never worth an error shown
    }
  }
}
