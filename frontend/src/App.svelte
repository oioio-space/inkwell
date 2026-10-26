<script lang="ts">
import { Stream } from "@wailsio/runtime";
import { onMount } from "svelte";
import { langOf } from "./editor/format";
import { t } from "./i18n";
import type { SocketLike } from "./net/WailsNetworkAdapter";
import { exportPdf, jumpFromCursor, jumpFromPage, jumpToProblem } from "./preview/actions";
import { lastChangeSeconds } from "./preview/docTime";
import Preview from "./preview/Preview.svelte";
import { preview } from "./preview/preview.svelte";
import { settings } from "./store/settings.svelte";
import AboutDialog from "./ui/AboutDialog.svelte";
import Editor from "./ui/Editor.svelte";
import ExternalChangeToast from "./ui/ExternalChangeToast.svelte";
import { session } from "./ui/editorSession.svelte";
import NoticeToast from "./ui/NoticeToast.svelte";
import ProjectionBanner from "./ui/ProjectionBanner.svelte";
import Preferences from "./ui/prefs/Preferences.svelte";
import SealedBanner from "./ui/SealedBanner.svelte";
import ShareDialog from "./ui/ShareDialog.svelte";
import HeaderBar from "./ui/shell/HeaderBar.svelte";
import Sidebar from "./ui/shell/Sidebar.svelte";
import StatusBar from "./ui/shell/StatusBar.svelte";
import FormatBar from "./ui/toolbar/FormatBar.svelte";
import { barLang } from "./ui/toolbar/spec";
import { workspace } from "./workspace/workspace.svelte";

let sharing = $state(false);
let preferences = $state(false);
let about = $state(false);
const me = $derived({ id: workspace.myId, name: workspace.myName });

const lang = $derived(langOf(workspace.current));
/** Only Typst and Markdown have a formatting bar; csv, bib and txt are just the editor. */
const barLanguage = $derived(barLang(workspace.current));
const hasBar = $derived(barLanguage !== undefined);
const barVisible = $derived(hasBar && settings.toolbarVisible(workspace.project));
const sidebarOpen = $derived(settings.sidebarVisible(workspace.project));
const previewOpen = $derived(settings.previewVisible(workspace.project));

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

function onKey(e: KeyboardEvent) {
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
  void workspace.external;
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

<div class="window">
  {#if sidebarOpen}
    <Sidebar files={workspace.files} current={workspace.current} onOpen={() => session.focus()} />
  {/if}
  <div class="column">
    <HeaderBar
      title={t("app.title")}
      subtitle={workspace.current}
      status={workspace.status}
      {me}
      peers={workspace.peers}
      {sidebarOpen}
      toolbarVisible={barVisible}
      canToggleToolbar={hasBar}
      {previewOpen}
      onShare={() => (sharing = true)}
      onToggleSidebar={toggleSidebar}
      onToggleToolbar={toggleToolbar}
      onPreferences={() => (preferences = true)}
      onTogglePreview={togglePreview}
      onExport={exportPdf}
      onAbout={() => (about = true)}
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
    <ProjectionBanner />
    <main>
      <div class="pane side-source">
        {#if workspace.handle}
          {#key workspace.handle.url}
            <Editor handle={workspace.handle} {me} {lang} />
          {/key}
        {/if}
      </div>
      {#if previewOpen}
        <div class="pane side-preview">
          <Preview onSourceJump={jumpFromPage} onCursorJump={jumpFromCursor} />
        </div>
      {/if}
    </main>
    <StatusBar
      {lang}
      status={workspace.status}
      peerCount={workspace.peers.length}
      editorState={session.state}
      info={workspace.fileInfo}
      showPage={previewOpen}
      onProblem={jumpToProblem}
    />
  </div>
</div>
<ShareDialog bind:open={sharing} />
<Preferences bind:open={preferences} />
<AboutDialog bind:open={about} />
<NoticeToast />
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
</style>
