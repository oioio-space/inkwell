import { next as A } from "@automerge/automerge";
import { Repo } from "@automerge/automerge-repo";
import type { HDoc } from "./types";

/** One device of a test: a real Automerge document with a fixed actor and a clock we control. */
export class Device {
  doc: A.Doc<HDoc>;
  /** The clock of this device, in seconds (what Automerge stores). */
  clock: number;

  constructor(
    readonly actor: string,
    base?: A.Doc<HDoc>,
    clock = 1_700_000_000,
  ) {
    this.clock = clock;
    this.doc = base
      ? A.clone(base, { actor })
      : A.change(A.init<HDoc>({ actor }), { time: clock }, (d) => {
          d.text = "";
          d.meta = { actors: {}, versions: [], variants: [] };
        });
  }

  /** Makes one change, `seconds` after the last one on this device. */
  edit(fn: (d: HDoc) => void, seconds = 1, message?: string): this {
    this.clock += seconds;
    this.doc = A.change(this.doc, { time: this.clock, ...(message ? { message } : {}) }, fn);
    return this;
  }

  /** Types `text` at `at` (the end by default). */
  type(text: string, at = this.doc.text.length, seconds = 1): this {
    return this.edit((d) => A.splice(d, ["text"], at, 0, text), seconds);
  }

  /** Replaces the text with `text` (one change). */
  set(text: string, seconds = 1): this {
    return this.edit((d) => A.updateText(d, ["text"], text), seconds);
  }

  /** Receives everything the other device has. */
  pull(other: Device): this {
    this.doc = A.merge(this.doc, other.doc);
    return this;
  }

  /** Both devices end with everything. */
  sync(other: Device): this {
    this.pull(other);
    other.pull(this);
    return this;
  }

  claim(name: string, peer = `peer-${this.actor}`): this {
    return this.edit((d) => {
      d.meta ??= {};
      d.meta.actors ??= {};
      d.meta.actors[this.actor] = { name, peer, at: 0 };
    });
  }

  get heads(): string[] {
    return A.getHeads(this.doc);
  }
}

/** A repo with no network and no storage, for the code that works on handles. */
export const memoryRepo = (): Repo => new Repo({});
