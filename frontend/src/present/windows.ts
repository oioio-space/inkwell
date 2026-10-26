/**
 * The window layer of a presentation. The public view is a second window; how one opens depends on
 * where Inkwell runs, and the rest of the code does not care:
 *
 * - the desktop app asks Go (`PresentService`), which opens an ordinary native window;
 * - the browser build (the end-to-end tests, `-tags server`) has no native window: Go says so and
 *   the page opens the same view, `/?view=public`, in a pop-up of its own.
 *
 * Neither guesses screens: the person drags the window to the one they want and makes it full
 * screen there (see `fullscreen.ts`).
 */

/** The part of `PresentService` the layer uses. */
export interface PresentApi {
  Open(): Promise<{ Native: boolean }>;
  Close(): Promise<void>;
}

/** The part of `window` the layer uses to open a pop-up. */
export interface PopupOpener {
  open(url: string, name: string, features: string): { close(): void; closed: boolean } | null;
  readonly location: { origin: string };
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

  /** Opens the public window; false for a browser pop-up. */
  async open(): Promise<boolean> {
    const { Native } = await this.#api.Open();
    if (Native) return true;
    if (!this.#popup || this.#popup.closed) {
      this.#popup = this.#host.open(
        `${this.#host.location.origin}${PUBLIC_URL}`,
        "inkwell-public",
        POPUP,
      );
    }
    return false;
  }

  async close(): Promise<void> {
    this.#popup?.close();
    this.#popup = null;
    await this.#api.Close();
  }
}
