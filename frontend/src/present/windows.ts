/**
 * The window layer of a presentation. The public view is a second window; how one opens depends on
 * where Inkwell runs, and the rest of the code does not care:
 *
 * - the desktop app asks Go (`PresentService`), which opens a native window and puts it on the
 *   other screen when there is one (fullscreen), or in an ordinary movable window when there is not;
 * - the browser build (the end-to-end tests, `-tags server`) has no native window: Go says so and
 *   the page opens the same view, `/?view=public`, in a pop-up of its own.
 */

/** What Go says of the screens, as the bindings give it (`PresentLayout`). */
export interface LayoutAnswer {
  Native: boolean;
  Dual: boolean;
}

/** The part of `PresentService` the layer uses. */
export interface PresentApi {
  Open(swapped: boolean): Promise<LayoutAnswer>;
  Close(): Promise<void>;
}

/** The part of `window` the layer uses to open a pop-up. */
export interface PopupOpener {
  open(url: string, name: string, features: string): { close(): void; closed: boolean } | null;
  readonly location: { origin: string };
}

export interface OpenedPublic {
  /** The public view has a screen of its own. */
  dual: boolean;
  /** A native window; false for a browser pop-up. */
  native: boolean;
}

const PUBLIC_URL = "/?view=public";
const POPUP = "popup=yes,width=960,height=540";

export class PublicWindows {
  readonly #api: PresentApi;
  readonly #host: PopupOpener;
  #popup: { close(): void; closed: boolean } | null = null;

  constructor(api: PresentApi, host: PopupOpener) {
    this.#api = api;
    this.#host = host;
  }

  /** Opens the public window, or moves it ("Échanger les écrans") if it is open. */
  async open(swapped: boolean): Promise<OpenedPublic> {
    const layout = await this.#api.Open(swapped);
    if (layout.Native) return { dual: layout.Dual, native: true };
    if (!this.#popup || this.#popup.closed) {
      this.#popup = this.#host.open(
        `${this.#host.location.origin}${PUBLIC_URL}`,
        "inkwell-public",
        POPUP,
      );
    }
    return { dual: false, native: false };
  }

  async close(): Promise<void> {
    this.#popup?.close();
    this.#popup = null;
    await this.#api.Close();
  }
}
