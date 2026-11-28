<script lang="ts">
import type { EditorState } from "@codemirror/state";
import { activeBlock, formatsOf, hintOf, type Lang } from "../../editor/format";
import { menuOf, nameOf } from "../../extensions/commands";
import { type MessageKey, t } from "../../i18n";
import Icon from "../icons/Icon.svelte";
import MenuButton from "../popover/MenuButton.svelte";
import CommandMenuItem from "./CommandMenuItem.svelte";

/**
 * The paragraph style at the caret, and the menu that sets it: the `format/heading` location
 * (`format.paragraph`, then the levels). Each level shows the syntax its language writes.
 */
let {
  lang,
  editorState,
  target,
}: {
  lang: Lang;
  editorState: EditorState;
  /** The editor the commands act on. */
  target?: Element | undefined;
} = $props();

let open = $state(false);

const groups = $derived(menuOf("format/heading", { formats: formatsOf(lang) }));
const levels = $derived(groups.find((g) => g.group === "headings")?.items ?? []);
const block = $derived(activeBlock(editorState, lang));
const level = $derived(levels.find((c) => nameOf(c.id) === block));
const current = $derived(
  level ? t(`heading.${nameOf(level.id)}` as MessageKey) : t("heading.paragraph"),
);
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
    {#each groups as g, gi (g.group)}
      {#if gi > 0}<div class="sep"></div>{/if}
      {#each g.items as c, i (c.id)}
        {@const heading = g.group === "headings"}
        <CommandMenuItem
          id={c.id}
          {close}
          {target}
          role="menuitemradio"
          checked={heading ? level === c : !level}
          class={heading ? `lvl lvl-${i + 1}` : "para"}
          detail={heading ? hintOf(lang, nameOf(c.id)) : t("heading.paragraph.syntax")}
        />
      {/each}
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
