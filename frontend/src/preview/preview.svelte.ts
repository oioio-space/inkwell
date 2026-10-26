import type { SocketLike } from "../net/WailsNetworkAdapter";
import { PreviewClient } from "./PreviewClient";
import type { Anchor, Diagnostic, Failure, PreviewMessage, ResultMessage } from "./protocol";
import { pageImage } from "./svg";

/** One page ready to show. An empty `src` is a page that could not be shown safely. */
export interface Page {
  /** The version of the sources this page was made from. */
  rev: number;
  src: string;
  /** Size in points. */
  width: number;
  height: number;
}

/** What Go may render at once, and how far around the view the pages are kept. */
const WINDOW = 40;
const MARGIN = 1;
const KEEP = 12;
const ANCHORS_TIMEOUT_MS = 60_000;
/** How long "updating" stays after an edit that Go never started compiling. */
const SETTLE_MS = 2500;

type Waiter = {
  resolve: (a: Anchor[]) => void;
  reject: (e: Error) => void;
  timer: ReturnType<typeof setTimeout>;
};

/** What the person sees of the compiled document: pages, page count, problems, state. */
export class PreviewModel {
  /** True from the first answer of Go, whatever it says. */
  hasResult = $state(false);
  compiling = $state(false);
  pageCount = $state(0);
  pages = $state.raw<ReadonlyMap<number, Page>>(new Map());
  diagnostics = $state.raw<readonly Diagnostic[]>([]);
  truncated = $state(false);
  /** Why there is no new document; undefined when the last compilation succeeded. */
  failure = $state.raw<Failure | undefined>();
  /** The page that fills most of the preview, 0 when the preview is not shown. */
  currentPage = $state(0);
  /** A mark to flash on the page after a jump from the source. */
  marker = $state.raw<{ page: number; y: number; id: number } | undefined>();

  errors = $derived(this.diagnostics.filter((d) => d.severity === "error").length);
  warnings = $derived(this.diagnostics.filter((d) => d.severity === "warning").length);

  #client: PreviewClient | undefined;
  /** The version of the last successful compilation: its pages are the fresh ones. */
  #okRev = -1;
  #anchors: { rev: number; list: Anchor[] } | undefined;
  #waiting: Waiter[] = [];
  #asked = "";
  #markerId = 0;
  #settle: ReturnType<typeof setTimeout> | undefined;

  start(open: () => SocketLike): void {
    this.#client ??= new PreviewClient(open, (m) => this.receive(m));
    this.#client.start();
  }

  stop(): void {
    this.#client?.stop();
  }

  /**
   * The document changed at `seconds` (Unix time): it dates the document. The preview is out of
   * date from now on, which it says at once; Go confirms a moment later that it is compiling, or
   * (when the change leaves the file as it was) nothing comes and the mention goes by itself.
   */
  noteEdit(seconds: number): void {
    if (seconds <= 0) return;
    this.#client?.edit(seconds);
    if (!this.hasResult) return;
    this.compiling = true;
    clearTimeout(this.#settle);
    this.#settle = setTimeout(() => {
      this.compiling = false;
    }, SETTLE_MS);
  }

  receive(m: PreviewMessage): void {
    switch (m.type) {
      case "compiling":
        clearTimeout(this.#settle);
        this.compiling = true;
        return;
      case "result":
        this.#result(m);
        return;
      case "anchors": {
        const waiters = this.#waiting.splice(0);
        for (const w of waiters) {
          clearTimeout(w.timer);
          if (m.failure) w.reject(new Error(m.failure.kind));
          else w.resolve(m.anchors);
        }
        if (!m.failure) this.#anchors = { rev: this.#okRev, list: m.anchors };
      }
    }
  }

  #result(m: ResultMessage): void {
    clearTimeout(this.#settle);
    this.compiling = false;
    this.hasResult = true;
    this.diagnostics = m.diagnostics;
    this.truncated = m.truncated;
    this.failure = m.failure;
    if (m.failure) return; // the pages of the last good version stay on screen
    this.#okRev = m.rev;
    this.pageCount = m.pageCount;
    const next = new Map(this.pages);
    m.pages.forEach((svg, i) => {
      const img = pageImage(svg);
      next.set(
        m.first + i,
        img
          ? { rev: m.rev, src: img.src, width: img.width, height: img.height }
          : { rev: m.rev, src: "", width: 595, height: 842 },
      );
    });
    for (const n of next.keys()) if (n > m.pageCount) next.delete(n);
    this.pages = next;
    this.#asked = "";
  }

  /** Size of a page in points once it is known. */
  box = (n: number): { width: number; height: number } | undefined => this.pages.get(n);

  /** Pages `first` to `last` are in view: asks Go for those that are missing or out of date, and forgets the far ones. */
  setVisible(first: number, last: number): void {
    if (this.pageCount === 0) return;
    const from = Math.max(1, first - MARGIN);
    const to = Math.min(this.pageCount, last + MARGIN, from + WINDOW - 1);
    let missing = false;
    for (let n = from; n <= to; n++) {
      if (this.pages.get(n)?.rev !== this.#okRev) missing = true;
    }
    const key = `${from}-${to}@${this.#okRev}`;
    if (missing && key !== this.#asked) {
      this.#asked = key;
      this.#client?.view(from, to);
    }
    // Pages far from the view are dropped: a long document must not fill memory.
    const far = [...this.pages.keys()].filter((n) => n < first - KEEP || n > last + KEEP);
    if (far.length > 0) {
      const next = new Map(this.pages);
      for (const n of far) next.delete(n);
      this.pages = next;
    }
  }

  /** Where the headings and paragraphs of the document are, for the jump. */
  anchors(): Promise<Anchor[]> {
    if (this.#anchors && this.#anchors.rev === this.#okRev)
      return Promise.resolve(this.#anchors.list);
    const client = this.#client;
    if (!client) return Promise.reject(new Error("no preview"));
    return new Promise<Anchor[]>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.#waiting = this.#waiting.filter((w) => w.timer !== timer);
        reject(new Error("timeout"));
      }, ANCHORS_TIMEOUT_MS);
      this.#waiting.push({ resolve, reject, timer });
      if (this.#waiting.length === 1) client.anchors();
    });
  }

  /** Flashes a mark at `y` points of `page` in the preview. */
  mark(page: number, y: number): void {
    this.marker = { page, y, id: ++this.#markerId };
  }
}

export const preview = new PreviewModel();
