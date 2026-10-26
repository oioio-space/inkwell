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
import { type FileInfo, FileKind, type ProjectFile } from "../store/files";
import { ExternalEditError, GoStorageAdapter } from "../store/GoStorageAdapter";
import { type Projection, startProjection } from "../store/projection";
import { safeGet, safeSet } from "../store/safeStorage";

export type Doc = { text: string };
export type Status = "alone" | "connecting" | "synced";

/** What the header and the status bar say: it follows who is connected, never a stored flag. */
export function statusOf(peerCount: number, joining: boolean): Status {
  if (peerCount > 0) return "synced";
  return joining ? "connecting" : "alone";
}

/** The projection of the document, in the project folder. */
const MAIN_FILE = "main.typ";

/**
 * Which file the editor area presents. Until the multi-file project (B4) opens a file per
 * document, the single document stands for whichever file is named by `?file=` (a plain
 * relative path): this is how a table is shown as a sheet in development and in the
 * end-to-end tests. Without it, the main document.
 */
function openedFile(): string {
  try {
    const wanted = new URLSearchParams(globalThis.location?.search ?? "").get("file");
    if (wanted && /^[\p{L}\p{N}_.-]+(\/[\p{L}\p{N}_.-]+)*$/u.test(wanted) && !wanted.includes(".."))
      return wanted;
  } catch {
    // No location (a unit test): the main document.
  }
  return MAIN_FILE;
}
const NAME_KEY = "inkwell.name";

/** The open project and who is in it — the single source of UI state. */
class Workspace {
  peers = $state<string[]>([]);
  /** True while an invitation is being followed, until the inviting person's project is open. */
  joining = $state(false);
  /** Derived from `peers`, so the header and the people count cannot disagree. */
  status = $derived(statusOf(this.peers.length, this.joining));
  myId = $state("");
  myName = $state(safeGet(NAME_KEY) ?? "");
  /** The invitation text to give to the person joining: opaque, copied and pasted as is. */
  invitation = $state("");
  /** Display names of present people, by peer id (from presence). */
  names = $state<Record<string, string>>({});
  error = $state("");
  /** A project file edited outside Inkwell, waiting for "Import" or "Ignore"; null if none. */
  external = $state<string | null>(null);
  /** True while the project file cannot be written (disk full, no permission...). */
  projectionFailed = $state(false);
  /** True once the member list is full: this device signs no more, so edits stay local. */
  sealed = $state(false);
  handle = $state.raw<DocHandle<Doc> | undefined>();
  /** The file shown in the editor (today the project's main document). */
  current = $state(openedFile());
  /** The project's files, for the sidebar and the Insert menu. */
  files = $state.raw<ProjectFile[]>([]);
  /** How the current file is stored on disk; undefined until Go has written it. */
  fileInfo = $state.raw<FileInfo | undefined>();

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
    });
    adapter.on("peer-disconnected", ({ peerId }) => {
      this.peers = this.peers.filter((p) => p !== peerId);
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
        onError: (err) => {
          console.warn("projection:", err instanceof Error ? err.message : "failed");
          // A refused write means the file was edited outside: the notice already asks the person.
          if (!(err instanceof ExternalEditError)) this.projectionFailed = true;
        },
        onWritten: () => {
          this.projectionFailed = false;
        },
        retryable: (err) => !(err instanceof ExternalEditError),
      });
    }
  }

  /** The key a project's remembered layout is filed under. */
  get project(): string {
    return this.handle?.url ?? "";
  }

  /** Reloads the file list; the main document always shows, even before its first write. */
  async refreshFiles(): Promise<void> {
    if (!this.#storage) return;
    try {
      const files = await this.#storage.listFiles();
      this.files = files.some((f) => f.path === MAIN_FILE)
        ? files
        : [{ path: MAIN_FILE, kind: FileKind.typst, size: 0, main: true }, ...files];
    } catch (err) {
      console.warn("files:", err instanceof Error ? err.message : "failed");
    }
  }

  /** Reloads how the current file is stored (encoding, line ending). */
  async refreshInfo(): Promise<void> {
    if (!this.#storage) return;
    // Only the main document has a file on disk until B4 gives every file its own document.
    if (this.current !== MAIN_FILE) {
      this.fileInfo = undefined;
      return;
    }
    try {
      this.fileInfo = await this.#storage.fileInfo(this.current);
    } catch {
      this.fileInfo = undefined;
    }
  }

  /** The text of a project file as UTF-8 with LF; undefined if absent or not text. */
  async readFile(path: string): Promise<string | undefined> {
    try {
      return await this.#storage?.readProjection(path);
    } catch {
      return undefined;
    }
  }

  /** Takes the outside edit of main.typ into the document, then lets Inkwell write it again. */
  async importExternal(): Promise<void> {
    const name = this.external;
    const handle = this.handle;
    if (!name || !handle || !this.#storage) return;
    const text = await this.#storage.readProjection(name);
    if (text !== undefined) handle.change((d) => A.updateText(d, ["text"], text));
    await this.#settleExternal(name);
  }

  /** Keeps the document as it is: the file is written from it again, replacing the outside edit. */
  async ignoreExternal(): Promise<void> {
    const name = this.external;
    if (!name) return;
    await this.#settleExternal(name);
  }

  /** The outside edit is dealt with: forget it and write the file from the document again. */
  async #settleExternal(name: string): Promise<void> {
    if (!this.#storage) return;
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
      this.invitation = (await Session.Invite("writer")).text;
    } catch (err) {
      this.error = t("error.host", { reason: String(err) });
    }
  }

  /** Connects using the invitation text and opens the inviting person's project. */
  async join(invitation: string): Promise<void> {
    this.error = "";
    this.joining = true;
    try {
      const peer = (await Session.Join(invitation.trim())) as PeerId;
      await this.open(await this.#projectOf(peer));
    } catch {
      this.error = t("error.join");
    } finally {
      this.joining = false;
    }
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
