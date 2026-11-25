<script lang="ts">
import type { Block, Lang } from "../../editor/format";
import { t } from "../../i18n";
import CommandMenuItem from "../command/CommandMenuItem.svelte";
import Icon from "../icons/Icon.svelte";
import MenuButton from "../popover/MenuButton.svelte";
import { HEADING_SYNTAX, HEADINGS } from "./spec";

/** The paragraph style at the caret, and the menu that sets it (`format.paragraph`, `format.h1`…). */
let {
  lang,
  block,
  target,
}: {
  lang: Exclude<Lang, "plain">;
  /** The block type at the caret: the menu shows the level it is at. */
  block: Block;
  /** The editor the commands act on. */
  target?: Element | undefined;
} = $props();

let open = $state(false);

const level = $derived(HEADINGS.find((h) => h === block));
const current = $derived(level ? t(`heading.${level}`) : t("heading.paragraph"));
</script>

<MenuButton
  bind:open
  label={t("heading.menu")}
  name={`${t("heading.menu")} : ${current}`}
  triggerClass="heading"
  width={300}
  align="start"
>
  {#snippet trigger()}
    <span class="current" data-testid="heading-current">{current}</span>
    <Icon name="down" size={12} />
  {/snippet}
  {#snippet children({ close })}
    <CommandMenuItem
      id="format.paragraph"
      {close}
      {target}
      role="menuitemradio"
      checked={!level}
      class="para"
      detail={t("heading.paragraph.syntax")}
    />
    <div class="sep"></div>
    {#each HEADINGS as h, i (h)}
      <CommandMenuItem
        id="format.{h}"
        {close}
        {target}
        role="menuitemradio"
        checked={level === h}
        class="lvl lvl-{i + 1}"
        detail={HEADING_SYNTAX[lang][h]}
      />
    {/each}
  {/snippet}
</MenuButton>

<style>
  :global(.fb.heading) {
    width: 132px;
    justify-content: space-between;
  }
  :global(.mi.lvl) {
    font-weight: 700;
    min-height: 34px;
  }
  :global(.mi.lvl-1) {
    font-size: 18px;
    min-height: 38px;
  }
  :global(.mi.lvl-2) {
    font-size: 16px;
  }
  :global(.mi.lvl-3) {
    font-size: 15px;
  }
  :global(.mi.lvl .k) {
    font-weight: 400;
  }
  :global(.mi.para .ico),
  :global(.mi.lvl .ico) {
    color: var(--accent);
  }
</style>
