<script lang="ts">
import type { EditorState, StateCommand } from "@codemirror/state";
import { featureState, formatsOf, hintOf, type Lang } from "../../editor/format";
import { menuOf, nameOf } from "../../extensions/commands";
import { type MessageKey, t } from "../../i18n";
import type { ProjectFile } from "../../store/files";
import { workspace } from "../../workspace/workspace.svelte";
import Icon from "../icons/Icon.svelte";
import MenuButton from "../popover/MenuButton.svelte";
import { visibleCount } from "../toolbar/layout";
import BarButton from "./BarButton.svelte";
import CommandIcon from "./CommandIcon.svelte";
import CommandMenuItem from "./CommandMenuItem.svelte";
import HeadingMenu from "./HeadingMenu.svelte";
import InsertMenu from "./InsertMenu.svelte";

/**
 * The formatting bar of a file: the `format` location (EX0c1) for the formats its language
 * writes, between the core's heading menu and Insert. Buttons follow the caret (aria-pressed);
 * what does not fit goes to a "⋯" menu, in the same order. Each runs its command on the editor
 * (`target`) through the registry. A file whose formats show no item gets no bar (the caller
 * asks `locationShows`).
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
  lang: Lang;
  editorState: EditorState;
  /** The editor the commands act on (its surface). */
  target: Element | undefined;
  /** Runs an insertion of the Insert menu on the editor (after its step: a file, a key). */
  run: (command: StateCommand) => boolean;
  files: readonly ProjectFile[];
  currentFile: string;
  readFile: (path: string) => Promise<string | undefined>;
} = $props();

let width = $state(1200);
let moreOpen = $state(false);

const items = $derived(
  menuOf("format", { formats: formatsOf(lang) }).flatMap((g, group) =>
    g.items.map((command) => ({ command, group, feature: nameOf(command.id) })),
  ),
);
const count = $derived(
  visibleCount(
    items.map((i) => i.group),
    width,
  ),
);
const shown = $derived(items.slice(0, count));
const hidden = $derived(items.slice(count));
const pressed = $derived(featureState(editorState, lang));

const languageName = $derived(t(lang.label as MessageKey));
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
  <HeadingMenu {lang} {editorState} {target} />
  <span class="gap"></span>
  {#each shown as item, i (item.command.id)}
    {#if i > 0 && shown[i - 1]?.group !== item.group}<span class="gap"></span>{/if}
    <BarButton
      command={item.command}
      feature={item.feature}
      hint={hintOf(lang, item.feature)}
      pressed={pressed(item.feature)}
      {target}
    />
  {/each}
  {#if hidden.length > 0}
    <MenuButton bind:open={moreOpen} label={t("fmt.more")} name={t("fmt.more")} width={260} align="start">
      {#snippet trigger()}<Icon name="more" />{/snippet}
      {#snippet children({ close })}
        {#each hidden as item (item.command.id)}
          <CommandMenuItem id={item.command.id} {close} {target} checked={pressed(item.feature)} refocus data-bar={item.feature}>
            {#snippet lead()}<CommandIcon icon={item.command.icon} lead />{/snippet}
          </CommandMenuItem>
        {/each}
      {/snippet}
    </MenuButton>
  {/if}
  <span class="gap"></span>
  <InsertMenu {lang} {editorState} {target} {run} {files} {currentFile} {readFile} />
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
</style>
