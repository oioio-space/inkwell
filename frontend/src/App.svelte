<script lang="ts">
import { next as A } from "@automerge/automerge";
import { Presence } from "@automerge/automerge-repo";
import { Stream, WailsSocket } from "@wailsio/runtime";
import { onMount, tick, untrack } from "svelte";
import { Session } from "../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { whenWritten } from "./editor/amSync";
import { looksTabular } from "./editor/csv/dialect";
import { markExternal } from "./editor/external";
import { langOf } from "./editor/format";
import type { CursorState } from "./editor/presence";
import { t } from "./i18n";
import { problemMarks } from "./language/marks";
import { resolveRange, toOffset, toPosition } from "./language/positions";
import { problems } from "./language/problems.svelte";
import { configureLanguage, language } from "./language/service";
import { validFilePath } from "./net/admission";
import { links } from "./net/links.svelte";
import type { SocketLike } from "./net/WailsNetworkAdapter";
import { profileNativeStage } from "./perf/nativeDispatch";
import PresentBanner from "./present/PresentBanner.svelte";
import PresentHost from "./present/PresentHost.svelte";
import { presentSession } from "./present/session.svelte";
import { exportPdf, jumpFromCursor, jumpFromPage, jumpToProblem } from "./preview/actions";
import { lastChangeSeconds } from "./preview/docTime";
import Preview from "./preview/Preview.svelte";
import { preview } from "./preview/preview.svelte";
import { settings } from "./store/settings.svelte";
import AboutDialog, { type BuildInfo } from "./ui/AboutDialog.svelte";
import Banner from "./ui/Banner.svelte";
import Editor from "./ui/Editor.svelte";
import ExternalBanner from "./ui/ExternalBanner.svelte";
import ExternalChangeToast from "./ui/ExternalChangeToast.svelte";
import { session } from "./ui/editorSession.svelte";
import HistoryBars from "./ui/history/HistoryBars.svelte";
import HistoryDrawer from "./ui/history/HistoryDrawer.svelte";
import { history } from "./ui/history/history.svelte";
import NoticeToast from "./ui/NoticeToast.svelte";
import ProjectionBanner from "./ui/ProjectionBanner.svelte";
import AccessBanner from "./ui/people/AccessBanner.svelte";
import PeopleDialog from "./ui/people/PeopleDialog.svelte";
import Preferences from "./ui/prefs/Preferences.svelte";
import ReadOnlyText from "./ui/ReadOnlyText.svelte";
import SealedBanner from "./ui/SealedBanner.svelte";
import ShareDialog from "./ui/ShareDialog.svelte";
import Sheet from "./ui/sheet/Sheet.svelte";
import { sheetView, showsSheet } from "./ui/sheet/viewMode.svelte";
import HeaderBar from "./ui/shell/HeaderBar.svelte";
import Sidebar from "./ui/shell/Sidebar.svelte";
import StatusBar from "./ui/shell/StatusBar.svelte";
import FormatBar from "./ui/toolbar/FormatBar.svelte";
import { barLang } from "./ui/toolbar/spec";
import { observeDocText, observeLinks } from "./workspace/observe";
import { workspace } from "./workspace/workspace.svelte";

let languageMounted = $state(false);
let sharing = $state(false);
let people = $state(false);
let preferences = $state(false);
let about = $state(false);
let buildInfo = $state.raw<BuildInfo | undefined>();
const me = $derived({ id: workspace.myId, name: workspace.myName });

const lang = $derived(langOf(workspace.current));
/** A table file shows as a spreadsheet, unless the person asked for its text. */
const tabular = $derived(sheetView.tabular(workspace.current));
/** A `.txt` that reads as a table is offered as one, never opened as one by itself. */
const offerTable = $derived(
  !tabular &&
    /\.txt$/i.test(workspace.current) &&
    looksTabular(workspace.opened?.text ?? workspace.handle?.doc()?.text ?? ""),
);
const showSheet = $derived(showsSheet(workspace.current));

