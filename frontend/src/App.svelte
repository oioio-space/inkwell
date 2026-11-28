<script lang="ts">
import { next as A } from "@automerge/automerge";
import { Presence } from "@automerge/automerge-repo";
import { Flags, Stream, WailsSocket } from "@wailsio/runtime";
import { onMount, tick, untrack } from "svelte";
import { Session } from "../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { flushAllTyped, whenWritten } from "./editor/amSync";
import { markExternal } from "./editor/external";
import {
  formatsOf,
  type InsertItem,
  insertsOf,
  noteInsertHost,
  openInsertStep,
} from "./editor/format";
import type { CursorState } from "./editor/presence";
import { activateStartup, registry, warmCommands } from "./extensions";
import {
  configure,
  installKeys,
  locationShows,
  menuOf,
  panelsOf,
  statusOf,
} from "./extensions/commands";
import { setLive } from "./extensions/live.svelte";
import ViewerHost from "./extensions/ViewerHost.svelte";
import { t } from "./i18n";
import { LANGUAGE_TYPE } from "./language/client";
import { problemMarks } from "./language/marks";
import { resolveRange, toOffset, toPosition } from "./language/positions";
import { problems } from "./language/problems.svelte";
import { configureLanguage, language } from "./language/service";
import { validFilePath } from "./net/admission";
import { links } from "./net/links.svelte";
import type { SocketLike } from "./net/WailsNetworkAdapter";
import { profileNativeStage } from "./perf/nativeDispatch";
import { toggleFullscreen } from "./present/fullscreen";
import PresentBanner from "./present/PresentBanner.svelte";
import PresentHost from "./present/PresentHost.svelte";
import { presentSession } from "./present/session.svelte";
import { exportPdf, jumpFromCursor, jumpFromPage, jumpToProblem } from "./preview/actions";
import { lastChangeSeconds } from "./preview/docTime";
import Preview from "./preview/Preview.svelte";
import { preview } from "./preview/preview.svelte";
import { SaveError } from "./store/GoStorageAdapter";
import { settings } from "./store/settings.svelte";
import { zoom } from "./store/zoom.svelte";
import AboutDialog, { type BuildInfo } from "./ui/AboutDialog.svelte";
import Banner from "./ui/Banner.svelte";
import CommandDialogs from "./ui/command/CommandDialogs.svelte";
import { dialogShell } from "./ui/command/dialogs.svelte";
import FormatBar from "./ui/command/FormatBar.svelte";
import PanelSide from "./ui/command/PanelSide.svelte";
import { configurePanels } from "./ui/command/panelSelection.svelte";
import { prefetchSide } from "./ui/command/panels.svelte";
import ViewBar from "./ui/command/ViewBar.svelte";
import ExternalBanner from "./ui/ExternalBanner.svelte";
import ExternalChangeToast from "./ui/ExternalChangeToast.svelte";
import { session } from "./ui/editorSession.svelte";
import HistoryBars, { keepCaret } from "./ui/history/HistoryBars.svelte";
import HistoryDialogs from "./ui/history/HistoryDialogs.svelte";
import { history } from "./ui/history/history.svelte";
import JoinWaiting from "./ui/JoinWaiting.svelte";
import NoticeToast from "./ui/NoticeToast.svelte";
import { notice } from "./ui/notice.svelte";
import ProjectionBanner from "./ui/ProjectionBanner.svelte";
import AccessBanner from "./ui/people/AccessBanner.svelte";
import PeopleDialog from "./ui/people/PeopleDialog.svelte";
import Preferences from "./ui/prefs/Preferences.svelte";
import SealedBanner from "./ui/SealedBanner.svelte";
import ShareDialog from "./ui/ShareDialog.svelte";
import HeaderBar from "./ui/shell/HeaderBar.svelte";
import StatusBar from "./ui/shell/StatusBar.svelte";
import { showStartupNotice } from "./ui/startupNotice";
import { ClosePage, closePublisher } from "./workspace/closePage";
import { nativeCloseProbe } from "./workspace/nativeCloseProbe";
import { observeDocText, observeHeads, observeLinks } from "./workspace/observe";
import { OpenRequests } from "./workspace/openRequests";
import { offersTable, viewChoice } from "./workspace/viewChoice.svelte";
import { workspace } from "./workspace/workspace.svelte";

