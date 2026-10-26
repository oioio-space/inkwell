import { Repo } from "@automerge/automerge-repo";
import { flushSync } from "svelte";
import type { IndexDoc, Role } from "../../project/model";
import { type BlobStatus, ProjectRuntime } from "../../project/runtime.svelte";
import { FakeGoStore } from "../../store/fakeStore";
import { GoStorageAdapter } from "../../store/GoStorageAdapter";

/** A project with its runtime, over the fake Go side: what the file manager's tests drive. */
export async function project(role: Role = "admin", blobStatus?: () => Promise<BlobStatus>) {
  const store = new FakeGoStore();
  store.strictOwnership = true;
  const storage = new GoStorageAdapter(() => store.socket());
  const repo = new Repo({});
  const root = repo.create<IndexDoc>({ text: "" });
  const runtime = new ProjectRuntime(root, {
    repo,
    storage,
    me: () => ({ id: "alice", name: "Alice" }),
    ...(blobStatus ? { blobStatus } : {}),
  });
  runtime.role = role;
  runtime.start();
  return { runtime, store, repo };
}

/** happy-dom has no modal dialog: give it the two methods the components call. */
export function shimDialog(): void {
  HTMLDialogElement.prototype.showModal = function showModal() {
    this.setAttribute("open", "");
  };
  HTMLDialogElement.prototype.close = function close() {
    this.removeAttribute("open");
    this.dispatchEvent(new Event("close"));
  };
}

/** The tree row of a path. */
export const rowOf = (host: HTMLElement, path: string) =>
  host.querySelector<HTMLElement>(`[role="treeitem"][data-path="${path}"]`);

export const rowPaths = (host: HTMLElement) =>
  [...host.querySelectorAll<HTMLElement>('[role="treeitem"]')].map((r) => r.dataset.path);

/** A drag event with a data transfer that remembers what was set on it. */
export function dragEvent(
  type: string,
  data: Record<string, string> = {},
  files: File[] = [],
): DragEvent {
  const store = new Map(Object.entries(data));
  const dt = {
    types: [...store.keys(), ...(files.length > 0 ? ["Files"] : [])],
    files,
    items: [],
    effectAllowed: "all",
    dropEffect: "none",
    setData: (k: string, v: string) => {
      store.set(k, v);
      (dt.types as string[]).push(k);
    },
    getData: (k: string) => store.get(k) ?? "",
  };
  const e = new Event(type, { bubbles: true, cancelable: true }) as DragEvent;
  Object.defineProperty(e, "dataTransfer", { value: dt });
  return e;
}

export function fire(el: Element, e: Event): void {
  el.dispatchEvent(e);
  flushSync();
}
