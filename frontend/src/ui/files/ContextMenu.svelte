
<script lang="ts" module>
/** One entry of the menu; `separator` draws a line instead. */
export type MenuItem =
  | { separator: true }
  | {
      separator?: false;
      id: string;
      label: string;
      /** The keys that do the same, shown at the end. */
      keys?: string;
      disabled?: boolean;
      danger?: boolean;
      onchoose: () => void;
    };
</script>

<script lang="ts">
import { tick } from "svelte";

/**
 * The menu of a file: opens at the pointer (right click) or at the row (Shift+F10, the menu
 * key), moves with the arrows, closes with Escape or a click elsewhere and gives focus back
 * to what had it.
 */
let {
  x,
  y,
  label,
  items,
  onclose,
}: {
  x: number;
  y: number;
  label: string;
  items: readonly MenuItem[];
  onclose: () => void;
} = $props();

let panel: HTMLDivElement | undefined = $state();
let left = $state(0);
let top = $state(0);

const buttons = () =>
  panel ? [...panel.querySelectorAll<HTMLElement>('[role="menuitem"]:not([aria-disabled="true"])')] : [];

// Place it at the pointer, kept inside the window, and move focus in.
$effect(() => {
  if (!panel) return;
  const box = panel.getBoundingClientRect();
  const margin = 8;
  left = Math.max(margin, Math.min(x, window.innerWidth - box.width - margin));
  top = Math.max(margin, Math.min(y, window.innerHeight - box.height - margin));
  void tick().then(() => buttons()[0]?.focus());
});

function onKey(e: KeyboardEvent) {
  if (e.key === "Escape" || e.key === "Tab") {
    e.preventDefault();
    e.stopPropagation();
    onclose();
    return;
  }
  const list = buttons();
  const at = list.indexOf(document.activeElement as HTMLElement);
  let to = -1;
  if (e.key === "ArrowDown") to = at < 0 ? 0 : (at + 1) % list.length;
  else if (e.key === "ArrowUp") to = at <= 0 ? list.length - 1 : at - 1;
  else if (e.key === "Home") to = 0;
  else if (e.key === "End") to = list.length - 1;
  if (to >= 0) {
    e.preventDefault();
    list[to]?.focus();
  }
}

function outside(e: PointerEvent) {
  if (panel && !panel.contains(e.target as Node)) onclose();
}
</script>

<svelte:window onpointerdown={outside} />

<div
  bind:this={panel}
  class="menu"
  role="menu"
  aria-label={label}
  tabindex="-1"
  style:left="{left}px"
  style:top="{top}px"
  onkeydown={onKey}
>
  {#each items as item, i (item.separator ? `sep${i}` : item.id)}
    {#if item.separator}
      <div class="sep" role="separator"></div>
    {:else}
      <button
        type="button"
        class="mi"
        class:danger={item.danger}
        role="menuitem"
        aria-disabled={item.disabled ? "true" : undefined}
        onclick={() => {
          if (item.disabled) return;
          onclose();
          item.onchoose();
        }}
      >
        <span class="text">{item.label}</span>
        {#if item.keys}<span class="k">{item.keys}</span>{/if}
      </button>
    {/if}
  {/each}
</div>

<style>
  .menu {
    position: fixed;
    z-index: 60;
    box-sizing: border-box;
    min-width: 220px;
    max-width: calc(100vw - 16px);
    padding: 6px;
    border-radius: var(--radius);
    background: var(--popover-bg);
    color: var(--fg);
    box-shadow: var(--shadow-popover);
  }
  .menu:focus {
    outline: none;
  }
  .sep {
    height: 1px;
    margin: 4px 6px;
    background: var(--border);
  }
  .danger {
    color: var(--warning);
  }
</style>