let languageMounted = $state(false);
let sharing = $state(false);
let people = $state(false);
let preferences = $state(false);
let about = $state(false);
/** The button that opened About, when it is not the main menu: it gets the focus back. */
let aboutOpener = $state<HTMLElement | undefined>();
let buildInfo = $state.raw<BuildInfo | undefined>();
const me = $derived({ id: workspace.myId, name: workspace.myName });

// The text editor and Typst's grammar: the main document opens with them.
activateStartup();
const mainFile = $derived(workspace.current === "main.typ");
/** How the open file opens (ADR 0027): the main document is text; another file as Go read it. */
const opening = $derived(
  mainFile
    ? registry.open(workspace.current, {
        choice: viewChoice.typeFor(workspace.project, workspace.current),
        content: "text",
      })
    : workspace.opened?.opening,
);
/** The view shown: the one the person switched to (the text of a table), else the default. */
const shown = $derived(
  opening &&
    (opening.alternatives.find(
      (v) => v.id === viewChoice.viewerFor(workspace.project, workspace.current),
    ) ??
      opening.viewer),
);
const textEditor = registry.essentialViewer("text");
/** A viewer that draws its own status bar (the grid, `ownStatus`): the shell's and the preview stand aside. */
const viewerStatus = $derived(shown?.ownStatus === true);
/** The file's formatting language: its grammar's format entry (EX0c3); undefined is plain text. */
const lang = $derived(registry.format(opening?.type.grammar));
/** The text of a file read from the disk, one object per read (a grid keeps its session by it). */
const openedText = $derived(
  workspace.opened?.text !== undefined ? { text: workspace.opened.text } : undefined,
);

/**
 * After a switch of view, the keyboard goes to the view just shown (UX-EX0-4): the editor, or
 * the grid, once it is in the page (its code and its parse may take a few frames).
 */
function focusShown() {
  const until = performance.now() + 3000;
  const step = () => {
    const target =
      shown === textEditor ? session.view : document.querySelector<HTMLElement>("[role=grid]");
    if (target) target.focus();
    else if (performance.now() < until) requestAnimationFrame(step);
  };
  void tick().then(() => requestAnimationFrame(step));
}

/** A text file that reads as a table is offered as one (never the main document, so not at start). */
const offerTable = $derived(
  !mainFile &&
    workspace.opened !== undefined &&
    offersTable(workspace.opened.opening, workspace.opened.text),
);
/** Opens the current file as a table from now on, on this device. */
function openAsTable() {
  viewChoice.openAsTable(workspace.project, workspace.current);
  void workspace.openFile(workspace.current);
}

/** Back to the file's default view (the grid) from another one, on the cell under the caret. */
function showDefault() {
  viewChoice.show(
    workspace.project,
    workspace.current,
    undefined,
    session.state?.selection.main.head,
  );
  focusShown();
}
/** The text view of the file, the caret at `offset`. */
function showText(offset: number) {
  viewChoice.show(workspace.project, workspace.current, textEditor.id, offset);
  focusShown();
}
/** A file whose formats show an item of the `format` location has a bar (L-6); csv, bib, txt none. */
const hasBar = $derived(mainFile && locationShows("format", { formats: formatsOf(lang) }));
const barVisible = $derived(hasBar && !workspace.big && settings.toolbarVisible(workspace.project));
const sidebarOpen = $derived(settings.sidebarVisible(workspace.project));
const previewOpen = $derived(settings.previewVisible(workspace.project));
/** The main document is presented: its pages are the slides (Present is Typst's, its `when`). */
const canPresent = $derived(mainFile);
/** The presentation takes the preview's place: it is the one that asks Go for pages. */
const presenting = $derived(presentSession.current !== undefined);

function present(how: "start" | "here" | "public") {
  if (!canPresent || presentSession.current) return;
  if (how === "public") presentSession.start("public");
  else presentSession.start("presenter", how === "here" ? { page: preview.currentPage || 1 } : {});
}

/** Back from a presentation: the caret goes to the slide that was on. */
function leftPresentation(page: number) {
  if (page > 0) void jumpFromPage(page, 0, 0);
}

function toggleToolbar() {
  settings.setToolbarVisible(workspace.project, !settings.toolbarVisible(workspace.project));
  session.focus();
}

function toggleSidebar() {
  settings.setSidebarVisible(workspace.project, !sidebarOpen);
}

