import {
  type AutomergeUrl,
  type DocHandle,
  type DocumentId,
  isValidAutomergeUrl,
  type PeerId,
  Repo,
} from "@automerge/automerge-repo";
import { Stream } from "@wailsio/runtime";
import {
  Projects,
  Session,
} from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import type { Span } from "../editor/external";
import type { Meta } from "../history/types";
import { t } from "../i18n";
import { AdmittedSet } from "../net/scope";
import { shareAdmitted } from "../net/sharing";
import { type SocketLike, WailsNetworkAdapter } from "../net/WailsNetworkAdapter";
import { ExternalSync, type Notice } from "../store/externalSync";
import { type FileInfo, FileKind, type ProjectFile } from "../store/files";
import { ExternalEditError, GoStorageAdapter, NotTextError } from "../store/GoStorageAdapter";
import { type SizeTier, sizeTier } from "../store/limits";
import { type Projection, startProjection } from "../store/projection";
import { safeGet, safeSet } from "../store/safeStorage";
import { settings } from "../store/settings.svelte";
import { accessOf, type Member } from "../ui/people/roster";
import { BootstrapLoader } from "./loader";
import { observeVariantHeads } from "./observe";

/** The project's text, and what the history keeps beside it (named versions, variants, actors). */
export type Doc = { text: string; meta?: Meta };
export type Status = "alone" | "connecting" | "synced" | "suspended";

