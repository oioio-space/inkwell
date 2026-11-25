<script lang="ts">
import { labelOf, menuOf } from "../../extensions/commands";
import { locationOf } from "../../extensions/manifest";
import CommandButton from "./CommandButton.svelte";
import CommandIcon from "./CommandIcon.svelte";

/**
 * The buttons of a location (EX0c1), drawn by the host: its items whose `when` holds, by its
 * groups' fixed order, each as its group says (the command's icon, its label, or its label as
 * the suggested action). `groups` draws only some of them, so a bar puts its fixed widgets
 * between groups (the header bar's people and main menu). No box of its own.
 */
let {
  location,
  groups,
  target,
  labelClass = "fb lbl",
}: {
  location: string;
  groups?: readonly string[];
  /** The surface the commands act on, and whose state greys and hides them. */
  target?: Element | undefined;
  /** How a `label` item is drawn where it sits (the history strip's buttons are `btn`). */
  labelClass?: string;
} = $props();

const spec = $derived(locationOf(location));
const shown = $derived(
  menuOf(location, {}, target).filter((g) => groups === undefined || groups.includes(g.group)),
);
const look = (group: string) => spec?.groups.find((g) => g.id === group)?.look ?? "icon";
</script>

{#each shown as g (g.group)}
  {#each g.items as c (c.id)}
    {#if look(g.group) === "icon"}
      <CommandButton id={c.id} {target}><CommandIcon icon={c.icon} /></CommandButton>
    {:else}
      <CommandButton id={c.id} {target} class={look(g.group) === "suggested" ? "sug" : labelClass}>{labelOf(c)}</CommandButton>
    {/if}
  {/each}
{/each}
