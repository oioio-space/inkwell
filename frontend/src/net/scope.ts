import {
  type AutomergeUrl,
  isValidAutomergeUrl,
  parseAutomergeUrl,
} from "@automerge/automerge-repo";

import { MAX_PROJECT_URL_LENGTH } from "./admission";

/** Interim root listing. Admission is sticky until the project changes. */
export class AdmittedSet {
  #ids = new Set<string>();
  #root?: AutomergeUrl;
  epoch = 0;
  #listeners = new Set<() => void>();
  get ids(): ReadonlySet<string> {
    return this.#ids;
  }
  has = (id: string): boolean => this.#ids.has(id);
  onChange(cb: () => void): () => void {
    this.#listeners.add(cb);
    return () => {
      this.#listeners.delete(cb);
    };
  }
  #changed(): void {
    for (const cb of this.#listeners) cb();
  }
  setRoot(url: AutomergeUrl): void {
    if (url === this.#root) return;
    this.#root = url;
    this.epoch++;
    this.#ids.clear();
    this.#ids.add(parseAutomergeUrl(url).documentId);
    this.#changed();
  }
  refresh(doc: { meta?: { variants?: readonly { url?: unknown }[] } }): void {
    let changed = false;
    for (const variant of doc.meta?.variants ?? []) {
      if (this.#ids.size >= 257) break;
      const url = variant?.url;
      if (
        typeof url !== "string" ||
        url.length > MAX_PROJECT_URL_LENGTH ||
        !isValidAutomergeUrl(url)
      )
        continue;
      const id = parseAutomergeUrl(url).documentId;
      if (id.length > 64 || this.#ids.has(id)) continue;
      this.#ids.add(id);
      changed = true;
    }
    if (changed) this.#changed();
  }
}
