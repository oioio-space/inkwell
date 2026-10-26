<script lang="ts">
import { onMount } from "svelte";
import { Session } from "../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { markExternal } from "./editor/external";
import { langOf } from "./editor/format";
import { t } from "./i18n";
import { settings } from "./store/settings.svelte";
import AboutDialog, { type BuildInfo } from "./ui/AboutDialog.svelte";
import Editor from "./ui/Editor.svelte";
import ExternalBanner from "./ui/ExternalBanner.svelte";
import ExternalChangeToast from "./ui/ExternalChangeToast.svelte";
import { session } from "./ui/editorSession.svelte";
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
let buildInfo = $state.raw<BuildInfo | undefined>();
const me = $derived({ id: workspace.myId, name: workspace.myName });

const lang = $derived(langOf(workspace.current));
/** Only Typst and Markdown have a formatting bar; csv, bib and txt are just the editor. */
const barLanguage = $derived(barLang(workspace.current));
const hasBar = $derived(barLanguage !== undefined);
const barVisible = $derived(hasBar && settings.toolbarVisible(workspace.project));
const sidebarOpen = $derived(settings.sidebarVisible(workspace.project));

function toggleToolbar() {
  settings.setToolbarVisible(workspace.project, !settings.toolbarVisible(workspace.project));
  session.focus();
}

function toggleSidebar() {
  settings.setSidebarVisible(workspace.project, !sidebarOpen);
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

onMount(() => {
  void workspace.start().then(refresh);
  // The About dialog just stays without a version if Go does not answer.
  Session.AppVersion()
    .then((info) => (buildInfo = info))
    .catch(() => undefined);
  const timer = setInterval(() => {
    if (!document.hidden) void refresh();
  }, 4000);
  return () => clearInterval(timer);
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
      {unsaved}
      status={workspace.status}
      {me}
      peers={workspace.peers}
      {sidebarOpen}
      toolbarVisible={barVisible}
      canToggleToolbar={hasBar}
      onShare={() => (sharing = true)}
      onToggleSidebar={toggleSidebar}
      onToggleToolbar={toggleToolbar}
      onPreferences={() => (preferences = true)}
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
    <ExternalBanner />
    <main>
      {#if workspace.handle}
        {#key workspace.handle.url}
          <Editor handle={workspace.handle} {me} {lang} />
        {/key}
      {/if}
    </main>
    <StatusBar
      {lang}
      status={workspace.status}
      peerCount={workspace.peers.length}
      editorState={session.state}
      info={workspace.fileInfo}
      {unsaved}
    />
  </div>
</div>
<ShareDialog bind:open={sharing} />
<Preferences bind:open={preferences} />
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
    flex: 1;
    min-height: 0;
  }
</style>
