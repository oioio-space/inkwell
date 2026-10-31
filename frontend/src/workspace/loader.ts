import type { DocumentId, Repo } from "@automerge/automerge-repo";
import type { AdmittedSet } from "../net/scope";
/** FIFO bootstrap, including loading handles; never waits for network readiness. */
export class BootstrapLoader {
  #queue: { id: string; epoch: number }[] = [];
  #pending = new Set<string>();
  #active = 0;
  #epoch = -1;
  constructor(
    private repo: Repo,
    private set: AdmittedSet,
    private ready: () => void = () => {},
  ) {}
  ensure(id: string): void {
    if (this.#epoch !== this.set.epoch) {
      for (const stale of this.#queue) this.#pending.delete(`${stale.epoch}:${stale.id}`);
      this.#queue = [];
      this.#epoch = this.set.epoch;
    }
    const key = `${this.set.epoch}:${id}`;
    if (!this.set.has(id) || this.#pending.has(key) || this.#queue.length >= 257) return;
    this.#pending.add(key);
    this.#queue.push({ id, epoch: this.set.epoch });
    this.#pump();
  }
  #pump(): void {
    while (this.#active < 4 && this.#queue.length) {
      const job = this.#queue.shift();
      if (!job) break;
      const key = `${job.epoch}:${job.id}`;
      if (job.epoch !== this.set.epoch || !this.set.has(job.id)) {
        this.#pending.delete(key);
        continue;
      }
      this.#active++;
      void this.repo
        .find(job.id as DocumentId, { allowableStates: ["ready", "requesting", "unavailable"] })
        .then(() => {
          if (job.epoch === this.set.epoch && this.set.has(job.id)) this.ready();
        })
        .catch(() => {})
        .finally(() => {
          this.#active--;
          this.#pending.delete(key);
          this.#pump();
        });
    }
  }
}
