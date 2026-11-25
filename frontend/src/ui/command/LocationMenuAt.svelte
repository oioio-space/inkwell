<script lang="ts">
import { tick } from "svelte";
import { menuOf } from "../../extensions/commands";
import type { Command } from "../../extensions/registry";
import { t } from "../../i18n";
import Icon from "../icons/Icon.svelte";
import CommandIcon from "./CommandIcon.svelte";
import CommandMenuItem from "./CommandMenuItem.svelte";

/**
 * A context menu drawn from a location at a point (EX0c2, the grid's; the six reserved ones of
 * UX1-D reuse it): its items whose `when` holds on the target surface, a separator between groups,
 * each its command's (name, chord, a toggle checked, greyed with its reason). Keys as the WAI-ARIA
 * menu pattern: the first item takes the focus, Up and Down wrap, Home and End, Enter and Space
 * run, Escape and Tab close it and `onclose` gives the focus back. Past {@link SUBMENU_AFTER}
 * items, the extensions' group is one « Extensions » item opening them as a submenu (Right or
 * Enter in, Left or Escape out), in the same order.
 */
let {
  location,
  x,
  y,
  label,
  target,
  onclose,
}: {
  location: string;
  x: number;
  y: number;
  label: string;
  target: Element | undefined;
  onclose: () => void;
} = $props();

const SUBMENU_AFTER = 8;

const groups = $derived(menuOf(location, {}, target));
const core = $derived(groups.filter((g) => g.group !== "extensions"));
const extensions = $derived(groups.find((g) => g.group === "extensions")?.items ?? []);
const nested = $derived(extensions.length > SUBMENU_AFTER);

let panel: HTMLDivElement | undefined = $state();
let sub: HTMLDivElement | undefined = $state();
let subOpen = $state(false);
let left = $state(0);
let top = $state(0);

$effect(() => {
  if (!panel) return;
  const box = panel.getBoundingClientRect();
  left = Math.max(8, Math.min(x, window.innerWidth - box.width - 8));
  top = Math.max(8, Math.min(y, window.innerHeight - box.height - 8));
  void tick().then(() => items(panel)[0]?.focus());
});

const items = (menu: HTMLElement | undefined) =>
  [...(menu?.querySelectorAll<HTMLElement>(":scope > [role^=menuitem]") ?? [])].filter(
    (el) => !el.hasAttribute("disabled"),
  );

function openSub() {
  subOpen = true;
  void tick().then(() => items(sub)[0]?.focus());
}

function closeSub() {
  subOpen = false;
  panel?.querySelector<HTMLElement>("[aria-haspopup]")?.focus();
}

function key(e: KeyboardEvent) {
  const inSub = !!sub && sub.contains(document.activeElement);
  const list = items(inSub ? sub : panel);
  const at = list.indexOf(document.activeElement as HTMLElement);
  const move = (i: number) => {
    e.preventDefault();
    list[i]?.focus();
  };
  if (e.key === "Escape" && inSub) {
    e.preventDefault();
    closeSub();
  } else if (e.key === "Escape" || e.key === "Tab") {
    e.preventDefault();
    onclose();
  } else if (e.key === "ArrowDown") move((at + 1) % list.length);
  else if (e.key === "ArrowUp") move((at <= 0 ? list.length : at) - 1);
  else if (e.key === "Home") move(0);
  else if (e.key === "End") move(list.length - 1);
  else if (
    e.key === "ArrowRight" &&
    !inSub &&
    document.activeElement?.hasAttribute("aria-haspopup")
  ) {
    e.preventDefault();
    openSub();
  } else if (e.key === "ArrowLeft" && inSub) {
    e.preventDefault();
    closeSub();
  }
}

function outside(e: PointerEvent) {
  if (!panel?.contains(e.target as Node)) onclose();
}
</script>

<!-- A menu item closes the menu, then its command runs on the target; its icon, or a blank of its width. -->
{#snippet item(c: Command)}
  <CommandMenuItem id={c.id} close={onclose} {target} refocus={false}>
    {#snippet lead()}{#if c.icon}<CommandIcon icon={c.icon} lead />{:else}<span class="blank"></span>{/if}{/snippet}
  </CommandMenuItem>
{/snippet}

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
  {#each core as g, gi (g.group)}
    {#if gi > 0}<div class="sep" role="separator"></div>{/if}
    {#each g.items as c (c.id)}{@render item(c)}{/each}
  {/each}
  {#if extensions.length > 0}
    <div class="sep" role="separator"></div>
    {#if nested}
      <button
        type="button"
        class="mi"
        role="menuitem"
        aria-haspopup="menu"
        aria-expanded={subOpen}
        data-fixed="extensions-submenu"
        onclick={() => (subOpen ? closeSub() : openSub())}
      >
        <span class="blank"></span>
        <span>{t("menu.extensions")}</span>
        <span class="chev"><Icon name="right" size={14} /></span>
      </button>
      {#if subOpen}
        <div bind:this={sub} class="pop ctx sub" role="menu" aria-label={t("menu.extensions")} tabindex="-1">
          {#each extensions as c (c.id)}{@render item(c)}{/each}
        </div>
      {/if}
    {:else}
      {#each extensions as c (c.id)}{@render item(c)}{/each}
    {/if}
  {/if}
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
  .sub {
    position: absolute;
    left: 100%;
    top: auto;
    margin-top: -40px;
  }
  .blank {
    width: 16px;
    flex: none;
  }
  .chev {
    margin-inline-start: auto;
    display: inline-flex;
  }
</style>