function togglePreview() {
  settings.setPreviewVisible(workspace.project, !previewOpen);
}

/** Not in the file yet: shown when saving with Ctrl+S, or when a write failed or has no file to go to. */
const unsaved = $derived(
  mainFile &&
    workspace.dirty &&
    (settings.saveMode === "manual" ||
      workspace.projectionFailed ||
      workspace.notice?.kind === "deleted"),
);

$effect(() => {
  document.title = `${unsaved ? "● " : ""}${t("app.title")} — ${workspace.current}`;
});

// The saving settings change while a write may be waiting (manual to automatic writes at once).
$effect(() => {
  void settings.saveMode;
  void settings.saveDelay;
  workspace.saveSettingsChanged();
});

// The lines brought in from the disk are tinted in the editor; the toast's "View" goes to them.
$effect(() => {
  const marks = workspace.marks;
  const view = session.view;
  if (view) markExternal(view, marks?.spans ?? null);
});

$effect(() => {
  const view = session.view;
  const file = workspace.current;
  const entries = problems.entries;
  if (!view) return;
  const handle = file === "main.typ" ? workspace.handle : undefined;
  // Anchors are places in the document: resolved once the keys typed since the paint are in it.
  const mark = () => {
    if (view !== session.view) return;
    const doc = handle?.doc();
    const text = view.state.doc.toString();
    const marks = entries
      .filter((d) => d.file === file && !d.stale)
      .map((d) => ({
        ...d,
        ...(doc && d.anchor
          ? resolveRange(doc, d.anchor)
          : d.position
            ? { from: toOffset(text, d.position.start), to: toOffset(text, d.position.end) }
            : {}),
      }));
    view.dispatch({ effects: problemMarks.of(marks) });
  };
  if (handle) whenWritten(handle, mark);
  else mark();
});

$effect(() => {
  const diagnostics = preview.diagnostics;
  const revision = preview.diagnosticRevision;
  const hashes = preview.sourceHashes;
  untrack(() => void problems.compilerVerified(diagnostics, revision, hashes));
});

// Projection views announce the active file without publishing main-document cursors. A file
// name peers would refuse (List does not validate names: NFD, a backslash, hidden characters) is
// not announced at all, rather than letting peers place this person on main.typ.
$effect(() => {
  const handle = workspace.handle;
  const file = workspace.current;
  const name = workspace.myName;
  if (!handle || file === "main.typ" || !validFilePath(file)) return;
  const presence = new Presence<CursorState, import("./workspace/workspace.svelte").Doc>({
    handle,
  });
  presence.start({ initialState: { name, file } });
  const unregister = closePublisher(
    () => presence.stop(),
    () => presence.start({ initialState: presence.getLocalState() }),
  );
  const refresh = () =>
    workspace.hearNames(
      Object.values(presence.getPeerStates().value).map((p) => ({
        peerId: p.peerId,
        name: p.value.name,
        ...(p.value.file ? { file: p.value.file } : {}),
      })),
    );
  for (const event of ["update", "snapshot", "goodbye", "pruning"] as const)
    presence.on(event, refresh);
  return () => {
    unregister();
    presence.stop();
  };
});

// The commands of the registry (ext/app, ext/typst) act through the shell: what the header bar,
// the main menu and the window's keys did by hand before EX0b1. One key listener runs them all.
/** How many times `app.save` ran (EX0b1-E1: a held Ctrl+S saves once), for the end-to-end tests only. */
let saves = 0;
configure({
  shell: {
    save: () => {
      saves++;
      void workspace.save();
    },
    toggleToolbar,
    togglePreview,
    toggleSidebar,
    toggleReview: () => history.toggle(),
    markSeen: () => history.markSeen(),
    // An Insert item with a step: the bar is shown first if the person hid it (its setting).
    step: (item) => {
      settings.setToolbarVisible(workspace.project, true);
      openInsertStep(item as InsertItem);
    },
    share: () => (sharing = true),
    people: () => (people = true),
    exportPdf: () => void exportPdf(),
    preferences: () => (preferences = true),
    about: () => {
      aboutOpener = undefined;
      about = true;
    },
    returnToPrevious: () => void workspace.returnToPrevious(),
    problem: (reverse) => problems.next(reverse),
    zoom: (step) => {
      zoom[step](workspace.project);
      void keepCaret();
    },
    present,
    jumpToPreview: () => {
      if (!previewOpen) togglePreview();
      void jumpFromCursor();
    },
    fullscreen: () => void toggleFullscreen(document),
    ...dialogShell,
  },
  // What was typed is in the document before any command acts on it (a save keeps the last letter).
  beforeCommand: () => flushAllTyped(),
  announce: (text) => notice.show(text, "status"),
  blocked: () => workspace.closing || presenting,
});

