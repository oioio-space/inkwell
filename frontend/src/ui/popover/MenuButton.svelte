<script lang="ts">
import { type Snippet, tick } from "svelte";
import { session } from "../editorSession.svelte";

/**
 * A button that opens an Adwaita popover (arrow, radius 12). Keyboard: Down/Up/Home/End move
 * between the items (`role="menuitem*"`), Escape closes and gives the editor its focus back,
 * Tab leaves. The panel opens below the button, or above it (`up`) for the status bar.
 */
let {
  label,
  name,
  open = $bindable(false),
  role = "menu",
  up = false,
  align = "start",
  width,
  triggerClass = "",
  disabled = false,
  trigger,
  children,
  onclose,
  tooltip,
}: {
  /** Accessible name of the panel. */
  label: string;
  /** Accessible name of the button when its text is not enough (an icon, or a state to add). */
  name?: string;
  open?: boolean;
  role?: "menu" | "dialog";
  up?: boolean;
  align?: "start" | "end";
  width?: number;
  triggerClass?: string;
  disabled?: boolean;
  trigger: Snippet;
  children: Snippet<[{ close: (refocus?: boolean) => void }]>;
  onclose?: () => void;
  tooltip?: Snippet;
} = $props();

let button: HTMLButtonElement | undefined = $state();
let panel: HTMLDivElement | undefined = $state();
let ax = $state(20);
let shift = $state(0);
let maxHeight = $state(0);
/** Opened with the keyboard: focus the first item (a mouse opening keeps the ring off). */
let byKeyboard = false;

function close(refocus = true) {
  if (!open) return;
  open = false;
  onclose?.();
  if (refocus) session.focus();
}

const items = () =>
  panel
    ? [
        ...panel.querySelectorAll<HTMLElement>(
          '[role^="menuitem"]:not([disabled]):not([aria-disabled="true"])',
        ),
      ]
    : [];

// On open: place the arrow under the button, keep the panel inside the window, move focus in.
$effect(() => {
  if (!open || !panel || !button) return;
  const host = button.getBoundingClientRect();
  const box = panel.getBoundingClientRect();
  ax = Math.max(12, host.width / 2 - 7);
  const margin = 8;
  const left = align === "start" ? host.left : host.right - box.width;
  const right = left + box.width;
  const over = right - (window.innerWidth - margin);
  if (over > 0) shift = over;
  else if (left < margin) shift = left - margin;
  else shift = 0;
  maxHeight = Math.max(
    160,
    up ? host.top - margin - 16 : window.innerHeight - host.bottom - margin - 16,
  );
  void tick().then(() => {
    const first = panel?.querySelector<HTMLElement>('[role^="menuitem"][aria-checked="true"]');
    (byKeyboard ? (first ?? items()[0] ?? panel) : panel)?.focus();
  });
});

function onKey(e: KeyboardEvent) {
  if (e.key === "Escape") {
    e.preventDefault();
    e.stopPropagation();
    close();
    return;
  }
  if (e.key === "Tab") {
    close(false);
    return;
  }
  const list = items();
  if (list.length === 0) return;
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

function onTriggerKey(e: KeyboardEvent) {
  if (!open && (e.key === "ArrowDown" || e.key === "ArrowUp")) {
    e.preventDefault();
    byKeyboard = true;
    open = true;
  } else if (open && e.key === "Escape") {
    e.preventDefault();
    close();
  }
}

function outside(e: PointerEvent) {
  if (!open) return;
  const target = e.target as Node;
  if (!panel?.contains(target) && !button?.contains(target)) close(false);
}
</script>

<svelte:window onpointerdown={outside} />

<div class="anchor">
  <button
    bind:this={button}
    type="button"
    class={["fb", "lbl", triggerClass]}
    class:on={open}
    aria-haspopup={role}
    aria-expanded={open}
    aria-label={name}
    {disabled}
    onmousedown={(e) => e.preventDefault()}
    onclick={(e) => {
      byKeyboard = e.detail === 0;
      if (open) close();
      else open = true;
    }}
    onkeydown={onTriggerKey}
  >
    {@render trigger()}
  </button>
  {#if tooltip && !open}
    {@render tooltip()}
  {/if}
  {#if open}
    <div
      bind:this={panel}
      class={["pop", up ? "above" : "below", align]}
      {role}
      aria-label={label}
      tabindex="-1"
      style:width={width ? `${width}px` : undefined}
      style:--ax="{ax}px"
      style:--shift="{shift}px"
      onkeydown={onKey}
    >
      <div class="pop-scroll" style:max-height="{maxHeight}px">
        {@render children({ close })}
      </div>
    </div>
  {/if}
</div>

<style>
  .anchor {
    position: relative;
    display: inline-flex;
  }
</style>
