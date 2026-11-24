import {
  type AutomergeUrl,
  type DocHandle,
  type DocumentId,
  isValidAutomergeUrl,
  type PeerId,
  Repo,
} from "@automerge/automerge-repo";
import { Stream } from "@wailsio/runtime";
import { tick } from "svelte";
import {
  Projects,
  Session,
} from "../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { flushAllTyped } from "../editor/amSync";
import type { Span } from "../editor/external";
import { type FileType, type Opening, registry } from "../extensions";
import type { Meta } from "../history/types";
import { t } from "../i18n";
import { links } from "../net/links.svelte";
import { AdmittedSet } from "../net/scope";
import { shareAdmitted } from "../net/sharing";
import {
  type GlobalSyncState,
  globalSyncState,
  isSlow,
  observePeers,
  type PeerSyncState,
  peerSyncState,
  SyncDisplay,
  sendingCount,
} from "../net/syncIndicator";
import { type SocketLike, WailsNetworkAdapter } from "../net/WailsNetworkAdapter";
import { type LeaveKind, markLeave } from "../restart/mark";
import { ExternalSync, type Notice } from "../store/externalSync";
import type { FileInfo, ProjectFile } from "../store/files";
import { ExternalEditError, GoStorageAdapter, NotTextError } from "../store/GoStorageAdapter";
import { type SizeTier, sizeTier } from "../store/limits";
import { type Projection, startProjection } from "../store/projection";
import { safeGet, safeSet } from "../store/safeStorage";
import { settings } from "../store/settings.svelte";
import { notice } from "../ui/notice.svelte";
import { accessOf, type Member, type Standing } from "../ui/people/roster";
import { flushCloseBuffers, pauseClosePresence, resumeClosePresence } from "./closePage";
import { BootstrapLoader } from "./loader";
import { observeSaveNow, observeSession, observeVariantHeads } from "./observe";
import { viewChoice } from "./viewChoice.svelte";

/** The project's text, and what the history keeps beside it (named versions, variants, actors). */
export type Doc = { text: string; meta?: Meta };
export type Status =
  | "alone"
  | "connecting"
  | "checking"
  | "sending"
  | "behind"
  | "synced"
  | "suspended";

/**
 * What the header and the status bar say. Alone or connecting while nobody is listed; otherwise
 * the sync state the indicator shows (W33): never « synced » just because someone is connected.
 * A listed peer the indicator has not yet seen is being checked.
 */
export function statusOf(
  shown: GlobalSyncState,
  peerCount: number,
  joining: boolean,
): Exclude<Status, "suspended"> {
  if (peerCount === 0) return joining ? "connecting" : "alone";
  return shown === "offline" ? "checking" : shown;
}

/** The projection of the document, in the project folder. */
const MAIN_FILE = "main.typ";

const NAME_KEY = "inkwell.name";

/** How long the lines taken from the disk stay marked in the gutter. */
const MARKS_MS = 60_000;

/** The open project and who is in it — the single source of UI state. */
/**
 * Fetches a project file from Go's asset handler. The page carries a per-launch token (a meta
 * tag Go fills in); the handler refuses a request without it. It travels in a header so it
 * never lands in a URL.
 */
export function assetFetch(url: string, method: "GET" | "HEAD" = "GET"): Promise<Response> {
  const token = document.querySelector<HTMLMetaElement>('meta[name="asset-token"]')?.content ?? "";
  return fetch(url, { method, headers: { "X-Inkwell-Token": token } });
}

/** How often an open file other than main.typ is checked for changes made outside Inkwell. */
const WATCH_MS = 2000;

/**
 * A file other than the main document, as it opens: how (its type and viewer, ADR 0027), its text
 * when Go read it as text, or that it could not be read.
 */
type Opened = { opening: Opening; text?: string; failed?: boolean; large?: boolean };
/** The answers of `Session.InvitationState` (PC1b, rule J-8). */
export type InvitationState = "open" | "used" | "expired";

/** How long the page waits for its edits to be saved before it leaves for a project change (rule N-15). */
export const LEAVE_FLUSH_MS = 5000;
/** How long the page waits before it asks again for the document of a join (rule J-2). */
export const JOIN_RETRY_MS = 1000;