// A side opens on a panel (`openPanel`): the files by their setting, the drawer as the counter's
// Shift+click always opened it (no preview swap: that is the toggle's).
configurePanels({
  start: () => settings.setSidebarVisible(workspace.project, true),
  end: () => (history.open = true),
});

// A step asked for from the palette belongs to this file and this bar (EX0c3 review).
$effect(() => noteInsertHost({ file: workspace.current, bar: !!barVisible }));

// What `when` clauses read (extensions/when.ts): the open file and the panels.
$effect(() => {
  setLive({
    fileType: opening?.type.id ?? "",
    formats: formatsOf(lang),
    inserts: insertsOf(lang),
    // A big file opens read-only (ADR 0028): the format and Insert commands say so.
    readOnly: !mainFile || workspace.locked || workspace.big !== undefined,
    hasToolbar: hasBar,
    toolbarVisible: barVisible,
    previewOpen,
    sidebarOpen,
    reviewOpen: history.open,
    canPresent,
    canReturn: workspace.canReturn && !workspace.leaving,
    main: mainFile,
    text: mainFile || workspace.opened?.text !== undefined,
  });
});

async function refresh() {
  await Promise.all([workspace.refreshFiles(), workspace.refreshInfo()]);
}

// The file list and the encoding come from the project folder, which other programs change too.
$effect(() => {
  void workspace.notice;
  void refresh();
});

// The preview dates the document by the last change the document itself records, so that every
// person sees the same date; Go compiles after the text has been written to the project folder.
$effect(() => {
  const handle = workspace.handle;
  if (!handle) return;
  const note = () => {
    if (import.meta.env.VITE_PERF_NATIVE === "1") {
      // Same work, split into labelled stages for the native attribution table only.
      profileNativeStage("app-change-note", () => {
        const doc = handle.doc();
        if (!doc) return;
        profileNativeStage("preview-noteEdit", () => preview.noteEdit(lastChangeSeconds(doc)));
        const text = profileNativeStage("automerge-doc-text", () => doc.text);
        const heads = profileNativeStage("automerge-getHeads", () => A.getHeads(doc).join(","));
        profileNativeStage("language-didChange", () => language.open("main.typ", text, heads));
        profileNativeStage("problems-edit", () => problems.edit(text));
      });
      return;
    }
    const doc = handle.doc();
    if (doc) preview.noteEdit(lastChangeSeconds(doc));
    if (doc) {
      language.open("main.typ", doc.text, A.getHeads(doc).join(","));
      problems.edit(doc.text);
    }
  };
  untrack(note);
  handle.on("change", note);
  return () => handle.off("change", note);
});

$effect(() => {
  const file = workspace.current;
  const text = workspace.opened?.text;
  if (file !== "main.typ" && workspace.fileTypeOf(file)?.id === LANGUAGE_TYPE && text !== undefined)
    language.open(file, text);
});

$effect(() => {
  const project = workspace.project;
  if (!languageMounted || !project) return;
  untrack(() => {
    problems.reset();
    language.reset();
    const doc = workspace.handle?.doc();
    if (doc) language.open("main.typ", doc.text, A.getHeads(doc).join(","));
  });
  return () => language.stop();
});

