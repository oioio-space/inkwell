<script lang="ts">
import Icon from "../icons/Icon.svelte";
import type { IconName } from "../icons/paths";
import type { BarId } from "./spec";

/**
 * A flat button of the formatting bar. Its tooltip says what it does, the syntax it writes and
 * its shortcut ("Gras `*texte*` Ctrl+B"); `pressed` is only given by toggles.
 */
let {
  id,
  label,
  icon,
  glyph,
  syntax,
  shortcut,
  pressed,
  onclick,
}: {
  id: BarId;
  label: string;
  icon?: IconName | undefined;
  glyph?: string | undefined;
  syntax?: string | undefined;
  shortcut?: string | undefined;
  pressed?: boolean | undefined;
  onclick: () => void;
} = $props();

const tipId = $derived(`tip-${id}`);
</script>

<span class="tipwrap">
  <button
    type="button"
    class="fb"
    data-bar={id}
    aria-label={label}
    aria-pressed={pressed}
    aria-describedby={tipId}
    onmousedown={(e) => e.preventDefault()}
    {onclick}
  >
    {#if icon}
      <Icon name={icon} />
    {:else}
      <span class="glyph g-{id}" aria-hidden="true">{glyph}</span>
    {/if}
  </button>
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
