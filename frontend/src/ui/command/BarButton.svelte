<script lang="ts">
import { cmdAttrs, shortcutOf } from "../../extensions/commands";
import type { Command } from "../../extensions/registry";
import CommandButton from "./CommandButton.svelte";
import CommandIcon from "./CommandIcon.svelte";

/**
 * A flat button of the formatting bar, bound to its command: its icon is the command's. Its
 * tooltip says what it does, the syntax it writes and this system's chord ("Gras `*texte*`
 * Ctrl+B"); `pressed` is only given by toggles.
 */
let {
  command,
  feature,
  hint,
  pressed,
  target,
}: {
  command: Command;
  /** The formatting feature it stands for (`bold`): its `data-bar`. */
  feature: string;
  hint?: string | undefined;
  pressed?: boolean | undefined;
  /** The editor the command acts on. */
  target?: Element | undefined;
} = $props();

const tipId = $derived(`tip-${feature}`);
const shortcut = $derived(shortcutOf(command.id));
const label = $derived(cmdAttrs(command.id)["aria-label"] ?? command.id);
</script>

<span class="tipwrap">
  <CommandButton id={command.id} {target} data-bar={feature} describedby={tipId} keepFocus pressed={pressed ?? undefined}>
    <CommandIcon icon={command.icon} />
  </CommandButton>
  <span class="tip" role="tooltip" id={tipId}>
    <b>{label}</b>
    {#if hint}<code>{hint}</code>{/if}
    {#if shortcut}<kbd class="kbd">{shortcut}</kbd>{/if}
  </span>
</span>