onMount(() => {
  observeDocText(() => session.state?.doc.toString());
  observeHeads(() => workspace.handle?.heads());
  observeLinks(
    () => links.view(),
    () => workspace.status,
  );
  if (import.meta.env.VITE_E2E_OBSERVE === "1") {
    (
      globalThis as typeof globalThis & { __inkwellLanguageVersion?: () => number }
    ).__inkwellLanguageVersion = () => language.version(workspace.current);
    (
      globalThis as typeof globalThis & { __inkwellLanguageReady?: () => boolean }
    ).__inkwellLanguageReady = () => language.ready;
    (
      globalThis as typeof globalThis & {
        __inkwellLanguageRequest?: (method: string, offset: number) => Promise<unknown>;
      }
    ).__inkwellLanguageRequest = (method, offset) => {
      if (
        !["textDocument/completion", "textDocument/signatureHelp", "textDocument/hover"].includes(
          method,
        ) ||
        !Number.isInteger(offset)
      )
        return Promise.resolve(undefined);
      const text = session.state?.doc.toString();
      return text === undefined
        ? Promise.resolve(undefined)
        : language.request(method, workspace.current, toPosition(text, offset));
    };

    (globalThis as typeof globalThis & { __inkwellSelection?: () => string }).__inkwellSelection =
      () =>
        session.state
          ? session.state.doc.sliceString(
              session.state.selection.main.from,
              session.state.selection.main.to,
            )
          : "";
    // A bound call made by the page itself, to prove what crosses the real bridge (W58, N-2).
    (
      globalThis as typeof globalThis & {
        __inkwellInvite?: (role: string) => Promise<{ refused?: string }>;
      }
    ).__inkwellInvite = (role) => Session.Invite(role);
    (globalThis as typeof globalThis & { __inkwellTestTable?: () => void }).__inkwellTestTable =
      () => viewChoice.openAsTable(workspace.project, "main.typ");
    (globalThis as typeof globalThis & { __inkwellSaves?: () => number }).__inkwellSaves = () =>
      saves;
    // What a location shows now, by the registry (EX0c1-E1, E2): its items, or the status bar's.
    (
      globalThis as typeof globalThis & {
        __inkwellLocation?: (location: string, target?: string) => string[];
      }
    ).__inkwellLocation = (location, target?: string) =>
      location === "status"
        ? statusOf().map((x) => x.id)
        : menuOf(location, {}, (target && document.querySelector(target)) || undefined).flatMap(
            (g) => g.items.map((c) => c.id),
          );
    // A side's panels now, by the registry (EX0c2-E1).
    (
      globalThis as typeof globalThis & { __inkwellPanels?: (side: "start" | "end") => string[] }
    ).__inkwellPanels = (side) => panelsOf(side).map((p) => p.id);
  }
  preview.start(() => Stream("preview") as unknown as SocketLike);
  configureLanguage(
    () => Stream("lsp") as unknown as SocketLike,
    (file, version, items) => problems.publish(file, version, items),
    () => workspace.current,
  );
  languageMounted = true;
  const removeKeys = installKeys();
  warmCommands();
  // The drawer's code at idle after the first paint: its first opening waits on nothing (EX0c2).
  prefetchSide("end");
  let nativePerfStop: (() => void) | undefined;
  let mounted = true;
  const closeWalk =
    import.meta.env.VITE_E2E_OBSERVE === "1" &&
    new URLSearchParams(location.search).get("wc1Native");
  // The native walk checks the Ctrl+S mode, and automatic saving where its mode says so.
  if (closeWalk) settings.setSaveMode(closeWalk === "auto" ? "auto" : "manual");
  let nativeProbe: ReturnType<typeof nativeCloseProbe> | undefined;
  const closePage = new ClosePage({
    save: (signal) => workspace.saveForClose(signal),
    goodbye: async (attempt, signal) => {
      await workspace.goodbyeForClose(attempt, signal);
      await nativeProbe?.waitForSelectionClear(signal);
    },
    restore: () => workspace.resumeAfterClose(),
    describe: (error) => {
      if (!(error instanceof SaveError)) return {};
      return {
        ...(error.message === "disk-full" ? { reason: "disk-full" } : {}),
        ...(error.document && error.document === workspace.handle?.documentId
          ? { files: ["main.typ"] }
          : {}),
      };
    },
  });
  let desktopClose = false;
  try {
    desktopClose = Flags.GetFlag("inkwellClose") === true;
  } catch {
    // The browser transport can start without the native bootstrap flags.
  }
  const allowed = desktopClose ? closePage.ready : Promise.resolve();
  const started = allowed.then(() => {
    if (mounted) return workspace.start().then(refresh);
  });
  if (closeWalk) nativeProbe = nativeCloseProbe(closeWalk, started);
  if (desktopClose)
    workspace.authorizeProjectLeave = () => closePage.requestProject() ?? Promise.resolve(false);
  if (import.meta.env.VITE_E2E_OBSERVE === "1") {
    (
      globalThis as typeof globalThis & { __inkwellPrepareDeparture?: () => Promise<void> }
    ).__inkwellPrepareDeparture = async () => {
      const signal = AbortSignal.timeout(5000);
      try {
        await workspace.saveForClose(signal);
        await workspace.goodbyeForClose(1, signal);
      } catch (error) {
        workspace.resumeAfterClose();
        throw error;
      }
    };
  }
  // After a take-over restarted Inkwell: how it ended, once, and the focus in the text (PC1a, N-12).
  void showStartupNotice(
    () => {
      if (mounted) session.focus();
    },
    started.then(() => tick()),
  );
  // What the desktop asked Inkwell to open (« Ouvrir avec », a second launch): Go raised the window.
  const openLog: { n: number; show?: string }[] = [];
  if (import.meta.env.VITE_E2E_OBSERVE === "1")
    (
      globalThis as typeof globalThis & { __inkwellOpenApplied?: typeof openLog }
    ).__inkwellOpenApplied = openLog;
  const openRequests = new OpenRequests(
    () => Stream("open") as unknown as SocketLike,
    {
      ready: started,
      closePage,
      prepare: async (signal) => {
        viewChoice.arrive();
        await workspace.refreshFiles(); // a file just made outside Inkwell may not be listed yet
        signal.throwIfAborted();
      },
      show: async (path, signal) => {
        signal.throwIfAborted(); // the stream was superseded: Go sends the request again
        await workspace.openFile(path);
        // A file the project does not list (hidden, temporary) is not shown: say so (R19).
        return workspace.current === path;
      },
      toast: (text, role) => notice.queue(text, role),
    },
    1000,
    (r, n) => openLog.push(r.show === undefined ? { n } : { n, show: r.show }),
  );
  openRequests.start();
  if (import.meta.env.VITE_PERF_NATIVE === "1") {
    void started
      .then(() => tick())
      .then(async () => {
        if (!mounted) return;
        if (new URLSearchParams(location.search).has("typing-check")) {
          // Typing checks (scripts/perf/typing_checks.py): no measurement in this mode.
          const { startTypingCheck } = await import("./perf/typingCheck");
          const { readOnlyFacets } = await import("./editor/setup");
          if (!mounted) return;
          nativePerfStop = startTypingCheck({
            open: () => Stream("typing-check") as unknown as SocketLike,
            view: () => session.view,
            readOnly: readOnlyFacets,
          });
          return;
        }
        const { startNativePerf } = await import("./perf/native");
        if (!mounted) return;
        nativePerfStop = startNativePerf({
          open: () => Stream("native-perf") as unknown as SocketLike,
          streams: WailsSocket.prototype,
          view: () => session.view,
          revision: () => {
            const doc = workspace.active?.doc();
            return doc ? A.getHeads(doc).join(",") : "";
          },
          foreignChange: (before, after) => {
            const doc = workspace.active?.doc();
            if (!doc || !before || !after) return undefined;
            try {
              return A.diff(doc, before.split(","), after.split(",")).map((p) => p.path.join("/"));
            } catch {
              return undefined;
            }
          },
        });
      });
  }
  // The About dialog just stays without a version if Go does not answer.
  Session.AppVersion()
    .then((info) => (buildInfo = info))
    .catch(() => undefined);
  const timer = setInterval(() => {
    if (!document.hidden) void refresh();
  }, 4000);
  return () => {
    mounted = false;
    removeKeys();
    workspace.authorizeProjectLeave = undefined;
    openRequests.stop();
    nativeProbe?.stop();
    nativePerfStop?.();
    clearInterval(timer);
    preview.stop();
    language.stop();
    void workspace.stop();
  };
});
</script>

