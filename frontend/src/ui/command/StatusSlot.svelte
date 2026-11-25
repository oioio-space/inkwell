<script lang="ts">
import type { StatusItem } from "../../extensions/registry";
import { componentOf, groupOf, type StatusBarProps } from "./status.svelte";

/**
 * One status item where its group puts it: its component once its entry is there. The wrapper
 * adds no box (`display: contents`); it carries the group's narrow-width rule for the bar's CSS.
 */
let { item, bar }: { item: StatusItem; bar: StatusBarProps } = $props();

const Item = $derived(componentOf(item));
const narrow = $derived(groupOf(item)?.narrow);
</script>

<span class="slot" data-status-item={item.id} data-narrow={narrow}>
  {#if Item}<Item {item} {bar} />{/if}
</span>

<style>
  .slot {
    display: contents;
  }
</style>
