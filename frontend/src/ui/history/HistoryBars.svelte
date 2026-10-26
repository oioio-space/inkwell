<script lang="ts">
import { EditorView } from "@codemirror/view";
import { tick, untrack } from "svelte";
import { PICK, showWritten, type Written } from "../../editor/blameMarks";
import { showChanges } from "../../editor/changeGutter";
import { changeMarks, resolver } from "../../history/gutter";
import { stamp } from "../../history/stamp";
import { t, tn } from "../../i18n";
import { zoom } from "../../store/zoom.svelte";
import { workspace } from "../../workspace/workspace.svelte";
import Banner from "../Banner.svelte";
import { session } from "../editorSession.svelte";
import { history } from "./history.svelte";

/**
 * What the history adds around the editor: the strip « Depuis votre dernière visite », the banner
 * when a variant is being edited, and the page-wide wiring (marks in the margin, « Qui a écrit
 * quoi », the zoom keys). It draws nothing when there is nothing to say.
 */
let count = $state(0);

$effect(() => {
  const h = workspace.handle;
  if (h) untrack(() => history.attach(h as never));
  return () => untrack(() => history.detach());
});

$effect(() => {
  void workspace.myName;
  void workspace.myId;
  untrack(() => history.renamed());
});

// The margin: what others changed since this device last left, anchored by Automerge cursors.
$effect(() => {
  const view = session.view;
  const a = history.analysis;
  const doc = history.doc;
  const handle = history.handle;
  if (!view || !a || !doc || !handle || workspace.variant) return;
  if (a.limited) {
    count = 0;
    return;
  }
  const marks = changeMarks(doc, a.blame, a.sinceIdx, history.author);
  count = marks.length;
  showChanges(view, { marks, colorBy: history.colorBy, resolve: resolver(handle.doc()) });
  return () => {
    try {
      showChanges(view, null);
    } catch {
      // The editor is already gone.
    }
  };
});

// « Qui a écrit quoi »: only while the analysis is of the text on screen.
$effect(() => {
  const view = session.view;
  const a = history.analysis;
  if (!view) return;
  if (!history.showWho || !a || a.limited || !history.isCurrent() || workspace.variant) {
    showWritten(view, null);
    return;
  }
  const spans: Written[] = [];
  for (const sp of a.blame.spans) {
    const s = a.blame.sessions[sp.s];
    if (!s) continue;
    const who = history.author(s.actor);
    spans.push({
      from: sp.from,
      to: sp.to,
      person: who.index,
      entry: s.id,
      label: t("history.who.hover", {
        name: who.name,
        when: stamp(s.end, s.skewed, who.name),
        time: stamp(s.start, false, who.name),
      }),
    });
  }
  showWritten(view, spans);
});

$effect(() => {
  const onPick = (e: Event) => history.show((e as CustomEvent<string>).detail);
  window.addEventListener(PICK, onPick);
  return () => window.removeEventListener(PICK, onPick);
});

// Text zoom: a setting of this device, per project, applied through one variable on the page.
const level = $derived(zoom.level(workspace.project));
$effect(() => {
  document.documentElement.style.setProperty("--text-zoom", String(level / 100));
  // TODO(sheet): ui/sheet should read --text-zoom once it has a zoom hook (another task owns it).
  return () => document.documentElement.style.removeProperty("--text-zoom");
});

async function keepCaret() {
  await tick();
  const view = session.view;
  if (!view) return;
  view.requestMeasure();
  view.dispatch({
    effects: EditorView.scrollIntoView(view.state.selection.main.head, { y: "nearest" }),
  });
}

function onKey(e: KeyboardEvent) {
  // The sheet keeps its own zoom: what it already handled is left alone.
  if (e.defaultPrevented) return;
  if (!(e.ctrlKey || e.metaKey) || e.altKey) return;
  const p = workspace.project;
  if (e.key === "=" || e.key === "+") zoom.in(p);
  else if (e.key === "-" || e.key === "_") zoom.out(p);
  else if (e.key === "0") zoom.reset(p);
  else return;
  e.preventDefault();
  void keepCaret();
}

$effect(() => {
  const onWheel = (e: WheelEvent) => {
    if (!(e.ctrlKey || e.metaKey)) return;
    const target = e.target as HTMLElement | null;
    if (!target?.closest(".cm-editor, [data-zoomable]")) return;
    e.preventDefault();
    if (e.deltaY < 0) zoom.in(workspace.project);
    else if (e.deltaY > 0) zoom.out(workspace.project);
    void keepCaret();
  };
  window.addEventListener("wheel", onWheel, { passive: false });
  return () => window.removeEventListener("wheel", onWheel);
});

const sinceText = $derived(history.sinceAt ? stamp(history.sinceAt, false, "") : "");
</script>

<svelte:window onkeydown={onKey} />

{#if workspace.variant}
  <Banner role="status">
    {t("history.variant.editing", { name: history.variantName })}
    {#snippet action()}
      <button type="button" class="btn" onclick={() => history.backToMain()}>{t("history.variant.back")}</button>
    {/snippet}
  </Banner>
{:else if count > 0}
  <div class="strip" data-testid="change-strip">
    <span class="since">{t("history.since", { when: sinceText })}</span>
    <span class="legend" aria-label={t("history.legend")}>
      {#if history.colorBy === "type"}
        <span><i class="sq added"></i>{t("history.mark.added")}</span>
        <span><i class="sq modified"></i>{t("history.mark.modified")}</span>
        <span><i class="tri"></i>{t("history.mark.deleted")}</span>
      {:else}
        <span>{tn("history.marks", count)}</span>
      {/if}
    </span>
    <span class="sp"></span>
    <div class="tg" role="radiogroup" aria-label={t("history.colorby")}>
      <button type="button" role="radio" aria-checked={history.colorBy === "type"} onclick={() => (history.colorBy = "type")}>{t("history.colorby.type")}</button>
      <button type="button" role="radio" aria-checked={history.colorBy === "person"} onclick={() => (history.colorBy = "person")}>{t("history.colorby.person")}</button>
    </div>
    <button type="button" class="btn" onclick={() => history.markSeen()}>{t("history.seen")}</button>
  </div>
{/if}

<style>
  .strip {
    box-sizing: border-box;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px 12px;
    min-height: 45px;
    padding: 4px 12px;
    background: var(--headerbar-bg);
    box-shadow: 0 1px var(--shade);
    color: var(--fg-dim);
    font-size: 13px;
  }
  .legend {
    display: inline-flex;
    gap: 12px;
  }
  .legend span {
    display: inline-flex;
    align-items: center;
    gap: 5px;
  }
  .sp {
    flex: 1;
  }
  .strip :global(.btn) {
    min-height: 28px;
    padding: 0 10px;
    font-weight: 600;
    white-space: nowrap;
  }
  .sq {
    width: 9px;
    height: 9px;
    border-radius: 2px;
  }
  .added {
    background: var(--success);
  }
  .modified {
    background: var(--accent-bg);
  }
  .tri {
    border-top: 5px solid transparent;
    border-bottom: 5px solid transparent;
    border-left: 8px solid var(--destructive);
  }
</style>
