<script lang="ts">
import type { Snippet } from "svelte";
import { labelOf, menuOf } from "../../extensions/commands";
import { locationOf } from "../../extensions/manifest";
import { t } from "../../i18n";
import Icon from "../icons/Icon.svelte";
import MenuButton from "../popover/MenuButton.svelte";
import { fitCount } from "../toolbar/layout";
import CommandButton from "./CommandButton.svelte";
import CommandIcon from "./CommandIcon.svelte";
import CommandMenuItem from "./CommandMenuItem.svelte";

/**
 * A toolbar drawn from a location (EX0c2, the grid's bar): its items whose `when` holds on the
 * target surface, a separator between groups, each its command's (icon, name, chord, pressed,
 * greyed with its reason). What does not fit goes to a "⋯" menu in the same order (`overflow:
 * "width"`), so nothing is lost. A press keeps the target's focus; after a click `after` says
 * where it goes. `end` holds the host's fixed widgets after the items.
 */
let {
  location,
  label,
  target,
  after,
  end,
}: {
  location: string;
  label: string;
  /** The surface the commands act on, and whose state greys and presses them. */
  target: Element | undefined;
  /** Called after an item ran, with its command (the grid takes its focus back). */
  after?: (id: string) => void;
  end?: Snippet;
} = $props();

/** The bar's buttons, as `.fb` and the separator draw them (theme/adwaita.css, `.vsep`). */
const METRICS = { button: 34, gap: 2, groupGap: 17 } as const;
const PADDING = 12;

let width = $state(4000);
let endWidth = $state(0);
let moreOpen = $state(false);

const spec = $derived(locationOf(location));
const items = $derived(
  menuOf(location, {}, target).flatMap((g, group) =>
    g.items.map((command) => ({ command, group, look: lookOf(g.group) })),
  ),
);
const count = $derived(
  spec?.overflow === "width"
    ? fitCount(
        items.map((i) => i.group),
        width - PADDING - endWidth,
        METRICS,
      )
    : items.length,
);
const shown = $derived(items.slice(0, count));
const hidden = $derived(items.slice(count));

function lookOf(group: string) {
  return spec?.groups.find((g) => g.id === group)?.look ?? "icon";
}
</script>

<div class="toolbar" role="toolbar" aria-label={label} bind:clientWidth={width}>
  {#each shown as item, i (item.command.id)}
    {#if i > 0 && shown[i - 1]?.group !== item.group}<span class="vsep" aria-hidden="true"></span>{/if}
    <CommandButton
      id={item.command.id}
      {target}
      keepFocus
      class={item.look === "icon" ? "fb" : "fb lbl"}
      onrun={() => after?.(item.command.id)}
    >
      {#if item.look === "icon"}<CommandIcon icon={item.command.icon} />{:else}{labelOf(item.command)}{/if}
    </CommandButton>
  {/each}
  {#if hidden.length > 0}
    <MenuButton bind:open={moreOpen} label={t("fmt.more")} name={t("fmt.more")} width={260} align="start">
      {#snippet trigger()}<Icon name="more" />{/snippet}
      {#snippet children({ close })}
        {#each hidden as item (item.command.id)}
          <!-- The menu closes without giving the text the focus: `after` says where it goes (the grid). -->
          <CommandMenuItem
            id={item.command.id}
            {close}
            {target}
            refocus={false}
            onrun={() => after?.(item.command.id)}
          >
            {#snippet lead()}<CommandIcon icon={item.command.icon} lead />{/snippet}
          </CommandMenuItem>
        {/each}
      {/snippet}
    </MenuButton>
  {/if}
  <span class="grow"></span>
  {#if end}<span class="end" bind:offsetWidth={endWidth}>{@render end()}</span>{/if}
</div>

<style>
  .toolbar {
    display: flex;
    align-items: center;
    gap: 2px;
    min-height: 40px;
    padding: 3px 6px;
    background: var(--headerbar-bg);
    box-shadow: 0 1px var(--shade);
    position: relative;
    z-index: 5;
  }
  .vsep {
    width: 1px;
    height: 20px;
    margin: 0 6px;
    background: var(--shade);
  }
  .grow {
    flex: 1;
  }
  .end {
    display: flex;
    align-items: center;
  }
</style>
