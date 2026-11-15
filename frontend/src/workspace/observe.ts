import {
  type DocHandle,
  isValidAutomergeUrl,
  parseAutomergeUrl,
  type Repo,
} from "@automerge/automerge-repo";
import type { LinkView } from "../net/links.svelte";

/** Whether this is a development or an explicitly instrumented e2e build. */
const observing = () =>
  import.meta.env.MODE === "development" || import.meta.env.VITE_E2E_OBSERVE === "1";

/** Read-only diagnostics for development and explicitly instrumented e2e builds. */
export function observeVariantHeads(
  root: () =>
    | DocHandle<{ meta?: { variants?: readonly { name: string; url: string }[] } }>
    | undefined,
  repo: () => Repo | undefined,
): void {
  if (!observing()) return;
  (
    globalThis as typeof globalThis & { __inkwellVariantHeads?: (name: string) => string[] }
  ).__inkwellVariantHeads = (name) => {
    const url = root()
      ?.doc()
      .meta?.variants?.find((v) => v.name === name)?.url;
    if (!url || !isValidAutomergeUrl(url)) return [];
    const id = parseAutomergeUrl(url).documentId;
    const handles = repo()?.handles ?? {};
    const handle = Object.hasOwn(handles, id) ? handles[id] : undefined;
    return handle?.isReady() ? [...handle.heads()].sort() : [];
  };
}

/**
 * The document as typed, from the editor state (the e2e specs read it: the DOM holds only part of
 * a long line).
 */
export function observeDocText(text: () => string | undefined): void {
  if (!observing()) return;
  (
    globalThis as typeof globalThis & { __inkwellDocText?: () => string | undefined }
  ).__inkwellDocText = text;
}

/**
 * The state of the links to the peers (link metrics contract, plan section 7). `shown` is what the
 * header says right now ("synced", "sending", ...): the e2e clause SI1 samples it beside the links.
 */
export function observeLinks(view: () => LinkView[], shown: () => string = () => ""): void {
  if (!observing()) return;
  (
    globalThis as typeof globalThis & {
      __inkwellLinks?: (() => LinkView[]) & { shown?: () => string };
    }
  ).__inkwellLinks = Object.assign(() => view(), { shown });
}

/** What this page holds of its session, as the e2e of a project change reads it (PC1, rule N-11). */
export interface SessionView {
  /** The signers whose attestations the page holds. */
  signers: string[];
  /** The peers the page talks to. */
  peers: string[];
}

/**
 * The startup id of the process image that answers (the only proof of a restart, rule N-12) and
 * what the page holds of its session: the e2e of a project change asserts on them, never on timing.
 */
export function observeSession(startupId: () => Promise<string>, view: () => SessionView): void {
  if (!observing()) return;
  const g = globalThis as typeof globalThis & {
    __inkwellStartupId?: () => Promise<string>;
    __inkwellSession?: () => SessionView;
  };
  g.__inkwellStartupId = startupId;
  g.__inkwellSession = view;
}
