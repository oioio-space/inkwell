import type { next as A } from "@automerge/automerge";
import {
  type AutomergeUrl,
  type DocHandle,
  isValidAutomergeUrl,
  type PeerId,
  parseAutomergeUrl,
  Repo,
} from "@automerge/automerge-repo";
import { Stream } from "@wailsio/runtime";
import {
  Projects,
  Session,
} from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import type { Span } from "../editor/external";
import { t } from "../i18n";
import type { MemberRole } from "../net/envelope";
import { type SocketLike, WailsNetworkAdapter } from "../net/WailsNetworkAdapter";
import { IndexValidator } from "../project/admit";
import { kindOfEntry } from "../project/kinds";
import type { IndexDoc, Role } from "../project/model";
import { ProjectRuntime } from "../project/runtime.svelte";
import { ExternalSync, type Notice } from "../store/externalSync";
import { type FileInfo, FileKind, type ProjectFile } from "../store/files";
import { ExternalEditError, GoStorageAdapter, NotTextError } from "../store/GoStorageAdapter";
import { type Projection, startProjection } from "../store/projection";
import { safeGet, safeSet } from "../store/safeStorage";
import { settings } from "../store/settings.svelte";

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

/** How long the lines taken from the disk stay marked in the gutter. */
const MARKS_MS = 60_000;

