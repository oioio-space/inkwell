<script lang="ts">
import type { EditorState } from "@codemirror/state";
import type { Lang } from "../../editor/format";
import { cursorPosition } from "../../editor/stats/position";
import { stats as computeStats, type Stats } from "../../editor/stats/stats";
import { t, tn } from "../../i18n";
import type { FileInfo } from "../../store/files";
import type { Status } from "../../workspace/workspace.svelte";
import EncodingMenu from "../popover/EncodingMenu.svelte";
import MenuButton from "../popover/MenuButton.svelte";
import StatsPopover from "../popover/StatsPopover.svelte";
import { count } from "./count";
import { whenIdle } from "./idle";

/**
 * People and sync on the left; on the right the caret position, the word count (statistics
 * open from it), the file's encoding when it is unusual, and its language.
 */
let {
  lang,
  status,
  peerCount,
  editorState,
  info,
}: {
  lang: Lang;
  status: Status;
  /** People connected besides this one. */
  peerCount: number;
  editorState: EditorState | undefined;
  info: FileInfo | undefined;
} = $props();

let statsOpen = $state(false);
let computed = $state.raw<Stats | undefined>();

const position = $derived(editorState ? cursorPosition(editorState) : { line: 1, column: 1 });
const words = $derived(computed ? count(computed.words, "mot") : "…");
const language = $derived(t(`lang.${lang}`));

// Statistics are computed when the browser is idle, from the (immutable) state they started at.
$effect(() => {
  const s = editorState;
  if (!s) return;
  return whenIdle(() => {
    const { from, to } = s.selection.main;
    computed = computeStats(s.doc.toString(), lang, { from, to });
  });
});
</script>

<footer class="bot" aria-label={t("status.bar")}>
  <span class="people" data-status={status}>
    <span class="dot" aria-hidden="true"></span>
    {peerCount === 0 ? t("status.alone") : tn("people.count", peerCount + 1)}
  </span>
  <span class="sp"></span>
  <span class="fb pos" data-testid="position">{t("status.position", { line: position.line, column: position.column })}</span>
  <MenuButton bind:open={statsOpen} role="dialog" up align="end" label={t("stats.title")}>
    {#snippet trigger()}<span data-testid="words">{words}</span>{/snippet}
    {#snippet children()}
      {#if computed}
        <StatsPopover stats={computed} />
      {/if}
    {/snippet}
  </MenuButton>
  <EncodingMenu {info} />
  <span class="fb lang" data-testid="language">{language}</span>
</footer>

<style>
  .bot {
    box-sizing: border-box;
    display: flex;
    align-items: center;
    gap: 2px;
    height: 36px;
    padding: 0 6px;
    background: var(--headerbar-bg);
    box-shadow: 0 -1px var(--shade);
    color: var(--fg-dim);
    font-size: 12.5px;
    position: relative;
    z-index: 2;
  }
  .bot :global(.fb) {
    height: 28px;
    padding: 0 10px;
    color: var(--fg);
    font-variant-numeric: tabular-nums;
    opacity: 0.8;
  }
  .bot :global(.fb.encoding) {
    opacity: 1;
  }
  .sp {
    flex: 1;
  }
  .people {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 0 10px;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--fg-dim);
  }
  [data-status="synced"] .dot {
    background: var(--success);
  }
</style>
