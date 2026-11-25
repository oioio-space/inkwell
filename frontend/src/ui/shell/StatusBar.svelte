<script lang="ts">
import type { EditorState } from "@codemirror/state";
import type { Lang } from "../../editor/format";
import { statusOf } from "../../extensions/commands";
import { t } from "../../i18n";
import type { Diagnostic } from "../../preview/protocol";
import type { FileInfo } from "../../store/files";
import type { Status } from "../../workspace/workspace.svelte";
import StatusMore from "../command/StatusMore.svelte";
import StatusSlot from "../command/StatusSlot.svelte";
import { type StatusBarProps, statusLayout } from "../command/status.svelte";

/**
 * The status bar: the `status` point of the registry (EX0c1). Who is here on the left; on the
 * right, by their groups' fixed order, the problems, the page of the preview, the unsaved text,
 * the caret, the words, the zoom, the encoding and the language, then the extensions' items
 * (3 at most, the rest in ⋯). What shows is each item's `when` on the live context.
 */
let {
  lang,
  status,
  peerCount,
  editorState,
  info,
  ofMain = true,
  onProblem = () => {},
  problemsNav = false,
  unsaved = false,
}: {
  lang: Lang;
  status: Status;
  peerCount: number;
  editorState: EditorState | undefined;
  info: FileInfo | undefined;
  /** The open file is main.typ: only then do its problems apply (outside the main window). */
  ofMain?: boolean;
  onProblem?: (d: Diagnostic) => void;
  problemsNav?: boolean;
  unsaved?: boolean;
} = $props();

let width = $state(1200);
const bar: StatusBarProps = $derived({
  lang,
  status,
  peerCount,
  editorState,
  info,
  onProblem,
  problemsNav,
  unsaved,
  ofMain,
});
/** Squeezed by the side panels (640 px or less), the extensions' items go to ⋯ (L-4). */
const layout = $derived(statusLayout(statusOf(), width <= 640));
</script>

<footer class="bot" aria-label={t("status.bar")} bind:clientWidth={width}>
  {#each layout.start as item (item.id)}<StatusSlot {item} {bar} />{/each}
  <span class="sp"></span>
  {#each layout.end as item (item.id)}<StatusSlot {item} {bar} />{/each}
  {#if layout.more.length > 0}<StatusMore items={layout.more} />{/if}
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
    white-space: nowrap;
  }
  .bot :global(.fb) {
    height: 28px;
    padding: 0 10px;
    color: var(--fg);
    font-variant-numeric: tabular-nums;
    opacity: 0.8;
  }
  .bot :global(.fb.encoding),
  .bot :global(.fb.unsaved) {
    opacity: 1;
  }
  .bot :global(.fb.unsaved) {
    display: inline-flex;
    align-items: center;
    font-weight: 700;
  }
  .sp {
    flex: 1;
  }
  /* Each group's narrow-width rule (manifest.ts STATUS). In a narrow window the tick says it:
     the words come back with the room. */
  @media (max-width: 900px) {
    .bot :global([data-narrow="text"] .ptext) {
      display: none;
    }
  }
  /* Squeezed by the side panels, the bar keeps what matters: the rest would overlap. */
  @container (max-width: 640px) {
    .bot :global([data-narrow="hide"] > *) {
      display: none;
    }
  }
</style>