/** Back to the grid from the text view, on the cell under the caret. */
function showTable() {
  sheetView.toGrid(session.state?.selection.main.head);
}
/** Only Typst and Markdown have a formatting bar; csv, bib and txt are just the editor. */
const barLanguage = $derived(barLang(workspace.current));
const mainFile = $derived(workspace.current === "main.typ");
const hasBar = $derived(mainFile && barLanguage !== undefined);
const barVisible = $derived(hasBar && !workspace.big && settings.toolbarVisible(workspace.project));
const sidebarOpen = $derived(settings.sidebarVisible(workspace.project));
const previewOpen = $derived(settings.previewVisible(workspace.project));
/** Only a Typst document is presented: its pages are the slides. */
const canPresent = $derived(mainFile && lang === "typst");
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
  return () => presence.stop();
});

function onKey(e: KeyboardEvent) {
  if (e.key === "F8" && !e.ctrlKey && !e.metaKey && !e.altKey) {
    e.preventDefault();
    problems.next(e.shiftKey);
    return;
  }
  if ((e.ctrlKey || e.metaKey) && !e.shiftKey && !e.altKey && e.key.toLowerCase() === "s") {
    e.preventDefault();
    void workspace.save();
    return;
  }
  if (e.key === "F5" && !e.ctrlKey && !e.metaKey && !e.altKey) {
    e.preventDefault();
    present(e.shiftKey ? "here" : "start");
    return;
  }
  if ((e.ctrlKey || e.metaKey) && e.altKey && e.key.toLowerCase() === "j") {
    e.preventDefault();
    if (!previewOpen) togglePreview();
    void jumpFromCursor();
    return;
  }
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key.toLowerCase() === "t") {
    e.preventDefault();
    if (hasBar) toggleToolbar();
  }
}

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
  if (file !== "main.typ" && file.endsWith(".typ") && text !== undefined) language.open(file, text);
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
    (globalThis as typeof globalThis & { __inkwellTestTable?: () => void }).__inkwellTestTable =
      () => sheetView.openAsTable("main.typ");
  }
  preview.start(() => Stream("preview") as unknown as SocketLike);
  configureLanguage(
    () => Stream("lsp") as unknown as SocketLike,
    (file, version, items) => problems.publish(file, version, items),
    () => workspace.current,
  );
  languageMounted = true;
  let nativePerfStop: (() => void) | undefined;
  let mounted = true;
  const started = workspace.start().then(refresh);
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
    nativePerfStop?.();
    clearInterval(timer);
    preview.stop();
    language.stop();
    void workspace.stop();
  };
});
</script>

<svelte:window onkeydown={onKey} />

