<script lang="ts">
import type { EditorState } from "@codemirror/state";
import type { Lang } from "../../editor/format";
import { cursorPosition } from "../../editor/stats/position";
import { stats as computeStats, type Stats } from "../../editor/stats/stats";
import { t, tn } from "../../i18n";
import { preview } from "../../preview/preview.svelte";
import type { Diagnostic } from "../../preview/protocol";
import type { FileInfo } from "../../store/files";
import type { Status } from "../../workspace/workspace.svelte";
import Icon from "../icons/Icon.svelte";
import EncodingMenu from "../popover/EncodingMenu.svelte";
import MenuButton from "../popover/MenuButton.svelte";
import StatsPopover from "../popover/StatsPopover.svelte";
import ZoomMenu from "../popover/ZoomMenu.svelte";
import { count } from "./count";
import { whenIdle } from "./idle";

/**
 * People and sync on the left; on the right the problems of the document (the list opens from
 * them), the page shown in the preview, the caret position, the word count (statistics open
 * from it), the file's encoding when it is unusual, and its language.
 */
let {
  lang,
  status,
  peerCount,
  editorState,
  info,
  showPage = false,
  onProblem = () => {},
  unsaved = false,
}: {
  lang: Lang;
  status: Status;
  /** People connected besides this one. */
  peerCount: number;
  editorState: EditorState | undefined;
  info: FileInfo | undefined;
  /** Says "Page X sur Y": true while the preview is shown. */
  showPage?: boolean;
  /** The person chose a problem in the list: go to its place. */
  onProblem?: (d: Diagnostic) => void;
  /** The text is not in the file yet (saving with Ctrl+S, or a write that failed). */
  unsaved?: boolean;
} = $props();

let statsOpen = $state(false);
let problemsOpen = $state(false);
let computed = $state.raw<Stats | undefined>();

const position = $derived(editorState ? cursorPosition(editorState) : { line: 1, column: 1 });
const words = $derived(computed ? count(computed.words, "mot") : "…");
const language = $derived(t(`lang.${lang}`));
const problemsName = $derived(
  preview.errors + preview.warnings === 0
    ? t("problems.none")
    : [
        preview.errors > 0 ? tn("problems.errors", preview.errors) : "",
        preview.warnings > 0 ? tn("problems.warnings", preview.warnings) : "",
      ]
        .filter(Boolean)
        .join(", "),
);

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
  {#if preview.hasResult}
    <MenuButton
      bind:open={problemsOpen}
      up
      align="end"
      width={460}
      label={t("problems.label")}
      name={`${t("problems.label")} : ${problemsName}`}
    >
      {#snippet trigger()}
        <span class="problems" data-testid="problems" data-errors={preview.errors} data-warnings={preview.warnings}>
          {#if preview.errors + preview.warnings === 0}
            <Icon name="check" size={13} /><span class="ptext">{t("problems.none")}</span>
          {:else}
            {#if preview.errors > 0}<span class="n err"><Icon name="fail" size={13} />{preview.errors}</span>{/if}
            {#if preview.warnings > 0}<span class="n warn"><Icon name="warn" size={13} />{preview.warnings}</span>{/if}
          {/if}
        </span>
      {/snippet}
      {#snippet children({ close })}
        {#if preview.diagnostics.length === 0}
          <p class="none">{t("problems.empty")}</p>
        {/if}
        {#each preview.diagnostics as d, i (i)}
          <button
            type="button"
            class="mi two problem"
            role="menuitem"
            data-severity={d.severity}
            disabled={d.line === undefined}
            onclick={() => {
              close(false);
              onProblem(d);
            }}
          >
            <Icon name={d.severity === "error" ? "fail" : "warn"} size={14} />
            <span class="text">
              {d.message.split("\n")[0]}
              {#if d.line !== undefined}
                <span class="sub">{t("problems.where", { file: d.file ?? "", line: d.line })}</span>
              {/if}
            </span>
          </button>
        {/each}
        {#if preview.truncated}<p class="none">{t("problems.more")}</p>{/if}
      {/snippet}
    </MenuButton>
  {/if}
  {#if showPage && preview.pageCount > 0}
    <span class="fb page" data-testid="page-indicator">
      {t("status.page", { page: Math.max(1, preview.currentPage), total: preview.pageCount })}
    </span>
  {/if}
  {#if unsaved}
    <span class="fb unsaved" data-testid="unsaved" title={t("save.unsaved.hint")}>{t("save.unsaved")}</span>
  {/if}
  <span class="fb pos" data-testid="position">{t("status.position", { line: position.line, column: position.column })}</span>
  <MenuButton bind:open={statsOpen} role="dialog" up align="end" label={t("stats.title")}>
    {#snippet trigger()}<span data-testid="words">{words}</span>{/snippet}
    {#snippet children()}
      {#if computed}
        <StatsPopover stats={computed} />
      {/if}
    {/snippet}
  </MenuButton>
  <ZoomMenu />
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
  .problems {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    font-size: 12.5px;
  }
  /* In a narrow window the tick says it: the words come back with the room. */
  @media (max-width: 900px) {
    .ptext {
      display: none;
    }
  }
  .problems .n {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-variant-numeric: tabular-nums;
  }
  .problems .err {
    color: var(--error);
  }
  .problems .warn {
    color: var(--warning);
  }
  .problem {
    gap: 8px;
  }
  .problem[data-severity="error"] :global(svg) {
    color: var(--error);
  }
  .problem[data-severity="warning"] :global(svg) {
    color: var(--warning);
  }
  .none {
    margin: 8px 12px;
    color: var(--fg-dim);
  }
</style>
