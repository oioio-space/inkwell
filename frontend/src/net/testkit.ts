import { type PeerId, Repo } from "@automerge/automerge-repo";
import type { SimNet } from "./simnet";
import { WailsNetworkAdapter } from "./WailsNetworkAdapter";

/** A simulated device: its repo, its adapter, and the peers its adapter announced. */
export interface TestPeer {
  id: string;
  adapter: WailsNetworkAdapter;
  repo: Repo;
  connected: Set<string>;
}

/** Builds peers on a {@link SimNet} and shuts their repos down at the end of a test. */
export class Kit {
  #repos: Repo[] = [];

  peer(net: SimNet, id: string): TestPeer {
    const adapter = new WailsNetworkAdapter(() => net.socket(id), {
      reconnectDelayMs: 5,
      admitted: () => true,
    });
    const repo = new Repo({
      peerId: id as PeerId,
      network: [adapter],
      sharePolicy: async () => true,
    });
    this.#repos.push(repo);
    const connected = new Set<string>();
    adapter.on("peer-candidate", ({ peerId }) => connected.add(peerId));
    adapter.on("peer-disconnected", ({ peerId }) => connected.delete(peerId));
    return { id, adapter, repo, connected };
  }

  async shutdown(): Promise<void> {
    await Promise.all(this.#repos.splice(0).map((r) => r.shutdown()));
  }
}

/** Polls `cond` every few milliseconds; throws `what` if it is still false after `ms`. */
export async function until(cond: () => boolean, ms = 5000, what = "timed out"): Promise<void> {
  const deadline = Date.now() + ms;
  while (!cond()) {
    if (Date.now() > deadline) throw new Error(what);
    await new Promise((r) => setTimeout(r, 5));
  }
}
