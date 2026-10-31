import {
  type DocHandle,
  isValidAutomergeUrl,
  parseAutomergeUrl,
  type Repo,
} from "@automerge/automerge-repo";

/** Read-only diagnostics for development and explicitly instrumented e2e builds. */
export function observeVariantHeads(
  root: () =>
    | DocHandle<{ meta?: { variants?: readonly { name: string; url: string }[] } }>
    | undefined,
  repo: () => Repo | undefined,
): void {
  if (import.meta.env.MODE !== "development" && import.meta.env.VITE_E2E_OBSERVE !== "1") return;
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
