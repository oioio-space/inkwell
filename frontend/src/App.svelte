<script lang="ts">
import { Stream } from "@wailsio/runtime";
import { onMount } from "svelte";
import { Session } from "../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { looksTabular } from "./editor/csv/dialect";
import { markExternal } from "./editor/external";
import { langOf } from "./editor/format";
import { t } from "./i18n";
import type { SocketLike } from "./net/WailsNetworkAdapter";
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
import { workspace } from "./workspace/workspace.svelte";

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
    looksTabular(workspace.handle?.doc()?.text ?? ""),
);
const showSheet = $derived(showsSheet(workspace.current));

/** Back to the grid from the text view, on the cell under the caret. */
function showTable() {
  sheetView.toGrid(session.state?.selection.main.head);
}
/** Only Typst and Markdown have a formatting bar; csv, bib and txt are just the editor. */
const barLanguage = $derived(barLang(workspace.current));
const hasBar = $derived(barLanguage !== undefined);
const barVisible = $derived(hasBar && !workspace.big && settings.toolbarVisible(workspace.project));
const sidebarOpen = $derived(settings.sidebarVisible(workspace.project));
const previewOpen = $derived(settings.previewVisible(workspace.project));
/** Only a Typst document is presented: its pages are the slides. */
const canPresent = $derived(lang === "typst");
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

function onKey(e: KeyboardEvent) {
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
    const doc = handle.doc();
    if (doc) preview.noteEdit(lastChangeSeconds(doc));
  };
  note();
  handle.on("change", note);
  return () => handle.off("change", note);
});

onMount(() => {
  preview.start(() => Stream("preview") as unknown as SocketLike);
  void workspace.start().then(refresh);
  // The About dialog just stays without a version if Go does not answer.
  Session.AppVersion()
    .then((info) => (buildInfo = info))
    .catch(() => undefined);
  const timer = setInterval(() => {
    if (!document.hidden) void refresh();
  }, 4000);
  return () => {
    clearInterval(timer);
    preview.stop();
  };
});
</script>

<svelte:window onkeydown={onKey} />

<!-- A presentation covers the window: what is under it neither takes keys nor focus. -->
<div class="window" inert={presenting}>
  {#if sidebarOpen}
    <Sidebar files={workspace.files} current={workspace.current} onOpen={() => session.focus()} />
  {/if}
  <div class="column">
    <HeaderBar
      title={t("app.title")}
      subtitle={workspace.current}
      {unsaved}
      status={workspace.status}
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
      <div class="pane side-source">
        {#if workspace.big}
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
      {#if previewOpen && !showSheet && !presenting && !workspace.big}
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
        onProblem={jumpToProblem}
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
