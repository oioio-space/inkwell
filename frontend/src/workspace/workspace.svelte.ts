import {
  type AutomergeUrl,
  type DocHandle,
  isValidAutomergeUrl,
  type PeerId,
  Repo,
} from "@automerge/automerge-repo";
import { IndexedDBStorageAdapter } from "@automerge/automerge-repo-storage-indexeddb";
import { Stream } from "@wailsio/runtime";
import { Session } from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { t } from "../i18n";
import { type SocketLike, WailsNetworkAdapter } from "../net/WailsNetworkAdapter";

export type Doc = { text: string };
export type Status = "alone" | "connecting" | "synced";

/** Port offered first when inviting, so addresses are easy to read aloud. */
const DEFAULT_PORT = 7419;
const PROJECT_KEY = "inkwell.project";
const NAME_KEY = "inkwell.name";

function safeGet(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function safeSet(key: string, value: string): void {
  try {
    localStorage.setItem(key, value);
  } catch {
    // Private mode or blocked storage: the session still works, it just won't remember.
  }
}

/** The open project and who is in it — the single source of UI state. */
class Workspace {
  status = $state<Status>("alone");
  peers = $state<string[]>([]);
  myId = $state("");
  myName = $state(safeGet(NAME_KEY) ?? "");
  hostAddrs = $state<string[]>([]);
  /** Display names of present people, by peer id (from presence). */
  names = $state<Record<string, string>>({});
  error = $state("");
  handle = $state.raw<DocHandle<Doc> | undefined>();

  #repo: Repo | undefined;
  #adapter: WailsNetworkAdapter | undefined;

  async start(): Promise<void> {
    const id = await Session.LocalPeer();
    this.myId = id;
    if (!this.myName) this.setName(`Personne ${id.slice(0, 4)}`);

    const adapter = new WailsNetworkAdapter(() => Stream("net") as unknown as SocketLike, {
      onDrop: (reason) => console.warn("bridge: dropped frame:", reason),
    });
    adapter.on("peer-candidate", ({ peerId }) => {
      if (!this.peers.includes(peerId)) this.peers = [...this.peers, peerId];
      this.status = "synced";
    });
    adapter.on("peer-disconnected", ({ peerId }) => {
      this.peers = this.peers.filter((p) => p !== peerId);
      if (this.peers.length === 0) this.status = "alone";
    });
    this.#adapter = adapter;
    this.#repo = new Repo({
      peerId: id as PeerId,
      network: [adapter],
      storage: new IndexedDBStorageAdapter("inkwell"),
      sharePolicy: async () => true,
    });

    const saved = safeGet(PROJECT_KEY);
    if (saved && isValidAutomergeUrl(saved)) {
      try {
        await this.open(saved);
        return;
      } catch {
        // The saved project is gone (storage cleared): start a fresh one.
      }
    }
    await this.open(this.#repo.create<Doc>({ text: "" }).url);
  }

  async open(url: AutomergeUrl): Promise<void> {
    if (!this.#repo) throw new Error("workspace not started");
    this.handle = await this.#repo.find<Doc>(url);
    this.#adapter?.setProject(url);
    safeSet(PROJECT_KEY, url);
  }

  setName(name: string): void {
    this.myName = name.trim().slice(0, 40);
    safeSet(NAME_KEY, this.myName);
  }

  async host(): Promise<void> {
    this.error = "";
    try {
      this.hostAddrs = (await Session.Host(DEFAULT_PORT).catch(() => Session.Host(0))) ?? [];
    } catch (err) {
      this.error = t("error.host", { reason: String(err) });
    }
  }

  /** Connects to the person at addr and opens their project. */
  async join(addr: string): Promise<void> {
    this.error = "";
    this.status = "connecting";
    try {
      const peer = (await Session.Join(addr.trim())) as PeerId;
      await this.open(await this.#projectOf(peer));
    } catch {
      this.error = t("error.join", { addr: addr.trim() });
    }
    this.status = this.peers.length > 0 ? "synced" : "alone";
  }

  /** Resolves with the project peer announced, waiting for its handshake if needed. */
  #projectOf(peer: PeerId, timeoutMs = 10_000): Promise<AutomergeUrl> {
    const adapter = this.#adapter;
    if (!adapter) return Promise.reject(new Error("workspace not started"));
    const known = adapter.projectOf(peer);
    if (known) return Promise.resolve(known);
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error("no project announced")), timeoutMs);
      adapter.onProject((url, from) => {
        if (from !== peer) return;
        clearTimeout(timer);
        resolve(url);
      });
    });
  }
}

export const workspace = new Workspace();
