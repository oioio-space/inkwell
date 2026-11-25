<script lang="ts">
import type { EditorState, StateCommand } from "@codemirror/state";
import { activeBlock, activeMarks, type Lang } from "../../editor/format";
import { t } from "../../i18n";
import type { ProjectFile } from "../../store/files";
import { workspace } from "../../workspace/workspace.svelte";
import CommandMenuItem from "../command/CommandMenuItem.svelte";
import Icon from "../icons/Icon.svelte";
import MenuButton from "../popover/MenuButton.svelte";
import BarButton from "./BarButton.svelte";
import HeadingMenu from "./HeadingMenu.svelte";
import InsertMenu from "./InsertMenu.svelte";
import { visibleCount } from "./layout";
import { type BarItem, commandOf, itemsFor } from "./spec";

/**
 * The formatting bar of a file: what the format can write, as buttons that follow the caret
 * (aria-pressed), a heading menu, the Insert menu, and a "⋯" menu for what does not fit. Each
 * button and item runs its `format.*` command on the editor (`target`) through the registry.
 * Plain text files get no bar (the caller does not render it).
 */
let {
  lang,
  editorState,
  target,
  run,
  files,
  currentFile,
  readFile,
}: {
  lang: Exclude<Lang, "plain">;
  editorState: EditorState;
  /** The editor the commands act on (its surface). */
  target: Element | undefined;
  /** Runs an insertion of the Insert menu on the editor (EX0c moves it onto commands). */
  run: (command: StateCommand) => boolean;
  files: readonly ProjectFile[];
  currentFile: string;
  readFile: (path: string) => Promise<string | undefined>;
} = $props();

let width = $state(1200);
let moreOpen = $state(false);

const items = $derived(itemsFor(lang));
const count = $derived(
  visibleCount(
    items.map((i) => i.group),
    width,
  ),
);
const shown = $derived(items.slice(0, count));
const hidden = $derived(items.slice(count));
const marks = $derived(activeMarks(editorState, lang));
const block = $derived(activeBlock(editorState, lang));

function pressed(item: BarItem): boolean | undefined {
  if (!item.toggle) return undefined;
  return "mark" in item.toggle ? marks.has(item.toggle.mark) : block === item.toggle.block;
}

const languageName = $derived(t(`lang.${lang}`));
/** A reader, or a disconnected person: every button would do nothing, so none is offered. */
const locked = $derived(workspace.locked);
</script>

<div
  class="tb"
  role="toolbar"
  aria-label={t("fmt.bar", { language: languageName })}
  bind:clientWidth={width}
  inert={locked}
  class:locked
>
  <HeadingMenu {lang} {block} {target} />
  <span class="gap"></span>
  {#each shown as item, i (item.id)}
    {#if i > 0 && shown[i - 1]?.group !== item.group}<span class="gap"></span>{/if}
    <BarButton
      id={item.id}
      command={commandOf(item)}
      icon={item.icon}
      glyph={item.glyph}
      syntax={item.syntax[lang]}
      pressed={pressed(item)}
      {target}
    />
  {/each}
  {#if hidden.length > 0}
    <MenuButton bind:open={moreOpen} label={t("fmt.more")} name={t("fmt.more")} width={260} align="start">
      {#snippet trigger()}<Icon name="more" />{/snippet}
      {#snippet children({ close })}
        {#each hidden as item (item.id)}
          <CommandMenuItem id={commandOf(item)} {close} {target} checked={pressed(item)} refocus data-bar={item.id}>
            {#snippet lead()}
              {#if item.icon}<Icon name={item.icon} />{:else}<span class="glyph" aria-hidden="true">{item.glyph}</span>{/if}
            {/snippet}
          </CommandMenuItem>
        {/each}
      {/snippet}
    </MenuButton>
  {/if}
  <span class="gap"></span>
  <InsertMenu {lang} {editorState} {run} {files} {currentFile} {readFile} />
</div>

<style>
  .tb {
    box-sizing: border-box;
    display: flex;
    align-items: center;
    gap: 2px;
    height: 46px;
    padding: 0 8px;
    background: var(--headerbar-bg);
    box-shadow: 0 1px var(--shade);
    position: relative;
    z-index: 2;
  }
  .tb.locked {
    opacity: 0.45;
  }
  .gap {
    width: 10px;
    flex: none;
  }
  .glyph {
    display: inline-grid;
    place-items: center;
    width: 16px;
    font: 15px / 1 var(--font-serif);
  }
</style>
