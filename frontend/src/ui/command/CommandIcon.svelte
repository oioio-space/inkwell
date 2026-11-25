<script lang="ts">
import { glyphOf } from "../icons/glyphs";
import Icon from "../icons/Icon.svelte";
import type { IconName } from "../icons/paths";

/**
 * A command's icon: a path of the set, or a typeset glyph (`glyph-bold` is a bold B), larger in
 * a bar than before a menu item's label (`lead`).
 */
let { icon, lead = false }: { icon: string | undefined; lead?: boolean } = $props();
const glyph = $derived(glyphOf(icon));
</script>

{#if glyph}
  <span class={["glyph", lead ? "lead" : glyph.style]} aria-hidden="true">{glyph.letter}</span>
{:else if icon}
  <Icon name={icon as IconName} />
{/if}

<style>
  .glyph {
    font: 17px / 1 var(--font-serif);
  }
  .glyph.lead {
    display: inline-grid;
    place-items: center;
    width: 16px;
    font-size: 15px;
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