<!-- A presentation covers the window: what is under it neither takes keys nor focus. -->
<div class="window" inert={presenting}>
  {#if sidebarOpen}
    <PanelSide side="start" />
  {/if}
  <div class="column">
    <HeaderBar
      title={t("app.title")}
      subtitle={workspace.current}
      {unsaved}
      status={workspace.status}
      sending={workspace.syncSending}
      slow={workspace.syncSlow}
      {me}
      peers={workspace.peers}
    />
    {#if barVisible && session.state && lang}
      <FormatBar
        {lang}
        editorState={session.state}
        target={session.view?.dom}
        run={session.run}
        files={workspace.files}
        currentFile={workspace.current}
        readFile={(path) => workspace.readFile(path)}
      />
    {/if}
    <SealedBanner />
    <AccessBanner onAsk={() => (sharing = true)} />
    <ViewBar
      offer={offerTable}
      onoffer={openAsTable}
      defaultViewer={opening?.viewer}
      {shown}
      ondefault={showDefault}
    />
    <ProjectionBanner />
    {#if workspace.tier === "large" && !workspace.big}
      <Banner role="status">{t("limit.slow")}</Banner>
    {/if}
    <ExternalBanner />
    <PresentBanner />
    <HistoryBars />
    <main>
      <div class="pane side-source" class:projection={!mainFile}>
        {#if !mainFile}
          {#key workspace.current}
            {#if workspace.opened && shown}
              <ViewerHost
                viewer={shown}
                opening={workspace.opened.opening}
                path={workspace.current}
                {lang}
                {me}
                big={openedText}
                readOnly
                readOnlyReason={t("file.readOnly")}
                failed={workspace.opened.failed}
                large={workspace.opened.large}
                startOffset={viewChoice.offset}
                onshowtext={showText}
                readImage={(path) => workspace.readImage(path)}
              />
            {:else}
              <div class="file-view" tabindex="-1" role="status"><h2>{workspace.current}</h2><p>{t("file.loading")}</p></div>
            {/if}
          {/key}
        {:else if workspace.big && opening && shown}
          {#key workspace.big}
            <ViewerHost
              viewer={shown}
              {opening}
              path={workspace.current}
              {lang}
              {me}
              big={workspace.big}
              startOffset={viewChoice.offset}
              onshowtext={showText}
              readImage={(path) => workspace.readImage(path)}
            />
          {/key}
        {:else if workspace.waitingFor !== undefined}
          <JoinWaiting />
        {:else if workspace.active && opening && shown}
          {@const active = workspace.active}
          {#key active.url}
            <ViewerHost
              viewer={shown}
              {opening}
              path={workspace.current}
              {lang}
              {me}
              handle={active}
              startOffset={viewChoice.offset}
              onshowtext={showText}
              readImage={(path) => workspace.readImage(path)}
            />
          {/key}
        {/if}
      </div>
      <!-- While the document of a join is waited for there is nothing to preview (PC1b, J-2). -->
      {#if mainFile && previewOpen && !viewerStatus && !presenting && !workspace.big && workspace.waitingFor === undefined}
        <div class="pane side-preview">
          <Preview
            onSourceJump={jumpFromPage}
            onDetails={(from) => {
              aboutOpener = from;
              about = true;
            }}
          />
        </div>
      {/if}
    </main>
    {#if !viewerStatus}
      <StatusBar
        {lang}
        status={workspace.status}
        peerCount={workspace.peers.length}
        editorState={session.state}
        info={workspace.fileInfo}
        ofMain={mainFile}
        onProblem={jumpToProblem}
        problemsNav
        {unsaved}
      />
    {/if}
  </div>
  {#if history.open}<PanelSide side="end" />{/if}
  <HistoryDialogs />
</div>
<PresentHost onExit={leftPresentation} />
<ShareDialog bind:open={sharing} />
<PeopleDialog bind:open={people} />
<Preferences bind:open={preferences} />
<NoticeToast />
<AboutDialog bind:open={about} info={buildInfo} opener={aboutOpener} />
<CommandDialogs />
<ExternalChangeToast />

<style>
 .projection { display: flex; flex-direction: column; }
 .file-view { flex: 1; min-height: 0; overflow: auto; padding: 24px; color: var(--fg-dim); background: var(--view-bg); }
  .window {
    display: flex;
    height: 100vh;
    overflow: hidden;
  }
  .column {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-width: 0;
    container-type: inline-size;
  }
  main {
    position: relative;
    display: flex;
    flex: 1;
    min-height: 0;
    container-type: inline-size;
  }
  .pane {
    min-width: 0;
    min-height: 0;
  }
  .side-source {
    flex: 1 1 0;
    min-width: 280px;
  }
  .side-preview {
    flex: 1 1 0;
    min-width: 260px;
    max-width: 55%;
  }
  /* Too narrow for two panes: the preview covers the source until it is hidden. */
  @container (max-width: 560px) {
    .side-preview {
      position: absolute;
      inset: 0;
      max-width: none;
      z-index: 5;
    }
  }
</style>