/** What this device may do, from the member list; a writer until the list is known. */
export function roleIn(roles: readonly MemberRole[], me: string): Role {
  const mine = roles.find((m) => m.peer === me);
  if (!mine) return "writer";
  return mine.suspended ? "reader" : mine.role;
}

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
  /** What happened to the project file outside Inkwell, for the toast or banner; null if nothing. */
  notice = $state.raw<Notice | null>(null);
  /** The lines the last outside edit brought in, for the gutter; cleared after a while. */
  marks = $state.raw<{ id: number; spans: Span[] } | null>(null);
  /** True while the document differs from the file on disk (always briefly, in automatic saving). */
  dirty = $state(false);
  /** True while the project file cannot be written (disk full, no permission...). */
  projectionFailed = $state(false);
  /** Why it cannot be written, as Go says it; empty if unknown. */
  projectionReason = $state("");
  /** True once the member list is full: this device signs no more, so edits stay local. */
  sealed = $state(false);
  /** The root document of the project: the index, which also holds the main file's text. */
  rootHandle = $state.raw<DocHandle<Doc> | undefined>();
  /** The devices of the project and their roles, as Go reports them. */
  roles = $state.raw<readonly MemberRole[]>([]);
  /** The files of the open project: the index, the documents, the tree on disk. */
  runtime = $state.raw<ProjectRuntime | undefined>();
  /** The name of a file the system could not open, until the person dismisses the notice. */
  openFailed = $state<string | null>(null);
  /** How the current file is stored on disk; undefined until Go has written it. */
  fileInfo = $state.raw<FileInfo | undefined>();

  #repo: Repo | undefined;
  #adapter: WailsNetworkAdapter | undefined;
  #storage: GoStorageAdapter | undefined;
  #projection: Projection | undefined;
  #sync: ExternalSync | undefined;
  /** Outside edits Go reported before the project was open. */
  #early: string[] = [];
  #marksTimer: ReturnType<typeof setTimeout> | undefined;

  /**
   * The path of the file shown (the main file until a project is open). `?file=` names the file
   * the single document stands for, in development and in the end-to-end tests of the sheet.
   */
  get current(): string {
    const forced = openedFile();
    if (forced !== MAIN_FILE) return forced;
    return this.runtime?.currentPath ?? MAIN_FILE;
  }

  /** The document of the file shown, once it is loaded. */
  get handle(): DocHandle<Doc> | undefined {
    if (openedFile() !== MAIN_FILE) return this.rootHandle;
    const id = this.runtime?.currentId;
    void this.runtime?.ready;
    return id ? this.runtime?.handleOf(id) : undefined;
  }

  /** The project's files, for the Insert menu and the formatting bar. */
  get files(): ProjectFile[] {
    const rt = this.runtime;
    if (!rt) return [{ path: MAIN_FILE, kind: FileKind.typst, size: 0, main: true }];
    return rt.tree.entries
      .filter((e) => e.type !== "dir")
      .map((e) => ({ path: e.path, kind: kindOfEntry(e), size: e.size, main: e.main }));
  }

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
    adapter.onRoles((roles) => {
      this.roles = roles;
      this.runtime?.setRole(roleIn(roles, this.myId));
    });
    this.#adapter = adapter;
    const storage = new GoStorageAdapter(() => Stream("store") as unknown as SocketLike);
    storage.onExternal((name) => {
      if (this.#sync) this.#sync.notify(name);
      else this.#early.push(name);
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
    await this.open(this.#repo.create<IndexDoc>({ text: "" }).url);
  }

  async open(url: AutomergeUrl): Promise<void> {
    const repo = this.#repo;
    const storage = this.#storage;
    if (!repo || !storage) throw new Error("workspace not started");
    // The rules for the index are in force before its first message can arrive.
    const { documentId } = parseAutomergeUrl(url);
    this.#adapter?.validate(
      documentId,
      new IndexValidator(() => {
        try {
          return repo.handles[documentId]?.doc() as A.Doc<IndexDoc> | undefined;
        } catch {
          return undefined;
        }
      }),
    );
    const root = await repo.find<IndexDoc>(url);
    const handle = root as unknown as DocHandle<Doc>;
    this.#projection?.stop();
    this.#sync?.stop();
    this.#sync = undefined;
    this.runtime?.stop();
    this.notice = null;
    this.rootHandle = handle;
    this.#adapter?.setProject(url);
    await Projects.SetRootURL(url);
    // Every file but the main one is projected by the runtime; the main one by the sync below.
    const runtime = new ProjectRuntime(root, {
      repo,
      storage,
      me: () => ({ id: this.myId, name: this.myName }),
      blobStatus: () => Projects.BlobProgress(),
      confirmBlobs: () => Projects.ConfirmBlobs(),
    });
    runtime.onProjectionError = (_path, err) => {
      console.warn("projection:", err instanceof Error ? err.message : "failed");
      if (!(err instanceof ExternalEditError)) this.projectionFailed = true;
    };
    runtime.onProjectionWritten = () => {
      this.projectionFailed = false;
    };
    runtime.setRole(roleIn(this.roles, this.myId));
    this.runtime = runtime;
    runtime.start();
    // Saving with Ctrl+S: the file keeps what it holds until asked, so dirty is known from it.
    const known =
      settings.saveMode === "manual"
        ? await storage.readProjection(MAIN_FILE).catch(() => undefined)
        : undefined;
    const sync = new ExternalSync({
      handle,
      name: MAIN_FILE,
      storage,
      projection: () => this.#projection,
      policy: () => settings.externalPolicy,
      manual: () => settings.saveMode === "manual",
      onNotice: (notice) => this.#show(notice),
      notText: (err) => (err instanceof NotTextError ? err : undefined),
      legacyKey: `inkwell.sync.${url}.${MAIN_FILE}`,
    });
    this.#sync = sync;
    this.#projection = startProjection(handle, storage, {
      name: MAIN_FILE,
      schedule: () => (settings.saveMode === "manual" ? undefined : settings.saveDelay * 1000),
      known,
      onDirty: (dirty) => {
        this.dirty = dirty;
      },
      onError: (err) => {
        console.warn("projection:", err instanceof Error ? err.message : "failed");
        // A refused write means the file was edited outside: the notice already asks the person.
        if (err instanceof ExternalEditError) return;
        this.projectionFailed = true;
        this.projectionReason = err instanceof Error ? err.message : "";
      },
      onWritten: () => {
        this.projectionFailed = false;
      },
      retryable: (err) => !(err instanceof ExternalEditError),
    });
    for (const name of this.#early.splice(0)) sync.notify(name);
  }

  #show(notice: Notice | null): void {
    this.notice = notice;
    if (notice?.kind !== "merged" || notice.ranges.length === 0) return;
    this.marks = { id: notice.id, spans: notice.ranges };
    clearTimeout(this.#marksTimer);
    this.#marksTimer = setTimeout(() => {
      this.marks = null;
    }, MARKS_MS);
  }

  /** Writes the file now (Ctrl+S): in automatic saving too, at once instead of after the pause. */
  async save(): Promise<void> {
    if (this.notice?.kind === "deleted") return this.#sync?.recreate();
    await this.#projection?.flush();
  }

  /** The save settings changed: apply them to what is waiting (manual to automatic writes at once). */
  saveSettingsChanged(): void {
    this.#projection?.reschedule();
  }

  /** Takes the last outside edit back: only what it brought in, not what others typed since. */
  undoExternal(): void {
    this.#sync?.undo();
    clearTimeout(this.#marksTimer);
    this.marks = null;
  }

  /** Ctrl+Z: takes the outside edit back if it is the last thing that happened to the text. */
  undoWithKey(): boolean {
    if (!this.#sync?.canKeyUndo) return false;
    this.undoExternal();
    return true;
  }

  /** "Ask" answered with "Integrate". */
  integrateExternal(): Promise<void> {
    return this.#sync?.integrateAsked() ?? Promise.resolve();
  }

  /** Keep Inkwell's version: the file is written from the document (at the next save in manual mode). */
  keepOurs(): Promise<void> {
    return this.#sync?.keepOurs() ?? Promise.resolve();
  }

  /** The file was deleted outside: write it again from the document. */
  recreate(): Promise<void> {
    return this.#sync?.recreate() ?? Promise.resolve();
  }

  /** Closes the toast or banner; what it said stays done. */
  dismissNotice(): void {
    this.notice = null;
  }

  /** Hands a project file to the system's own program for it (a PDF, a font, an archive). */
  async openWithSystem(path: string): Promise<void> {
    try {
      await Projects.OpenWithSystem(path);
    } catch {
      this.openFailed = path.slice(path.lastIndexOf("/") + 1);
    }
  }

  /** The key a project's remembered layout is filed under. */
  get project(): string {
    return this.rootHandle?.url ?? "";
  }

  /** Looks for files in the project folder that the project does not have. */
  async refreshFiles(): Promise<void> {
    await this.runtime?.refreshUntracked();
  }

  /** Reloads how the current file is stored (encoding, line ending). */
  async refreshInfo(): Promise<void> {
    if (!this.#storage) return;
    if (this.runtime?.current?.type === "blob") {
      this.fileInfo = undefined;
      return;
    }
    try {
      this.fileInfo = await this.#storage.fileInfo(this.current);
    } catch {
      this.fileInfo = undefined;
    }
  }

  /** Writes the document to the project folder now, if a write is still waiting for a pause in typing. */
  async flushProjection(): Promise<void> {
    await this.#projection?.flush();
  }

  /** The text of a project file as UTF-8 with LF; undefined if absent or not text. */
  async readFile(path: string): Promise<string | undefined> {
    try {
      return await this.#storage?.readProjection(path);
    } catch {
      return undefined;
    }
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
