import { next as A } from "@automerge/automerge";
import type { DocHandle } from "@automerge/automerge-repo";
import { Session as Backend } from "../../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import {
  type Analysis,
  type Analyzer,
  inlineAnalyzer,
  type WorkerLike,
  workerAnalyzer,
} from "../../history/analysis";
import { type Filters, filterSessions, NO_FILTER } from "../../history/filters";
import { buildGraph, type Row } from "../../history/graph";
import { sinceHeads } from "../../history/gutter";
import type { Session } from "../../history/log";
import { textAt } from "../../history/log";
import { watchHeadings } from "../../history/normalize";
import { type Author, authorBook, claimActor, type Directory } from "../../history/people";
import type { HDoc, Heads, Variant, Version } from "../../history/types";
import {
  abandonVariant,
  fork,
  mergeVariant,
  nameVersion,
  previewUndo,
  type Reverted,
  reopenVariant,
  revertTo,
  undoChange,
  undoRevert,
  whatChanged,
} from "../../history/versions";
import { t } from "../../i18n";
import { tooBigToEdit } from "../../store/limits";
import { settings } from "../../store/settings.svelte";
import { lastVisit, saveVisit } from "../../store/visit";
import { workspace } from "../../workspace/workspace.svelte";

export type Tab = "history" | "comments" | "activity";
export type ColorBy = "type" | "person";

/** What the differences window shows: two texts, and the history that explains them. */
export interface DiffRequest {
  title: string;
  /** The older text and the newer one. */
  before: string;
  after: string;
  /** The entry being looked at, when it is one entry (its author is every stretch's author). */
  session?: Session;
  /** Who wrote the stretches: the document's own history (compared with now), or nobody. */
  attributed: boolean;
  /** The moment the text is compared with, for « Revenir » and « Annuler ». */
  revert?: { heads: Heads; label: string };
  undo?: { session: Session };
}

/** A change waiting for a yes: shown with its result first (the preview). */
export interface ApplyRequest {
  kind: "revert" | "undo";
  title: string;
  /** The text now and the text it would become. */
  before: string;
  after: string;
  confirm: () => void;
}

/** The rows a person can pick, with whatever they carry. */
export type RowKey = string;

/** Height of one row of the history list (the list is virtual). */
export const ROW_HEIGHT = 64;

/** How long a person pauses after an edit before the device says in the document who it is. */
const CLAIM_PAUSE_MS = 2_000;

const HIDDEN = new Set(["version", "variant", "merge", "claim"]);

class HistoryState {
  open = $state(false);
  tab = $state<Tab>("history");
  colorBy = $state<ColorBy>("type");
  showWho = $state(false);
  showAbandoned = $state(false);
  filters = $state<Filters>({ ...NO_FILTER });
  selected = $state<RowKey | undefined>();
  analysis = $state.raw<Analysis | undefined>();
  /** The document the analysis was made from, and its heads. */
  doc = $state.raw<A.Doc<HDoc> | undefined>();
  analysisHeads = $state.raw<Heads>([]);
  busy = $state(false);
  /** The point changes are counted from, and when the person left. */
  since = $state.raw<Heads>([]);
  sinceAt = $state<number | undefined>();
  members = $state.raw<{ name: string; role: string; self: boolean; peers: string[] }[]>([]);
  diff = $state.raw<DiffRequest | undefined>();
  apply = $state.raw<ApplyRequest | undefined>();
  /** Set while a person is naming a version or a variant. */
  naming = $state<{ kind: "version" | "variant"; heads: Heads } | undefined>();
  error = $state("");
  /** The variant being edited in the editor, by name; empty for the main document. */
  variantName = $state("");

  #handle: DocHandle<HDoc> | undefined;
  #analyzer: Analyzer | undefined;
  #seq = 0;
  #timer: ReturnType<typeof setTimeout> | undefined;
  #claimTimer: ReturnType<typeof setTimeout> | undefined;
  #stop: (() => void)[] = [];
  #lastRevert: Reverted | undefined;

  get handle(): DocHandle<HDoc> | undefined {
    return this.#handle;
  }

  #previewWasOpen = false;

