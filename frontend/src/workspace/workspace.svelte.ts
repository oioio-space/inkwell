import { next as A } from "@automerge/automerge";
import {
  type AutomergeUrl,
  type DocHandle,
  isValidAutomergeUrl,
  type PeerId,
  Repo,
} from "@automerge/automerge-repo";
import { Stream } from "@wailsio/runtime";
import {
  Projects,
  Session,
} from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { t } from "../i18n";
import { type SocketLike, WailsNetworkAdapter } from "../net/WailsNetworkAdapter";
import { GoStorageAdapter } from "../store/GoStorageAdapter";
import { type Projection, startProjection } from "../store/projection";

export type Doc = { text: string };
export type Status = "alone" | "connecting" | "synced";

/** The projection of the document, in the project folder. */
const MAIN_FILE = "main.typ";
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
  /** The text to give to the person joining: opaque, copied and pasted as is. */
  invitation = $state("");
  /** Display names of present people, by peer id (from presence). */
  names = $state<Record<string, string>>({});
  error = $state("");
  /** A project file edited outside Inkwell, waiting for "Import" or "Ignore"; null if none. */
  external = $state<string | null>(null);
  /** True once the member list is full: this device signs no more, so edits stay local. */
  sealed = $state(false);
  handle = $state.raw<DocHandle<Doc> | undefined>();

  #repo: Repo | undefined;
  #adapter: WailsNetworkAdapter | undefined;
  #storage: GoStorageAdapter | undefined;
  #projection: Projection | undefined;

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
    adapter.onSealed(() => {
      this.sealed = true;
    });
    this.#adapter = adapter;
    const storage = new GoStorageAdapter(() => Stream("store") as unknown as SocketLike);
    storage.onExternal((name) => {
      if (name === MAIN_FILE) this.external = name;
    });
    this.#storage = storage;
    this.#repo = new Repo({
      peerId: id as PeerId,
      network: [adapter],
      storage,
      sharePolicy: async () => true,
    });

    const saved = (await Projects.Current()).RootURL;
    if (saved && isValidAutomergeUrl(saved)) {
      try {
        await this.open(saved);
        return;
      } catch {
        // The saved document is gone from the project folder: start a fresh one.
      }
    }
    await this.open(this.#repo.create<Doc>({ text: "" }).url);
  }

  async open(url: AutomergeUrl): Promise<void> {
    if (!this.#repo) throw new Error("workspace not started");
    const handle = await this.#repo.find<Doc>(url);
    this.#projection?.stop();
    this.handle = handle;
    this.#adapter?.setProject(url);
    await Projects.SetRootURL(url);
    if (this.#storage) {
      this.#projection = startProjection(handle, this.#storage, {
        name: MAIN_FILE,
        // A refused write means the file was edited outside: the notice already asks the person.
        onError: (err) =>
          console.warn("projection:", err instanceof Error ? err.message : "failed"),
      });
    }
  }

  /** Takes the outside edit of main.typ into the document, then lets Inkwell write it again. */
  async importExternal(): Promise<void> {
    const name = this.external;
    const handle = this.handle;
    if (!name || !handle || !this.#storage) return;
    const text = await this.#storage.readProjection(name);
    if (text !== undefined) handle.change((d) => A.updateText(d, ["text"], text));
    await this.#storage.acknowledge(name);
    this.external = null;
    await this.#projection?.flush(true);
  }

  /** Keeps the document as it is: the file is written from it again, replacing the outside edit. */
  async ignoreExternal(): Promise<void> {
    const name = this.external;
    if (!name || !this.#storage) return;
    await this.#storage.acknowledge(name);
    this.external = null;
    await this.#projection?.flush(true);
  }

  setName(name: string): void {
    this.myName = name.trim().slice(0, 40);
    safeSet(NAME_KEY, this.myName);
  }

  async host(): Promise<void> {
    this.error = "";
    try {
      this.invitation = await Session.Host();
    } catch (err) {
      this.error = t("error.host", { reason: String(err) });
    }
  }

  /** Connects using the invitation text and opens the inviting person's project. */
  async join(invitation: string): Promise<void> {
    this.error = "";
    this.status = "connecting";
    try {
      const peer = (await Session.Join(invitation.trim())) as PeerId;
      await this.open(await this.#projectOf(peer));
    } catch {
      this.error = t("error.join");
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
