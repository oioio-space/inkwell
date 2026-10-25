<script lang="ts">
import type { StateCommand } from "@codemirror/state";
import { type Block, type Lang, setBlock } from "../../editor/format";
import { t } from "../../i18n";
import Icon from "../icons/Icon.svelte";
import MenuButton from "../popover/MenuButton.svelte";
import { HEADING_SYNTAX, HEADINGS, shortcutLabel } from "./spec";

let {
  lang,
  block,
  run,
}: {
  lang: Exclude<Lang, "plain">;
  /** The block type at the caret: the menu shows the level it is at. */
  block: Block;
  run: (command: StateCommand) => boolean;
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
    <button
      type="button"
      class="mi"
      role="menuitemradio"
      aria-checked={!level}
      onclick={() => {
        run(setBlock(lang, "paragraph"));
        close();
      }}
    >
      <span>{t("heading.paragraph")}</span>
      {#if !level}<Icon name="check" />{/if}
      <span class="k mono">{t("heading.paragraph.syntax")} · {shortcutLabel("Mod-0")}</span>
    </button>
    <div class="sep"></div>
    {#each HEADINGS as h, i (h)}
      <button
        type="button"
        class="mi lvl lvl-{i + 1}"
        role="menuitemradio"
        aria-checked={level === h}
        onclick={() => {
          run(setBlock(lang, h));
          close();
        }}
      >
        <span>{t(`heading.${h}`)}</span>
        {#if level === h}<Icon name="check" />{/if}
        <span class="k mono">{HEADING_SYNTAX[lang][h]} · {shortcutLabel(`Mod-${i + 1}`)}</span>
      </button>
    {/each}
  {/snippet}
</MenuButton>

<style>
  :global(.fb.heading) {
    width: 132px;
    justify-content: space-between;
  }
  .lvl {
    font-weight: 700;
    min-height: 34px;
  }
  .lvl-1 {
    font-size: 18px;
    min-height: 38px;
  }
  .lvl-2 {
    font-size: 16px;
  }
  .lvl-3 {
    font-size: 15px;
  }
  .lvl .k {
    font-weight: 400;
  }
  .mi :global(.ico) {
    color: var(--accent);
  }
</style>