<!-- A presentation covers the window: what is under it neither takes keys nor focus. -->
<div class="window" inert={presenting}>
  {#if sidebarOpen}
    <Sidebar files={workspace.files} current={workspace.current} onOpen={(path, keepFocus) => { sheetView.toGrid(undefined); void workspace.openFile(path).then(() => tick()).then(() => { if (keepFocus) return; if (session.view) session.focus(); else document.querySelector<HTMLElement>(".file-view, [role=grid]")?.focus(); }); }} />
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
      {sidebarOpen}
      toolbarVisible={barVisible}
      canToggleToolbar={hasBar}
      {previewOpen}
      historyOpen={history.open}
      onToggleHistory={() => history.toggle()}
      onShare={() => (sharing = true)}
      onPeople={() => (people = true)}
      onToggleSidebar={toggleSidebar}
      onToggleToolbar={toggleToolbar}
      onPreferences={() => (preferences = true)}
      onTogglePreview={togglePreview}
      onExport={exportPdf}
      onAbout={() => (about = true)}
      {canPresent}
      onPresent={present}
    />
    {#if barVisible && session.state && barLanguage}
      <FormatBar
        lang={barLanguage}
        editorState={session.state}
        run={session.run}
        files={workspace.files}
        currentFile={workspace.current}
        readFile={(path) => workspace.readFile(path)}
      />
    {/if}
    <SealedBanner />
    <AccessBanner />
    {#if offerTable}
      <div class="viewbar">
        <button
          type="button"
          class="fb lbl"
          data-sheet="open-table"
          onclick={() => sheetView.openAsTable(workspace.current)}
        >
          {t("sheet.openAsTable")}
        </button>
      </div>
    {/if}
    {#if tabular && sheetView.text}
      <div class="viewbar">
        <button type="button" class="fb lbl" data-sheet="table" onclick={showTable}>
          {t("sheet.showTable")}
        </button>
      </div>
    {/if}
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
            {#if workspace.opened?.text !== undefined}
              {#if showSheet}
                <Sheet big={{ text: workspace.opened.text }} {me} path={workspace.current} readOnly readOnlyReason={t("file.readOnly")} onshowtext={(offset) => sheetView.toText(offset)} />
              {:else}
                <ReadOnlyText text={workspace.opened.text} message={t("file.readOnly")} {lang} />
              {/if}
            {:else if workspace.opened?.image}
              <div class="file-view" tabindex="-1"><img src={workspace.opened.image} alt={workspace.current} /></div>
            {:else}
              <div class="file-view" tabindex="-1" role="status"><h2>{workspace.current}</h2><p>{t(workspace.opened?.failed ? "file.unavailable" : workspace.opened ? "file.info" : "file.loading")}</p></div>
            {/if}
          {/key}
        {:else if workspace.big}
          {#key workspace.big}
            {#if showSheet}
              <Sheet
                big={workspace.big}
                {me}
                path={workspace.current}
                startOffset={sheetView.offset}
                onshowtext={(offset) => sheetView.toText(offset)}
              />
            {:else}
              <ReadOnlyText text={workspace.big.text} />
            {/if}
          {/key}
        {:else if workspace.active}
          {@const active = workspace.active}
          {#key active.url}
            {#if showSheet}
              <Sheet
                handle={active}
                {me}
                path={workspace.current}
                startOffset={sheetView.offset}
                onshowtext={(offset) => sheetView.toText(offset)}
              />
            {:else}
              <Editor handle={active} {me} {lang} at={tabular ? sheetView.offset : undefined} />
            {/if}
          {/key}
        {/if}
      </div>
      {#if mainFile && previewOpen && !showSheet && !presenting && !workspace.big}
        <div class="pane side-preview">
          <Preview onSourceJump={jumpFromPage} onCursorJump={jumpFromCursor} />
        </div>
      {/if}
    </main>
    {#if !showSheet}
      <StatusBar
        {lang}
        status={workspace.status}
        peerCount={workspace.peers.length}
        editorState={session.state}
        info={workspace.fileInfo}
        showPage={previewOpen}
        ofMain={mainFile}
        hasText={mainFile || workspace.opened?.text !== undefined}
        onProblem={jumpToProblem}
        onNextProblem={(reverse)=>problems.next(reverse)}
        {unsaved}
      />
    {/if}
  </div>
  <HistoryDrawer />
</div>
<PresentHost onExit={leftPresentation} />
<ShareDialog bind:open={sharing} />
<PeopleDialog bind:open={people} />
<Preferences bind:open={preferences} />
<NoticeToast />
<AboutDialog bind:open={about} info={buildInfo} />
<ExternalChangeToast />

<style>
 .projection { display: flex; flex-direction: column; }
 .file-view { flex: 1; min-height: 0; overflow: auto; padding: 24px; color: var(--fg-dim); background: var(--view-bg); }
 .file-view img { max-width: 100%; max-height: 100%; object-fit: contain; }
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
  .viewbar {
    display: flex;
    align-items: center;
    min-height: 40px;
    padding: 3px 6px;
    background: var(--headerbar-bg);
    box-shadow: 0 1px var(--shade);
  }
</style>