/** What the header and the status bar say: it follows who is connected, never a stored flag. */
export function statusOf(peerCount: number, joining: boolean): Exclude<Status, "suspended"> {
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

/** The open project and who is in it — the single source of UI state. */
class Workspace {
  peers = $state<string[]>([]);
  /** True while an invitation is being followed, until the inviting person's project is open. */
  joining = $state(false);
  /** Derived from `peers`, so the header and the people count cannot disagree. */
  status = $derived<Status>(
    this.access === "suspended" ? "suspended" : statusOf(this.peers.length, this.joining),
  );
  myId = $state("");
  myName = $state(safeGet(NAME_KEY) ?? "");
  /** The invitation text to give to the person joining: opaque, copied and pasted as is. */
  invitation = $state("");
  /** True once someone new connected after the invitation was made: it works once, so it is dead. */
  invitationUsed = $state(false);
  /** The people already connected when the invitation was made: none of them used it. */
  #knownAtInvite = new Set<string>();
  /** Display names of present people, by peer id (from presence). */
  names = $state<Record<string, string>>({});
  /** Every name heard since the start, kept when the person goes quiet (UX-S7-3). */
  #heard: Record<string, string> = {};
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
  /** Counts the reports of the members' roles from Go, so that what depends on rights recomputes. */
  rolesVersion = $state(0);
  /** The people of the project with their roles and suspensions, as Go reports them. */
  roster = $state.raw<Member[]>([]);
  /** True once the member list is full: this device signs no more, so edits stay local. */
  sealed = $state(false);
  handle = $state.raw<DocHandle<Doc> | undefined>();
  /** A variant being edited instead of the main document (B2); undefined when the main one is. */
  variant = $state.raw<DocHandle<Doc> | undefined>();
  /** The document the editor shows: the variant if one is open, else the project's. */
  get active(): DocHandle<Doc> | undefined {
    return this.variant ?? this.handle;
  }
  /** Asked before Ctrl+Z undoes; the first to return true has used the key (see `undoWithKey`). */
  keyUndoHooks: (() => boolean)[] = [];
  /**
   * A text too big to enter a document (ADR 0028): read from the disk as one plain string, shown
   * read-only, in no Automerge document. An object, so each text is its own key.
   */
  big = $state.raw<{ text: string } | undefined>();
  /** How big the open document is: "large" earns a discreet notice (ADR 0028 D1). */
  tier = $state<SizeTier>("fluid");
  /** The file shown in the editor (today the project's main document). */
  current = $state(openedFile());
  /** The project's files, for the sidebar and the Insert menu. */
  files = $state.raw<ProjectFile[]>([]);
  /** How the current file is stored on disk; undefined until Go has written it. */
  fileInfo = $state.raw<FileInfo | undefined>();

  #rosterTicket = 0;
  #admitted = new AdmittedSet();
  #loader: BootstrapLoader | undefined;
  #offAdmission: (() => void) | undefined;
  #repo: Repo | undefined;
  get repo(): Repo | undefined {
    return this.#repo;
  }

  #adapter: WailsNetworkAdapter | undefined;
  #storage: GoStorageAdapter | undefined;
  #projection: Projection | undefined;
  #sync: ExternalSync | undefined;
  /** Outside edits Go reported before the project was open. */
  #early: string[] = [];
  #marksTimer: ReturnType<typeof setTimeout> | undefined;
  #offSize: (() => void) | undefined;

  async start(): Promise<void> {
    const id = await Session.LocalPeer();
    this.myId = id;
    if (!this.myName) this.setName(`Personne ${id.slice(0, 4)}`);

    const adapter = new WailsNetworkAdapter(() => Stream("net") as unknown as SocketLike, {
      admitted: this.#admitted.has,
      ready: (id) =>
        Object.hasOwn(this.#repo?.handles ?? {}, id) &&
        !!this.#repo?.handles[id as DocumentId]?.inState(["ready", "requesting", "unavailable"]),
      load: (id) => this.#loader?.ensure(id),
      onDrop: (reason) => console.warn("bridge: dropped frame:", reason),
    });
    adapter.on("peer-candidate", ({ peerId }) => {
      this.peerUp(peerId);
    });
    adapter.on("peer-disconnected", ({ peerId }) => {
      this.peers = this.peers.filter((p) => p !== peerId);
    });
    adapter.onRoles(() => {
      this.rolesVersion++;
      void this.refreshRoster();
    });
    adapter.onSealed(() => {
      this.sealed = true;
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
      isEphemeral: true,
      shareConfig: shareAdmitted(this.#admitted),
    });

    this.#loader = new BootstrapLoader(this.#repo, this.#admitted, () => adapter.drain());
    let epoch = this.#admitted.epoch;
    this.#admitted.onChange(() => {
      if (epoch !== this.#admitted.epoch) {
        adapter.scopeChanged();
        epoch = this.#admitted.epoch;
      }
      this.#repo?.shareConfigChanged();
      for (const id of this.#admitted.ids) this.#loader?.ensure(id);
    });

    observeVariantHeads(
      () => this.handle,
      () => this.#repo,
    );
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
    this.#offAdmission?.();
    this.#admitted.setRoot(url);
    const epoch = this.#admitted.epoch;
    const handle = await this.#repo.find<Doc>(url);
    if (epoch !== this.#admitted.epoch) return;
    const refresh = () => this.#admitted.refresh(handle.doc());
    refresh();
    handle.on("change", refresh);
    this.#offAdmission = () => handle.off("change", refresh);
    this.variant = undefined;
    this.#projection?.stop();
    this.#sync?.stop();
    this.#sync = undefined;
    this.notice = null;
    this.handle = handle;
    this.big = undefined;
    this.#watchSize(handle);
    this.#adapter?.setProject(url);
    await Projects.SetRootURL(url);
    const storage = this.#storage;
    if (!storage) return;
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
      onBig: (text) => {
        this.big = text === undefined ? undefined : { text };
      },
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

  /** Keeps `tier` at the size of the open document, measured at most twice a second. */
  #watchSize(handle: DocHandle<Doc>): void {
    this.#offSize?.();
    let timer: ReturnType<typeof setTimeout> | undefined;
    const measure = () => {
      timer = undefined;
      this.tier = sizeTier(handle.doc()?.text.length ?? 0);
    };
    const onChange = () => {
      timer ??= setTimeout(measure, 500);
    };
    measure();
    handle.on("change", onChange);
    this.#offSize = () => {
      clearTimeout(timer);
      handle.off("change", onChange);
    };
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
    if (this.keyUndoHooks.some((hook) => hook())) return true;
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

  /**
   * Whether `peer` may pilot a presentation: a writer or an admin of the signed member list, not
   * suspended. A device the list does not name may not. This is the check every device makes on
   * what it receives, so that a reader's messages never move anyone's slides.
   */
  canPilot(peer: string): boolean {
    void this.rolesVersion;
    const member = this.#adapter?.roleOf(peer);
    return member !== undefined && member.role !== "reader" && !member.suspended;
  }

  /**
   * Why this device cannot edit: "reader" (its role), or "suspended" (an admin disconnected it);
   * undefined when it can, or while Go has not yet said.
   */
  get access(): "reader" | "suspended" | undefined {
    void this.rolesVersion;
    const me = this.#adapter?.roleOf(this.myId);
    return accessOf(me?.role, me?.suspended ?? false);
  }

  /**
   * Whether this device may make an invitation: only an admin who is not disconnected may (Go
   * refuses the others). True while Go has not said, so that a first project is never blocked.
   */
  get mayInvite(): boolean {
    void this.rolesVersion;
    const me = this.#adapter?.roleOf(this.myId);
    return me === undefined || (me.role === "admin" && !me.suspended);
  }

  /** Asks Go for the people of the project (it changes when an admin acts, here or elsewhere). */
  async refreshRoster(): Promise<void> {
    const ticket = ++this.#rosterTicket;
    try {
      const list = ((await Session.Members()) ?? []) as unknown as Member[];
      // Answers can cross: only the latest question may set the list.
      if (ticket === this.#rosterTicket) this.roster = list;
    } catch {
      // Go does not answer (no project yet): the list stays as it was.
    }
  }

  /** Whether this device knows its role at all (Go has reported the member list). */
  get rolesKnown(): boolean {
    void this.rolesVersion;
    return this.#adapter?.roleOf(this.myId) !== undefined;
  }

  /** The name to show for a device: the one it announced, else a neutral one. */
  nameOf(peer: string): string {
    return this.knownName(peer) ?? `Personne ${peer.slice(0, 4)}`;
  }

  /** The name a device announced, now or earlier: someone let back in is not renamed. */
  knownName(peer: string): string | undefined {
    return this.names[peer] || this.#heard[peer] || undefined;
  }

  /** The people present, from presence: a name once heard is remembered when they leave. */
  hearNames(people: readonly { peerId: string; name: string }[]): void {
    this.names = Object.fromEntries(people.map((p) => [p.peerId, p.name]));
    for (const p of people) if (p.name) this.#heard[p.peerId] = p.name;
  }

  setName(name: string): void {
    this.myName = name.trim().slice(0, 40);
    safeSet(NAME_KEY, this.myName);
  }

  /** Lists a connected person; a new one after the invitation was made means it was used. */
  peerUp(peerId: string): void {
    if (this.invitation && !this.#knownAtInvite.has(peerId)) this.invitationUsed = true;
    if (!this.peers.includes(peerId)) this.peers = [...this.peers, peerId];
  }

  /** Makes a fresh invitation, replacing the last one (used or not). */
  async host(): Promise<void> {
    this.error = "";
    try {
      const known = new Set(this.peers);
      this.invitation = (await Session.Invite("writer")).text;
      this.#knownAtInvite = known;
      this.invitationUsed = false;
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
