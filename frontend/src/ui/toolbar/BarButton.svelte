<script lang="ts">
import { cmdAttrs, shortcutOf } from "../../extensions/commands";
import CommandButton from "../command/CommandButton.svelte";
import Icon from "../icons/Icon.svelte";
import type { IconName } from "../icons/paths";
import type { BarId } from "./spec";

/**
 * A flat button of the formatting bar, bound to its `format.*` command. Its tooltip says what it
 * does, the syntax it writes and this system's chord ("Gras `*texte*` Ctrl+B"); `pressed` is only
 * given by toggles.
 */
let {
  id,
  command,
  icon,
  glyph,
  syntax,
  pressed,
  target,
}: {
  id: BarId;
  command: string;
  icon?: IconName | undefined;
  glyph?: string | undefined;
  syntax?: string | undefined;
  pressed?: boolean | undefined;
  /** The editor the command acts on. */
  target?: Element | undefined;
} = $props();

const tipId = $derived(`tip-${id}`);
const shortcut = $derived(shortcutOf(command));
const label = $derived(cmdAttrs(command)["aria-label"] ?? command);
</script>

<span class="tipwrap">
  <CommandButton id={command} {target} data-bar={id} describedby={tipId} keepFocus pressed={pressed ?? undefined}>
    {#if icon}
      <Icon name={icon} />
    {:else}
      <span class="glyph g-{id}" aria-hidden="true">{glyph}</span>
    {/if}
  </CommandButton>
  <span class="tip" role="tooltip" id={tipId}>
    <b>{label}</b>
    {#if syntax}<code>{syntax}</code>{/if}
    {#if shortcut}<kbd class="kbd">{shortcut}</kbd>{/if}
  </span>
</span>

<style>
  .glyph {
    font: 17px / 1 var(--font-serif);
  }
  .g-bold {
    font-weight: 700;
  }
  .g-italic {
    font-style: italic;
  }
  .g-underline {
    text-decoration: underline;
  }
  .g-strike {
    text-decoration: line-through;
  }
  .g-formula {
    font-size: 16px;
  }
</style>
