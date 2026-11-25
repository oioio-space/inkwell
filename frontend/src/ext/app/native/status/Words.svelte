<script lang="ts">
import { stats as computeStats, type Stats } from "../../../../editor/stats/stats";
import { t } from "../../../../i18n";
import type { StatusItemProps } from "../../../../ui/command/status.svelte";
import MenuButton from "../../../../ui/popover/MenuButton.svelte";
import StatsPopover from "../../../../ui/popover/StatsPopover.svelte";
import { count } from "../../../../ui/shell/count";
import { whenIdle } from "../../../../ui/shell/idle";

/** The word count; the statistics open from it. Computed when the browser is idle. */
let { bar }: StatusItemProps = $props();

let open = $state(false);
let computed = $state.raw<Stats | undefined>();
const words = $derived(computed ? count(computed.words, "mot") : "…");

// From the (immutable) state they started at.
$effect(() => {
  const s = bar.editorState;
  const lang = bar.lang;
  if (!s) return;
  return whenIdle(() => {
    const { from, to } = s.selection.main;
    computed = computeStats(s.doc.toString(), lang, { from, to });
  });
});
</script>

<MenuButton bind:open role="dialog" up align="end" label={t("stats.title")}>
  {#snippet trigger()}<span data-testid="words">{words}</span>{/snippet}
  {#snippet children()}
    {#if computed}
      <StatsPopover stats={computed} />
    {/if}
  {/snippet}
</MenuButton>