  /**
   * Opens or closes the drawer. It takes the place of the preview (both live on the right): the
   * preview comes back when the drawer closes, so the text keeps its room.
   */
  toggle(): void {
    const project = workspace.project;
    this.open = !this.open;
    if (this.open) {
      this.#previewWasOpen = settings.previewVisible(project);
      if (this.#previewWasOpen) settings.setPreviewVisible(project, false);
    } else if (this.#previewWasOpen) {
      settings.setPreviewVisible(project, true);
      this.#previewWasOpen = false;
    }
  }

  /** The signed list says this device's person only reads. */
  readOnly = $derived(this.members.some((m) => m.self && m.role === "reader"));

  /** The actor of this device in the open document. */
  myActor = $state("");

  directory: Directory = $derived({
    myPeer: workspace.myId,
    myActor: this.myActor,
    nameOfPeer: (peer) => this.members.find((m) => m.peers.includes(peer))?.name || undefined,
    present: workspace.names,
  });

  author = $derived(authorBook(this.doc, this.directory));

  /** Starts following a document: its history, the marks since the last visit, the headings rule. */
  attach(handle: DocHandle<HDoc>): void {
    this.detach();
    this.#handle = handle;
    const doc = handle.doc();
    if (!doc) return;
    this.doc = doc;
    this.myActor = A.getActorId(doc);
    // First opening: the last visit is now, nothing is marked (HIS-01).
    const visit = lastVisit(handle.url);
    this.since = sinceHeads(doc, visit);
    this.sinceAt = visit && this.since === visit.heads ? visit.at : Date.now();
    if (!visit) saveVisit(handle.url, this.since);

    this.#analyzer = makeAnalyzer();
    // The device says who it is once its person pauses after an edit, never while they type:
    // opening a document writes nothing, and the claim never lands in the middle of typing.
    const onChange = ({ patchInfo }: { patchInfo: { source: string } }) => {
      if (patchInfo.source === "change") {
        clearTimeout(this.#claimTimer);
        this.#claimTimer = setTimeout(() => this.#claim(true), CLAIM_PAUSE_MS);
      }
      this.#schedule();
    };
    handle.on("change", onChange);
    this.#stop.push(() => handle.off("change", onChange), watchHeadings(handle));
    const leave = (e: Event) => {
      if (e.type === "pagehide" || document.visibilityState === "hidden") this.#remember();
    };
    window.addEventListener("pagehide", leave);
    document.addEventListener("visibilitychange", leave);
    this.#stop.push(() => {
      window.removeEventListener("pagehide", leave);
      document.removeEventListener("visibilitychange", leave);
    });
    const hook = () => this.undoKey();
    workspace.keyUndoHooks.push(hook);
    this.#stop.push(() => {
      workspace.keyUndoHooks = workspace.keyUndoHooks.filter((h) => h !== hook);
    });
    void this.loadMembers();
    void this.refresh();
  }

  detach(): void {
    this.#remember();
    for (const stop of this.#stop.splice(0)) stop();
    clearTimeout(this.#timer);
    clearTimeout(this.#claimTimer);
    this.#analyzer?.dispose();
    this.#analyzer = undefined;
    this.#handle = undefined;
    this.analysis = undefined;
    this.doc = undefined;
    this.#seq++;
  }

  /**
   * Says in the document who this device is (once per actor and name); readers write nothing.
   * Without `create`, only a claim already made is brought up to date (a new name).
   */
  #claim(create = false): void {
    const handle = this.#handle;
    // A document too big to edit is only read: nothing is written into it (ADR 0026).
    if (!handle || this.readOnly || !workspace.myId || tooBigToEdit(handle.doc()?.text.length ?? 0))
      return;
    claimActor(handle, { id: workspace.myId, name: workspace.myName }, create);
  }

  /** The name changed: the document says it. */
  renamed(): void {
    this.#claim();
  }

  /** Writes down where this device left the document: the next visit counts changes from here. */
  #remember(): void {
    const handle = this.#handle;
    const doc = handle?.doc();
    if (handle && doc) saveVisit(handle.url, A.getHeads(doc));
  }

  async loadMembers(): Promise<void> {
    try {
      const people = (await Backend.Members()) ?? [];
      this.members = people.map((p) => ({
        name: p.name,
        role: p.role,
        self: p.self,
        peers: (p.devices ?? []).map((d) => d.peer),
      }));
    } catch {
      this.members = [];
    }
  }

  /** The person has seen what changed: counting starts again from now. */
  markSeen(): void {
    const handle = this.#handle;
    const doc = handle?.doc();
    if (!handle || !doc) return;
    this.since = A.getHeads(doc);
    this.sinceAt = Date.now();
    saveVisit(handle.url, this.since);
    void this.refresh();
  }

  #schedule(): void {
    clearTimeout(this.#timer);
    this.#timer = setTimeout(() => void this.refresh(), 300);
  }

  async refresh(): Promise<void> {
    const handle = this.#handle;
    const analyzer = this.#analyzer;
    const doc = handle?.doc();
    if (!handle || !analyzer || !doc) return;
    const seq = ++this.#seq;
    this.busy = true;
    try {
      const result = await analyzer.analyze(doc, this.since);
      if (seq !== this.#seq) return;
      this.analysis = result;
      this.doc = doc;
      this.analysisHeads = A.getHeads(doc);
      this.error = "";
    } catch (err) {
      if (seq !== this.#seq) return;
      // The worker failed: do the work here, in slices, rather than leave the screen empty.
      if (this.#analyzer === analyzer) {
        analyzer.dispose();
        this.#analyzer = inlineAnalyzer();
        this.#schedule();
      }
      this.error = err instanceof Error ? err.message : "";
    } finally {
      if (seq === this.#seq) this.busy = false;
    }
  }

  // The list ---------------------------------------------------------------------------------

  /** The sessions that are entries of their own (not the bookkeeping of versions and variants). */
  get entries(): Session[] {
    return (this.analysis?.sessions ?? []).filter(
      (s) => !s.claimOnly && !(s.action && HIDDEN.has(s.action.kind)),
    );
  }

  filtered = $derived.by(() => {
    const doc = this.doc;
    const a = this.analysis;
    if (!doc || !a) return { shown: [] as Session[], total: 0 };
    const entries = a.sessions.filter(
      (s) => !s.claimOnly && !(s.action && HIDDEN.has(s.action.kind)),
    );
    return filterSessions(entries, this.filters, {
      author: this.author,
      now: Date.now(),
      doc,
      fileOf: () => workspace.current,
    });
  });

  get filtering(): boolean {
    const f = this.filters;
    return (
      f.person !== undefined ||
      f.period !== "all" ||
      f.section !== undefined ||
      f.file !== undefined
    );
  }

  rows = $derived.by((): Row[] => {
    const a = this.analysis;
    const doc = this.doc;
    if (!a || !doc) return [];
    const narrowed = this.filtering;
    return buildGraph({
      order: a.order,
      sessions: this.filtered.shown,
      versions: narrowed ? [] : (doc.meta?.versions ?? []).filter((v) => !v.auto),
      variants: narrowed ? [] : (doc.meta?.variants ?? []),
      showAbandoned: this.showAbandoned,
    });
  });

  /** The people who wrote something, for the person filter. */
  get people(): string[] {
    const names = new Set<string>();
    for (const s of this.entries) names.add(this.author(s.actor).name);
    return [...names].sort((x, y) => x.localeCompare(y));
  }

  /** The sections of the current text, for the section filter. */
  sections = $derived.by((): string[] => {
    const text = this.doc?.text ?? "";
    const out: string[] = [];
    for (const m of text.matchAll(/^(?:={1,6}|#{1,6})[ \t]+(.+?)\s*$/gm)) {
      const title = (m[1] as string).replace(/\s*<[^<>\s]+>\s*$/, "").trim();
      if (title && !out.includes(title)) out.push(title);
    }
    return out;
  });

  rowOf(key: RowKey | undefined): Row | undefined {
    return key ? this.rows.find((r) => r.key === key) : undefined;
  }

  /** Opens the drawer on one entry (a click on a coloured passage of the text). */
  show(sessionId: string): void {
    if (!this.open) this.toggle();
    this.tab = "history";
    this.filters = { ...NO_FILTER };
    this.selected = `s:${sessionId}`;
  }

  // What a row can do -------------------------------------------------------------------------

  textAtHeads(heads: Heads): string {
    const doc = this.doc;
    return doc ? textAt(doc, heads) : "";
  }

  /** « Voir les changements » of one entry. */
  viewEntry(session: Session): void {
    this.diff = {
      title: this.entryTitle(session),
      before: this.textAtHeads(session.before),
      after: this.textAtHeads(session.after),
      session,
      attributed: false,
      revert: { heads: session.after, label: this.entryTitle(session) },
      undo: { session },
    };
  }

  /** « Comparer avec maintenant » for the point reached at `heads`. */
  compareWithNow(heads: Heads, label: string): void {
    this.diff = {
      title: t("history.compare.title", { name: label }),
      before: this.textAtHeads(heads),
      after: this.doc?.text ?? "",
      attributed: true,
      revert: { heads, label },
    };
  }

  /** The main document and a variant, side by side. */
  async compareVariant(variant: Variant): Promise<void> {
    const repo = workspace.repo;
    if (!repo || !this.doc) return;
    const handle = await repo.find<HDoc>(variant.url as never);
    this.diff = {
      title: t("history.compare.variant", { name: variant.name }),
      before: this.doc.text,
      after: handle.doc().text,
      attributed: false,
    };
  }

  entryTitle(s: Session): string {
    const a = this.author(s.actor);
    if (s.action) return this.actionTitle(s);
    return a.mine ? t("history.you") : a.name;
  }

  actionTitle(s: Session): string {
    const label = s.action?.label ?? "";
    switch (s.action?.kind) {
      case "revert":
        return t("history.row.revert", { name: label });
      case "undo":
        return t("history.row.undo");
      case "fix":
        return label;
      default:
        return label;
    }
  }

  // Naming, going back, variants --------------------------------------------------------------

  startNaming(kind: "version" | "variant", heads: Heads): void {
    this.naming = { kind, heads };
  }

  confirmNaming(name: string): void {
    const handle = this.#handle;
    const naming = this.naming;
    const label = name.trim();
    this.naming = undefined;
    if (!handle || !naming || !label) return;
    if (naming.kind === "version") nameVersion(handle, label, naming.heads);
    else if (workspace.repo) {
      const { variant } = fork(workspace.repo, handle, label, naming.heads);
      this.selected = `b:${variant.url}`;
    }
    void this.refresh();
  }

  /** « Revenir à ce moment »: shows what the text would become, applies on a yes. */
  askRevert(heads: Heads, label: string): void {
    const handle = this.#handle;
    if (!handle || this.readOnly) return;
    const after = textAt(handle.doc(), heads);
    this.apply = {
      kind: "revert",
      title: t("history.revert.title", { name: label }),
      before: handle.doc().text,
      after,
      confirm: () => {
        this.#lastRevert = revertTo(handle, heads, label) ?? this.#lastRevert;
        this.apply = undefined;
        this.diff = undefined;
        void this.refresh();
      },
    };
  }

  /** « Annuler cette modification »: the targeted inverse, previewed. */
  askUndo(session: Session): void {
    const handle = this.#handle;
    if (!handle || this.readOnly) return;
    const done = whatChanged(handle, session.before, session.after);
    if (!done) return;
    const label = this.entryTitle(session);
    this.apply = {
      kind: "undo",
      title: t("history.undo.title", { name: label }),
      before: handle.doc().text,
      after: previewUndo(handle, done),
      confirm: () => {
        undoChange(handle, done, label);
        this.apply = undefined;
        this.diff = undefined;
        void this.refresh();
      },
    };
  }

  /** Ctrl+Z takes back a « Revenir » when it was the last thing that happened. */
  undoKey(): boolean {
    const handle = this.#handle;
    const done = this.#lastRevert;
    if (!handle || !done) return false;
    const ok = undoRevert(handle, done);
    if (ok) {
      this.#lastRevert = undefined;
      void this.refresh();
    }
    return ok;
  }

  merge(variant: Variant): void {
    const handle = this.#handle;
    const repo = workspace.repo;
    if (!handle || !repo || this.readOnly) return;
    void repo.find<HDoc>(variant.url as never).then((v) => {
      mergeVariant(handle, v, variant);
      void this.refresh();
    });
  }

  abandon(variant: Variant): void {
    const handle = this.#handle;
    if (!handle || this.readOnly) return;
    abandonVariant(handle, variant.url);
    void this.refresh();
  }

  reopen(variant: Variant): void {
    const handle = this.#handle;
    if (!handle || this.readOnly) return;
    reopenVariant(handle, variant.url);
    void this.refresh();
  }

  /** Edits a variant in the editor instead of the main document. */
  async editVariant(variant: Variant): Promise<void> {
    const repo = workspace.repo;
    if (!repo) return;
    const handle = await repo.find<HDoc>(variant.url as never);
    claimActor(handle, { id: workspace.myId, name: workspace.myName });
    workspace.variant = handle as never;
    this.variantName = variant.name;
  }

  backToMain(): void {
    workspace.variant = undefined;
    this.variantName = "";
  }

  /** Whether the doc is the one on screen (for the overlays that use positions). */
  isCurrent(): boolean {
    const doc = this.#handle?.doc();
    return !!doc && A.getHeads(doc).join() === this.analysisHeads.join();
  }

  /** The colour slot of a person by name, for the legend of « Qui a écrit quoi ». */
  legendIndex(name: string): number {
    const s = this.entries.find((e) => this.author(e.actor).name === name);
    return s ? this.author(s.actor).index : 1;
  }

  authorOf(session: Session): Author {
    return this.author(session.actor);
  }

  versionBy(v: Version): Author {
    return this.author(v.by);
  }

  /** Used by the specs: heads fixed by hand, no worker. */
  useInline(): void {
    this.#analyzer?.dispose();
    this.#analyzer = inlineAnalyzer();
  }
}

function makeAnalyzer(): Analyzer {
  if (typeof Worker === "undefined") return inlineAnalyzer();
  return workerAnalyzer(
    () =>
      new Worker(new URL("../../history/analysis.worker.ts", import.meta.url), {
        type: "module",
      }) as unknown as WorkerLike,
  );
}

export const history = new HistoryState();
