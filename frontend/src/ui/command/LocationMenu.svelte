<script lang="ts">
import type { Snippet } from "svelte";
import { menuOf } from "../../extensions/commands";
import MenuButton from "../popover/MenuButton.svelte";
import CommandMenuItem from "./CommandMenuItem.svelte";

/**
 * A menu drawn from a location (the main menu): its items whose `when` holds, a separator
 * between groups, each item its command's (label, chord, toggle, reason when greyed).
 */
let {
  location,
  label,
  name,
  align = "end",
  width = 300,
  trigger,
}: {
  location: string;
  label: string;
  name?: string;
  align?: "start" | "end";
  width?: number;
  trigger: Snippet;
} = $props();

let open = $state(false);
const groups = $derived(open ? menuOf(location) : []);
</script>

<MenuButton bind:open {label} name={name ?? label} {align} {width} {trigger}>
  {#snippet children({ close })}
    {#each groups as g, i (g.group)}
      {#if i > 0}<div class="sep" role="separator"></div>{/if}
      {#each g.items as c (c.id)}
        <CommandMenuItem id={c.id} {close} />
      {/each}
    {/each}
  {/snippet}
</MenuButton>
