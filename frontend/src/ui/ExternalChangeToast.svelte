<script lang="ts">
import { revealExternal } from "../editor/external";
import { t, tn } from "../i18n";
import { workspace } from "../workspace/workspace.svelte";
import { session } from "./editorSession.svelte";
import { sessions } from "./sheet/sessions";
import { showsSheet } from "./sheet/viewMode.svelte";
import Toast from "./Toast.svelte";

/** How long the "taken in" toast stays: the lines remain marked, and Ctrl+Z still takes it back. */
const SHOW_MS = 12_000;

let busy = $state(false);
const notice = $derived(workspace.notice);

async function resolve(action: () => Promise<void>) {
  busy = true;
  try {
    await action();
  } finally {
    busy = false;
  }
}

function view() {
  const first = notice?.kind === "merged" ? notice.ranges[0] : undefined;
  const sheet = workspace.handle ? sessions.get(workspace.handle) : undefined;
  if (showsSheet(workspace.current) && sheet && first) sheet.placeAtOffset(first.from);
  else if (session.view) revealExternal(session.view);
}

// The toast that reports a done thing goes away by itself; one that asks waits for an answer.
$effect(() => {
  if (notice?.kind !== "merged") return;
  const timer = setTimeout(() => workspace.dismissNotice(), SHOW_MS);
  return () => clearTimeout(timer);
});
</script>

{#if notice?.kind === "merged"}
  <Toast role="status" onclose={() => workspace.dismissNotice()}>
    {notice.lines > 0 ? tn("external.merged", notice.lines) : t("external.locked")}
    {#snippet actions()}
      {#if notice.ranges.length > 0}
        <button type="button" class="fb lbl" onclick={view}>{t("external.view")}</button>
        <button type="button" class="fb lbl" onclick={() => workspace.undoExternal()}>
          {t("external.undo")}
        </button>
      {/if}
    {/snippet}
  </Toast>
{:else if notice?.kind === "ask"}
  <Toast role="alert">
    {t("external.message", { name: notice.name })}
    {#snippet actions()}
      <button type="button" class="fb lbl" disabled={busy} onclick={() => resolve(() => workspace.integrateExternal())}>
        {t("external.integrate")}
      </button>
      <button type="button" class="fb lbl" disabled={busy} onclick={() => resolve(() => workspace.keepOurs())}>
        {t("external.keep")}
      </button>
    {/snippet}
  </Toast>
{/if}
