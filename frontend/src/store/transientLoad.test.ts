import type { Chunk, StorageAdapterInterface, StorageKey } from "@automerge/automerge-repo";
import { Repo } from "@automerge/automerge-repo";
import { expect, test } from "vitest";

// Repro of CI run 45542633229 (persistence.spec.ts:53): a store stream closed while the start
// loads the saved document. GoStorageAdapter rejects the pending load ("store stream closed");
// the question is what find() then does, since openStart() reads any rejection as "the saved
// document is gone" and creates a fresh one.
class Memory implements StorageAdapterInterface {
  data = new Map<string, Uint8Array>();
  failNext = 0;
  #fail(): void {
    if (this.failNext > 0) {
      this.failNext--;
      throw new Error("store stream closed");
    }
  }
  async load(key: StorageKey) {
    this.#fail();
    return this.data.get(key.join("/"));
  }
  async save(key: StorageKey, v: Uint8Array) {
    this.data.set(key.join("/"), v);
  }
  async remove(key: StorageKey) {
    this.data.delete(key.join("/"));
  }
  async loadRange(prefix: StorageKey): Promise<Chunk[]> {
    this.#fail();
    const p = prefix.join("/");
    return [...this.data]
      .filter(([k]) => k.startsWith(p))
      .map(([k, data]) => ({ key: k.split("/"), data }));
  }
  async removeRange(prefix: StorageKey) {
    for (const k of [...this.data.keys()]) if (k.startsWith(prefix.join("/"))) this.data.delete(k);
  }
}

test("a transient storage failure makes find() reject a document that storage holds", async () => {
  const storage = new Memory();
  const first = new Repo({ storage, isEphemeral: true });
  const h = first.create<{ text: string }>({ text: "persisted" });
  await first.flush();
  await first.shutdown();

  const again = new Repo({ storage, isEphemeral: true });
  storage.failNext = 1; // the one load in flight when the stream closed
  await expect(again.find(h.url)).rejects.toThrow();
  // The document was never gone: a second find, once the stream is back, has it.
  const back = new Repo({ storage, isEphemeral: true });
  expect((await back.find<{ text: string }>(h.url)).doc().text).toBe("persisted");
});
