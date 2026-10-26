<script lang="ts">
import { tick } from "svelte";
import Icon from "../icons/Icon.svelte";
import type { MenuItem } from "./menu";

/** A context menu at the pointer: Up/Down move between items, Enter runs, Escape closes. */
let {
  x,
  y,
  items,
  label,
  onclose,
}: {
  x: number;
  y: number;
  items: MenuItem[];
  label: string;
  onclose: () => void;
} = $props();

let panel: HTMLDivElement | undefined = $state();
let left = $state(0);
let top = $state(0);

$effect(() => {
  if (!panel) return;
  const box = panel.getBoundingClientRect();
  left = Math.max(8, Math.min(x, window.innerWidth - box.width - 8));
  top = Math.max(8, Math.min(y, window.innerHeight - box.height - 8));
  void tick().then(() =>
    panel?.querySelector<HTMLElement>('[role="menuitem"]:not([disabled])')?.focus(),
  );
});

const live = () => [
  ...(panel?.querySelectorAll<HTMLElement>('[role="menuitem"]:not([disabled])') ?? []),
];

function key(e: KeyboardEvent) {
  const list = live();
  const at = list.indexOf(document.activeElement as HTMLElement);
  if (e.key === "Escape" || e.key === "Tab") {
    e.preventDefault();
    onclose();
  } else if (e.key === "ArrowDown") {
    e.preventDefault();
    list[(at + 1) % list.length]?.focus();
  } else if (e.key === "ArrowUp") {
    e.preventDefault();
    list[(at <= 0 ? list.length : at) - 1]?.focus();
  } else if (e.key === "Home") {
    e.preventDefault();
    list[0]?.focus();
  } else if (e.key === "End") {
    e.preventDefault();
    list[list.length - 1]?.focus();
  }
}

function outside(e: PointerEvent) {
  if (!panel?.contains(e.target as Node)) onclose();
}
</script>

<svelte:window onpointerdown={outside} />

<div
  bind:this={panel}
  class="pop ctx"
  role="menu"
  aria-label={label}
  tabindex="-1"
  style:left="{left}px"
  style:top="{top}px"
  onkeydown={key}
>
  {#each items as item, i (i)}
    {#if item.separator}
      <div class="sep" role="separator"></div>
    {:else}
      <button
        type="button"
        role="menuitem"
        class="mi"
        disabled={item.disabled}
        onclick={() => {
          onclose();
          item.run?.();
        }}
      >
        {#if item.icon}<Icon name={item.icon} />{:else}<span class="blank"></span>{/if}
        <span>{item.label}</span>
        {#if item.shortcut}<span class="k">{item.shortcut}</span>{/if}
      </button>
    {/if}
  {/each}
</div>

<style>
  .ctx {
    position: fixed;
    z-index: 60;
    min-width: 260px;
  }
  .ctx::before {
    display: none;
  }
  .blank {
    width: 16px;
    flex: none;
  }
</style>