class Workspace {
  peers = $state<string[]>([]);
  /** True while an invitation is being followed, until the inviting person's project is open. */
  joining = $state(false);
  /** The sync state the header shows, after the hold of D-SI5; see {@link refreshSync}. */
  syncShown = $state<GlobalSyncState>("offline");
  /** n of « Envoi en cours (n) », as shown: frozen with its label (DS-4). */
  syncSending = $state(0);
  /** True while a link holds a message for more than 2 s (« Lien lent »). */
  syncSlow = $state(false);
  /** Each connected person's own state, for the people popover, by device. */
  peerSync = $state<Record<string, PeerSyncState>>({});
  #hooked = new WeakSet<object>();
  #display = new SyncDisplay((shown, n) => {
    this.syncShown = shown;
    this.syncSending = n;
  });
  /**
   * Who this start waits for: the person whose invitation was accepted, while the document of
   * the join has not arrived (PC1b, rule J-2); "" when no name is known for that person (the
   * sentence then names nobody); undefined when nothing is waited for.
   */
  waitingFor = $state<string | undefined>();
  /** Follows `peers` and the indicator, so the header and the people count cannot disagree. */
  status = $derived<Status>(
    this.access === "suspended"
      ? "suspended"
      : // Never « Seul » while the document of a join is waited for (PC1b, J-2).
        statusOf(this.syncShown, this.peers.length, this.joining || this.waitingFor !== undefined),
  );
  myId = $state("");
  myName = $state(safeGet(NAME_KEY) ?? "");
  /** The invitation text to give to the person joining: opaque, copied and pasted as is. */
  invitation = $state("");
  /**
   * What became of {@link invitation}, as Go says (PC1b, rule J-8): it works ("open") until its
   * ticket is redeemed ("used", whether or not that person then connects) or reaches its end
   * ("expired"). Only an open invitation is offered to copy.
   */
  invitationState = $state<InvitationState>("open");
  /** Display names of present people, by peer id (from presence). */
  names = $state<Record<string, string>>({});
  peerFiles = $state<Record<string, string>>({});
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
  /**
   * Whether the member list names this device, as Go says (`Session.Standing`); undefined until
   * Go has answered. Asked again whenever the roles event arrives.
   */
  standing = $state<Standing | undefined>();
  /**
   * Whether Go can take a document over here (`Session.CanTakeOver`): only where Inkwell can
   * restart itself (PC1, residual R-8). Asked with the standing.
   */
  canTakeOver = $state(false);
  /**
   * True from a successful join until the member list names this device: in a copy of the project
   * the list arrives by sync (seconds, more on a bad network), and the banner says so meanwhile.
   */
  listPending = $state(false);
  #standingTicket = 0;
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
  current = $state(MAIN_FILE);
  opened = $state.raw<Opened>();
  #openTicket = 0;
  /** Polls the open file (not main.typ, which Go watches) while it is shown. */
  #watch: ReturnType<typeof setInterval> | undefined;
  /** The version the open file had when last read (its ETag). */
  #version: string | null | undefined;
  #checking = false;

  /** Only the latest file click may install its projection. */
  async openFile(path: string): Promise<void> {
    if (!this.files.some((file) => file.path === path)) return;
    const ticket = ++this.#openTicket;
    clearInterval(this.#watch);
    this.current = path;
    this.opened = undefined;
    if (path === MAIN_FILE) {
      // Back on the main document: its encoding and line ending show again.
      void this.refreshInfo();
      return;
    }
    const version = await this.#versionOf(path);
    const result = await this.#load(path);
    if (ticket !== this.#openTicket) return;
    this.opened = result;
    this.#version = version;
    this.#watch = setInterval(() => void this.#checkOpened(ticket), WATCH_MS);
    void this.refreshInfo();
  }

  /**
   * Reads path for its view. Its type comes from its name; a type kept as bytes is not read here
   * (its viewer reads what it needs), anything else is read as text, and what Go finds in the
   * bytes decides how it opens (ADR 0027 rule 6).
   */
  async #load(path: string): Promise<Opened> {
    const choice = viewChoice.typeFor(this.project, path);
    if (!registry.needsContent(path, { choice }))
      return { opening: registry.open(path, { choice }) };
    const read = await this.#readText(path);
    if (typeof read === "object") {
      const firstLine = read.text.slice(0, 512).split("\n", 1)[0];
      const opening = registry.open(path, { choice, content: "text", firstLine });
      this.#learn(path, opening);
      return { opening, text: read.text };
    }
    const opening = registry.open(path, { choice, content: "binary" });
    if (read === "binary") this.#learn(path, opening);
    if (read === "large") return { opening, large: true };
    return read === "binary" ? { opening } : { opening, failed: true };
  }

  /**
   * What Go found reading a project file as text: the text, "binary" when its bytes are not text,
   * "large" when a text is over what a document holds, undefined when it cannot be read.
   */
  async #readText(path: string): Promise<{ text: string } | "binary" | "large" | undefined> {
    try {
      const response = await assetFetch(`/project-text?path=${encodeURIComponent(path)}`);
      if (response.status === 415) return "binary";
      if (response.status === 413) return "large";
      return response.ok ? { text: await response.text() } : undefined;
    } catch {
      return undefined;
    }
  }

  /** The types Go's reading gave to names no type claims (a README is text once read). */
  #known = $state.raw<ReadonlyMap<string, string>>(new Map());

  #learn(path: string, opening: Opening): void {
    if (registry.typeOf(path) || this.#known.get(path) === opening.type.id) return;
    this.#known = new Map(this.#known).set(path, opening.type.id);
  }

  /**
   * The type of a project file: the person's choice for it, else what its name says, else what
   * reading it showed. Undefined for an unknown name not yet read (the generic icon).
   */
  fileTypeOf(path: string): FileType | undefined {
    const choice = viewChoice.typeFor(this.project, path);
    return registry.typeOf(path, { choice }) ?? registry.fileType(this.#known.get(path) ?? "");
  }

  /** An image of the project as a data URL (Go serves only its closed list of image types). */
  async readImage(path: string): Promise<string | undefined> {
    try {
      const response = await assetFetch(`/project-image?path=${encodeURIComponent(path)}`);
      return response.ok ? await response.text() : undefined;
    } catch {
      return undefined;
    }
  }

  /** The version Go reports for path: null once it is gone, undefined when it cannot tell. */
  async #versionOf(path: string): Promise<string | null | undefined> {
    try {
      const response = await assetFetch(`/project-text?path=${encodeURIComponent(path)}`, "HEAD");
      if (response.status === 404) return null;
      return response.ok ? (response.headers.get("ETag") ?? undefined) : undefined;
    } catch {
      return undefined;
    }
  }

  /**
   * Follows the open file: a new version is read again; a file deleted or renamed outside Inkwell
   * gives a toast (design §4.16) and the main document comes back.
   */
  async #checkOpened(ticket: number): Promise<void> {
    if (this.#checking) return;
    this.#checking = true;
    try {
      const path = this.current;
      const version = await this.#versionOf(path);
      if (ticket !== this.#openTicket) return;
      if (version === null) {
        clearInterval(this.#watch);
        notice.show(t("file.gone", { name: path }));
        await this.refreshFiles();
        // Another file may have been opened while the list refreshed.
        if (ticket !== this.#openTicket) return;
        await this.openFile(MAIN_FILE);
        return;
      }
      if (version === undefined || version === this.#version) return;
      const result = await this.#load(path);
      if (ticket !== this.#openTicket) return;
      this.#version = version;
      this.opened = result;
    } finally {
      this.#checking = false;
    }
  }
  /** The project's files, for the sidebar and the Insert menu. */
  files = $state.raw<ProjectFile[]>([]);
  /** How the current file is stored on disk; undefined until Go has written it. */
  fileInfo = $state.raw<FileInfo | undefined>();

  #rosterTicket = 0;
  #admitted = new AdmittedSet();
  /** The scope the sync indicator counts documents of: the admitted set, or a test's own. */
  #scope: AdmittedSet = this.#admitted;
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
  #offPageGone: (() => void) | undefined;

  #projectWaits = new Set<() => void>();
  #generation = 0;
  #offScope: (() => void) | undefined;

  /**
   * True from the confirmation of a project change until the page has left, or until its edits
   * could not be saved: the text is read-only meanwhile (PC1, rule N-15).
   */
  leaving = $state(false);
  closing = $state(false);
  authorizeProjectLeave: (() => Promise<boolean> | undefined) | undefined;
  #closeFocus: HTMLElement | undefined;
  #storeStarted = false;
  /**
   * The accepted invitation Go holds until the person confirms (PC1b, J-2): the joined document
   * and the folder it gets, as Go named them, and the folder of the text open now, "" when the
   * person has no text of their own (a first launch). Undefined when nothing is held.
   */
  joinOffer = $state<{ title: string; folder: string; current: string } | undefined>();
  /**
   * True once a held invitation was dropped, until the next « Rejoindre »: an invitation is used
   * once, so its text is of no use any more and the page asks for a new one (PC1b, J-2).
   */
  joinCancelled = $state(false);
  /**
   * Whether the document open before the join that made this folder can be returned to: on the
   * waiting page while the join waits (PC1b, J-2), in the main menu once it completed (J-9).
   */
  canReturn = $state(false);

  /**
   * Leaves this page for the one at `to` before a project change is asked of Go (PC1, rule N-15):
   * the text becomes read-only, the edits and the pending file write are saved (at most
   * {@link LEAVE_FLUSH_MS}), the repo is closed (the same time at most), then the page is replaced. This page asks Go for
   * nothing: the next one does. When the edits cannot be saved in time, nothing is sent, the text
   * is unlocked and the person is told; the answer is then false. Best effort: what makes sure no
   * write is lost or misapplied is Go's drain (rule N-16).
   */
  async leave(
    to: string,
    kind: LeaveKind = "takeOver",
    navigate: (url: string) => void = (url) => location.replace(url),
  ): Promise<boolean> {
    if (this.leaving || this.closing) return false;
    const nativeApproval = this.authorizeProjectLeave?.();
    if (nativeApproval && !(await nativeApproval)) return false;
    this.leaving = true;
    if (!nativeApproval && !(await this.flushBeforeRestart())) {
      this.leaving = false;
      notice.show(t("change.flushFailed"), "alert");
      return false;
    }
    await this.closeBeforeRestart();
    markLeave(kind); // the next page asks Go only after this confirmed leave (rule N-9)
    navigate(to);
    return true;
  }

  private async flushBeforeRestart(): Promise<boolean> {
    let timer: ReturnType<typeof setTimeout> | undefined;
    try {
      await Promise.race([
        Promise.all([this.#repo?.flush(), this.#autoProjectionFlush()]),
        new Promise<never>((_, reject) => {
          timer = setTimeout(() => reject(new Error("flush deadline")), LEAVE_FLUSH_MS);
        }),
      ]);
      return true;
    } catch {
      return false;
    } finally {
      clearTimeout(timer);
    }
  }

  /** Writes the file now unless saving waits for Ctrl+S, where a departure leaves it as it is. */
  #autoProjectionFlush(): Promise<void> | undefined {
    return settings.saveMode === "manual" ? undefined : this.#projection?.flush();
  }

  private async closeBeforeRestart(): Promise<void> {
    // The navigation goes ahead once the close is bounded. Go drains the bridge as well.
    let closing: ReturnType<typeof setTimeout> | undefined;
    await Promise.race([
      this.stop().catch(() => undefined),
      new Promise<void>((resolve) => {
        closing = setTimeout(resolve, LEAVE_FLUSH_MS);
      }),
    ]);
    clearTimeout(closing);
  }

  /**
   * Saves what was typed, now: the keys still waiting for the paint into their documents, then every
   * changed document to the store, without automerge-repo's save pause, so that a page that goes
   * right after a keystroke loses nothing (SAV-03b). Runs as the page goes (pagehide, or hidden).
   * Native close uses {@link saveForClose}, which propagates an unconfirmed write.
   */
  async saveNow(): Promise<void> {
    flushAllTyped();
    await this.#repo?.flush().catch(() => undefined);
  }

  /** A page that never started its store can confirm an empty editing cut. */
  async saveForClose(signal: AbortSignal): Promise<boolean> {
    const repo = this.#repo;
    const storage = this.#storage;
    this.closing = true;
    const focus = document.activeElement;
    this.#closeFocus = focus instanceof HTMLElement ? focus : undefined;
    this.#closeFocus?.blur(); // finish a native input-method composition
    document.body.inert = true;
    if (!this.#storeStarted) return false;
    if (!repo || !storage) throw new Error("workspace is not ready to save");
    await tick();
    signal.throwIfAborted();
    await flushCloseBuffers();
    signal.throwIfAborted();
    flushAllTyped(true);
    const ready = Object.values(repo.handles)
      .filter((handle) => handle.isReady())
      .map((handle) => handle.documentId);
    const pending = storage.pendingWrites();
    try {
      // In automatic saving the file is part of what a close keeps (WC1 W-2). Its write never
      // rejects: a refused one keeps its banner, and the next open writes the stored text again.
      await Promise.all([repo.flush(ready), pending, this.#autoProjectionFlush()]);
    } finally {
      signal.throwIfAborted();
    }
    return true;
  }

  async goodbyeForClose(attempt: number, signal: AbortSignal): Promise<void> {
    if (!this.#adapter) throw new Error("presence stream unavailable");
    pauseClosePresence();
    await this.#adapter.flushPresence(attempt, signal);
  }

  resumeAfterClose(): void {
    resumeClosePresence();
    this.closing = false;
    document.body.inert = false;
    this.#closeFocus?.focus();
    this.#closeFocus = undefined;
  }

  async stop(): Promise<void> {
    this.#offPageGone?.();
    this.#offPageGone = undefined;
    this.#generation++;
    this.#openTicket++;
    clearInterval(this.#watch);
    for (const cancel of this.#projectWaits) cancel();
    this.#offScope?.();
    this.#offScope = undefined;
    this.#offAdmission?.();
    this.#offAdmission = undefined;
    this.#offSize?.();
    this.#offSize = undefined;
    this.#loader = undefined;
    clearTimeout(this.#marksTimer);
    this.#projection?.stop();
    this.#sync?.stop();
    this.#projection = undefined;
    this.#sync = undefined;
    const repo = this.#repo;
    this.#repo = undefined;
    this.#adapter = undefined;
    const storage = this.#storage;
    this.#storage = undefined;
    this.handle = undefined;
    this.#display.reset();
    this.syncSlow = false;
    this.peerSync = {};
    this.waitingFor = undefined;
    this.canReturn = false;
    this.joinCancelled = false;
    try {
      await repo?.shutdown();
    } finally {
      storage?.close();
    }
  }

  async start(): Promise<void> {
    const generation = ++this.#generation;
    const id = await Session.LocalPeer();
    if (generation !== this.#generation) return;
    this.myId = id;
    void this.refreshStanding();
    if (!this.myName) this.setName(`Personne ${id.slice(0, 4)}`);

    const adapter = new WailsNetworkAdapter(() => Stream("net") as unknown as SocketLike, {
      admitted: this.#admitted.has,
      ready: (id) =>
        Object.hasOwn(this.#repo?.handles ?? {}, id) &&
        !!this.#repo?.handles[id as DocumentId]?.inState(["ready", "requesting", "unavailable"]),
      load: (id) => this.#loader?.ensure(id),
      onDrop: (reason) => console.warn("bridge: dropped frame:", reason),
    });
    adapter.onLinks((list) => {
      links.set(list);
      this.refreshSync();
    });
    links.syncCounts = (peer) => adapter.syncCounts(peer);
    adapter.onSealed(() => {
      this.sealed = true;
    });
    const storage = new GoStorageAdapter(() => Stream("store") as unknown as SocketLike);
    storage.onExternal((name) => {
      if (this.#sync) this.#sync.notify(name);
      else this.#early.push(name);
    });
    // Go took a project change while this page still wrote: the write is refused, never lost in
    // silence (PC1, rule N-16).
    storage.onLeaving(() => notice.show(t("change.flushFailed"), "alert"));
    this.#storage = storage;
    this.#storeStarted = true;
    const repo = new Repo({
      peerId: id as PeerId,
      network: [adapter],
      storage,
      isEphemeral: true,
      shareConfig: shareAdmitted(this.#admitted),
    });
    this.attach(adapter, repo);
    // The page may go at any time after a keystroke (a reload, a navigation): its writes start now,
    // before the page's timers die. The writes run in the event's microtasks, so they are sent.
    const gone = (e: Event) => {
      if (e.type === "pagehide" || document.visibilityState === "hidden") void this.saveNow();
    };
    this.#offPageGone?.();
    window.addEventListener("pagehide", gone);
    document.addEventListener("visibilitychange", gone);
    this.#offPageGone = () => {
      window.removeEventListener("pagehide", gone);
      document.removeEventListener("visibilitychange", gone);
    };

    this.#loader = new BootstrapLoader(this.#repo as Repo, this.#admitted, () => adapter.drain());
    let epoch = this.#admitted.epoch;
    this.#offScope = this.#admitted.onChange(() => {
      if (epoch !== this.#admitted.epoch) {
        adapter.scopeChanged();
        epoch = this.#admitted.epoch;
      }
      this.#repo?.shareConfigChanged();
      for (const id of this.#admitted.ids) this.#loader?.ensure(id);
    });

    observeSaveNow(() => this.saveNow());
    observeVariantHeads(
      () => this.handle,
      () => this.#repo,
    );
    observeSession(
      () => Session.StartupID(),
      () => ({ signers: adapter.attestedSigners, peers: [...this.peers] }),
    );
    await this.openStart(generation);
  }

  /**
   * Opens the document this start shows: the one of the join the folder waits for (PC1b), else
   * the recorded one, else a fresh one. While a join waits, no document is ever created: the
   * joined list would get a root that is not the inviter's (rule J-2).
   */
  async openStart(generation = this.#generation): Promise<void> {
    const repo = this.#repo;
    if (!repo) throw new Error("workspace not started");
    const pending = await Session.PendingJoin();
    if (generation !== this.#generation) return;
    // The way back to the document open before the join, waiting or not (PC1b, J-2 and J-9).
    this.canReturn = pending.canReturn ?? false;
    if (pending.waiting) return this.#resumeJoin(pending, generation);
    const saved = (await Projects.Current()).RootURL;
    if (generation !== this.#generation) return;
    if (saved && isValidAutomergeUrl(saved)) {
      try {
        await this.open(saved);
        return;
      } catch {
        // The saved document is gone from the project folder: start a fresh one.
      }
    }
    await this.open(repo.create<Doc>({ text: "" }).url);
  }

  /**
   * Waits for the document of the join this folder was made for, for as long as needed, then
   * opens it (PC1b, rules J-1 and J-2). The root is the one the invitation stated, else the one
   * the inviter announces, and nobody else's: nothing signed states the root yet, and the first
   * one recorded binds the project for good, so another member of the joined list is not
   * listened to (Go refuses it too). Go is asked to consume the record
   * (`Session.CompleteJoin`) only once the repo holds the document and its flush returned: Go
   * records the root then, and before that no root is recorded at all. Whatever fails (the
   * inviter is away, Go does not hold the document yet) is tried again after
   * {@link JOIN_RETRY_MS}; meanwhile {@link waitingFor} names who is waited for.
   */
  async #resumeJoin(
    pending: { root: string; inviter: string; name: string; folder?: string; canReturn?: boolean },
    generation: number,
  ): Promise<void> {
    const inviter = pending.inviter as PeerId;
    // Never the neutral « Personne 12D3 »: every device id starts alike, it would name nobody.
    this.waitingFor = pending.name || this.knownName(inviter) || "";
    // The root, once known: kept from one try to the next.
    let root = isValidAutomergeUrl(pending.root) ? pending.root : undefined;
    for (;;) {
      const repo = this.#repo;
      if (!repo || generation !== this.#generation) return;
      try {
        root ??= await this.#announced(inviter);
        this.#admitted.setRoot(root);
        const handle = await repo.find<Doc>(root);
        await repo.flush([handle.documentId]);
        await Session.CompleteJoin(root, inviter);
        break;
      } catch {
        if (generation !== this.#generation) return;
        await new Promise((resolve) => setTimeout(resolve, JOIN_RETRY_MS));
      }
    }
    // The record is consumed, once: what follows is an ordinary start on the recorded root.
    if (!root || generation !== this.#generation) return;
    // The folder is named when it is not the one the person read before the restart.
    notice.queue(
      pending.folder ? t("join.doneFolder", { folder: pending.folder }) : t("join.done"),
    );
    try {
      await this.open(root);
    } finally {
      this.waitingFor = undefined;
    }
  }

  async open(url: AutomergeUrl): Promise<void> {
    if (!this.#repo) throw new Error("workspace not started");
    this.#offAdmission?.();
    this.#admitted.setRoot(url);
    const epoch = this.#admitted.epoch;
    const generation = this.#generation;
    const handle = await this.#repo.find<Doc>(url);
    if (generation !== this.#generation || epoch !== this.#admitted.epoch) return;
    this.#openTicket++;
    clearInterval(this.#watch);
    this.current = MAIN_FILE;
    this.opened = undefined;
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
    if (generation !== this.#generation) return;
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
    if (this.locked) return; // a user action that writes: nothing while the rights are unknown (N-3)
    this.#sync?.undo();
    clearTimeout(this.#marksTimer);
    this.marks = null;
  }

  /** Ctrl+Z: takes the outside edit back if it is the last thing that happened to the text. */
  undoWithKey(): boolean {
    if (this.keyUndoHooks.some((hook) => hook())) return true;
    if (!this.#sync?.canKeyUndo || this.locked) return false;
    this.undoExternal();
    return true;
  }

  /** "Ask" answered with "Integrate". */
  integrateExternal(): Promise<void> {
    if (this.locked) return Promise.resolve(); // writes the outside text in: not while locked (N-3)
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
        : [{ path: MAIN_FILE, size: 0, main: true }, ...files];
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
    let info: FileInfo | undefined;
    try {
      info = await this.#storage.fileInfo(this.current);
    } catch {
      info = undefined;
    }
    // Another file may have been opened meanwhile: its (absent) info stands.
    if (this.current === MAIN_FILE) this.fileInfo = info;
  }

  /** Writes the document to the project folder now, if a write is still waiting for a pause in typing. */
  async flushProjection(): Promise<void> {
    await this.#projection?.flush();
  }

  /** The text of a project file as UTF-8 with LF; undefined if absent, too large or not text. */
  async readFile(path: string): Promise<string | undefined> {
    const read = await this.#readText(path);
    return typeof read === "object" ? read.text : undefined;
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
   * Why this device cannot edit: "notNamed" or "removed" (the member list does not name it, W58),
   * "reader" (its role), or "suspended" (an admin disconnected it); undefined when it can, or while
   * Go has not yet said.
   */
  get access(): "reader" | "suspended" | "notNamed" | "removed" | undefined {
    void this.rolesVersion;
    const me = this.#adapter?.roleOf(this.myId);
    return accessOf(me?.role, me?.suspended ?? false, this.standing);
  }

  /**
   * Whether the rights of this device are known: Go has said its standing and, for a member, has
   * reported its role (a device the list does not name gets no role, and none is waited for).
   */
  get rightsKnown(): boolean {
    void this.rolesVersion;
    if (this.standing === undefined) return false;
    if (this.standing !== "member") return true;
    return this.#adapter?.roleOf(this.myId) !== undefined;
  }

  /**
   * Whether the text cannot be changed now (W58a, N-3): until the rights are known, for every
   * project, so nothing typed before Go answers is saved or sent; then for whoever has an
   * {@link access} reason, and while the page leaves for a project change (PC1, N-15). A locked
   * editor still shows, selects and copies its text.
   */
  get locked(): boolean {
    return this.closing || this.leaving || !this.rightsKnown || this.access !== undefined;
  }

  /**
   * What the invitation controls show: "loading" until Go has reported this device's role, then
   * "admin", "nonAdmin", or "notMember" (the list does not name this device, and no role will
   * come).
   */
  get inviteState(): "loading" | "admin" | "nonAdmin" | "notMember" {
    void this.rolesVersion;
    if (this.standing === "notNamed" || this.standing === "removed") return "notMember";
    const me = this.#adapter?.roleOf(this.myId);
    if (me === undefined) return "loading";
    return me.role === "admin" && !me.suspended ? "admin" : "nonAdmin";
  }

  /** Whether this device may make an invitation: only once Go has reported an admin who is not disconnected. */
  get mayInvite(): boolean {
    return this.inviteState === "admin";
  }

  /** Asks Go whether the member list names this device; the latest answer wins. */
  async refreshStanding(): Promise<void> {
    const ticket = ++this.#standingTicket;
    try {
      const [standing, canTakeOver] = await Promise.all([
        Session.Standing() as Promise<Standing>,
        Session.CanTakeOver(),
      ]);
      if (ticket !== this.#standingTicket) return;
      this.standing = standing;
      this.canTakeOver = canTakeOver;
      if (standing === "member") this.listPending = false;
    } catch {
      // Go does not answer (no project yet): the standing stays as it was.
    }
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
  hearNames(people: readonly { peerId: string; name: string; file?: string }[]): void {
    this.names = Object.fromEntries(people.map((p) => [p.peerId, p.name]));
    this.peerFiles = Object.fromEntries(people.map((p) => [p.peerId, p.file ?? "main.typ"]));
    for (const p of people) if (p.name) this.#heard[p.peerId] = p.name;
  }

  setName(name: string): void {
    this.myName = name.trim().slice(0, 40);
    safeSet(NAME_KEY, this.myName);
  }

  /**
   * Wires the sync indicator to the network adapter and the repo: who is connected, what they
   * report, and (through `refreshSync`) the heads of my own documents. `start` calls it; tests
   * call it with their own pair and scope.
   */
  attach(adapter: WailsNetworkAdapter, repo: Repo, admitted: AdmittedSet = this.#admitted): void {
    this.#adapter = adapter;
    this.#repo = repo;
    this.#scope = admitted;
    adapter.onRoles(() => {
      this.rolesVersion++;
      void this.refreshRoster();
      void this.refreshStanding();
      void this.refreshInvitation();
    });
    adapter.on("peer-candidate", ({ peerId }) => {
      this.peerUp(peerId);
      this.refreshSync();
    });
    adapter.on("peer-disconnected", ({ peerId }) => {
      this.peers = this.peers.filter((p) => p !== peerId);
      this.refreshSync();
    });
    adapter.onSync(() => this.refreshSync());
  }

  /**
   * Derives the sync state from what this device observes (W33): the heads heard from each
   * connected peer, my documents, the links. Called whenever one of them changes.
   */
  refreshSync(): void {
    const adapter = this.#adapter;
    const repo = this.#repo;
    // This version of automerge-repo emits no event for a new handle: hook the ones met here, and
    // follow their heads (a local edit must leave « Synchronisé » at once).
    for (const handle of Object.values(repo?.handles ?? {})) {
      if (this.#hooked.has(handle)) continue;
      this.#hooked.add(handle);
      handle.on("heads-changed", () => this.refreshSync());
      // A handle turns ready after the heads-changed that delivered its content: no event follows.
      void handle
        .whenReady()
        .then(() => this.refreshSync())
        .catch(() => {}); // deleted or timed out: nothing to show for it
    }
    const observed =
      adapter && repo
        ? observePeers(
            adapter,
            (id) => {
              const handle = Object.hasOwn(repo.handles, id)
                ? repo.handles[id as DocumentId]
                : undefined;
              return handle?.isReady() ? handle.doc() : undefined;
            },
            (peer) => links.of(peer),
            this.#scope.has,
          )
        : new Map();
    const states = new Map([...observed].map(([peer, o]) => [peer, peerSyncState(o)] as const));
    const list = [...states.values()];
    this.peerSync = Object.fromEntries(states);
    this.syncSlow = isSlow([...observed.values()]);
    this.#display.update(globalSyncState(list), sendingCount(observed.values()));
  }

  /** Lists a connected person; the invitation may be what let them in, so Go is asked about it. */
  peerUp(peerId: string): void {
    if (!this.peers.includes(peerId)) this.peers = [...this.peers, peerId];
    void this.refreshInvitation();
  }

  /**
   * Asks Go what became of the invitation shown (PC1b, rule J-8): « Partager » calls it when it
   * opens, and so do a change of the member list (the person invited is admitted) and a new
   * connection. The answer about another invitation than the one now shown is ignored.
   */
  async refreshInvitation(): Promise<void> {
    const text = this.invitation;
    if (!text) return;
    try {
      const state = (await Session.InvitationState(text)) as InvitationState;
      if (text === this.invitation) this.invitationState = state;
    } catch {
      // Go does not answer (no project, or leaving): the state stays as it was.
    }
  }

  /** Makes a fresh invitation, replacing the last one (used or not). */
  async host(): Promise<void> {
    this.error = "";
    try {
      const made = await Session.Invite("writer");
      if (made.refused) {
        // A refusal is a result with a stable key: the sentence is ours, never Go's text (N-2).
        this.error = t(
          made.refused === "notMember" ? "share.host.notMember" : "share.host.adminOnly",
        );
        return;
      }
      this.invitation = made.text;
      this.invitationState = "open";
    } catch {
      this.error = t("error.host");
    }
  }

  /**
   * Uses the invitation text. Into another document than the open one, Go only holds the answer
   * (PC1b, rules J-1 and J-2): the page asks the person ({@link joinOffer}), and nothing restarts
   * before {@link confirmJoin}. Into the open document (a copy of it on a device the list did
   * not name, rule J-5) nothing restarts: the page follows the inviting person's document.
   */
  async join(invitation: string): Promise<void> {
    this.error = "";
    this.joinCancelled = false;
    this.joining = true;
    try {
      const joined = await Session.Join(invitation.trim());
      if (joined.restart) {
        this.joinOffer = {
          title: joined.title ?? "",
          folder: joined.folder ?? "",
          current: this.handle?.doc()?.text.trim() ? (joined.current ?? "") : "",
        };
        return;
      }
      const peer = joined.peer as PeerId;
      await this.refreshStanding(); // a join turns "notNamed" into "member" (N-1)
      this.listPending = this.standing !== "member";
      await this.open(await this.#announced(peer, 10_000));
    } catch {
      this.listPending = false;
      this.error = t("error.join");
    } finally {
      this.joining = false;
    }
  }

  /**
   * « Redémarrer et ouvrir »: the page leaves as for any project change ({@link leave}) and the
   * restart page asks Go to finish the held join; when the edits cannot be saved in time the
   * held invitation is dropped and nothing changes (rule N-15).
   */
  async confirmJoin(): Promise<void> {
    if (!this.joinOffer) return;
    if (!(await this.leave("restart.html", "join"))) await this.cancelJoin();
  }

  /**
   * « Annuler »: Go drops the invitation it holds; nothing was written, nothing changes. The
   * invitation is used all the same: the person who invites admitted this device.
   */
  async cancelJoin(): Promise<void> {
    this.joinCancelled = true;
    this.joinOffer = undefined;
    await Session.CancelChange().catch(() => undefined);
  }

  /**
   * « Revenir à mon document », while a join waits (PC1b, J-2) or after it (J-9): the page leaves,
   * and the restart page asks Go to restart Inkwell on the document that was open before. A join
   * that waits keeps waiting in its own folder.
   */
  async returnToPrevious(): Promise<void> {
    await this.leave("restart.html", "return");
  }

  /**
   * Resolves with the project `peer` announced, and no other peer's, waiting for a handshake if
   * needed: for `timeoutMs` when given, else until the page stops.
   */
  #announced(peer: PeerId, timeoutMs?: number): Promise<AutomergeUrl> {
    const adapter = this.#adapter;
    if (!adapter) return Promise.reject(new Error("workspace not started"));
    const known = adapter.projectOf(peer);
    if (known) return Promise.resolve(known);
    return new Promise((resolve, reject) => {
      const cleanup = () => {
        clearTimeout(timer);
        off();
        this.#projectWaits.delete(cancel);
      };
      const cancel = () => {
        cleanup();
        reject(new Error("workspace stopped"));
      };
      const timer =
        timeoutMs === undefined
          ? undefined
          : setTimeout(() => {
              cleanup();
              reject(new Error("no project announced"));
            }, timeoutMs);
      const off = adapter.onProject((url, from) => {
        if (from !== peer) return;
        cleanup();
        resolve(url);
      });
      this.#projectWaits.add(cancel);
    });
  }
}

export const workspace = new Workspace();
