<script lang="ts">
import type { EditorState, StateCommand } from "@codemirror/state";
import { onMount } from "svelte";
import {
  INSERT_INPUT,
  type InsertArgs,
  type InsertItem,
  insert,
  insertItemOf,
  insertPrecondition,
  insertStepTaken,
  insertsOf,
  type Lang,
  onInsertStep,
  relativePath,
} from "../../editor/format";
import { registry } from "../../extensions";
import { cmdAttrs, labelOf, menuOf, textOf } from "../../extensions/commands";
import { locationOf } from "../../extensions/manifest";
import type { Command } from "../../extensions/registry";
import { type MessageKey, t } from "../../i18n";
import type { ProjectFile } from "../../store/files";
import FileIcon from "../icons/FileIcon.svelte";
import Icon from "../icons/Icon.svelte";
import MenuButton from "../popover/MenuButton.svelte";
import { bibKeys, parseCsv, typstLabels } from "../toolbar/csv";
import CommandIcon from "./CommandIcon.svelte";

/**
 * The Insert menu: the `insert` location (EX0c3) for the items the file's language offers, by
 * group under its caption. Each item is its command (`data-cmd`, named by its label, described by
 * its description and the syntax it writes). An item that needs nothing inserts at once; one that
 * needs a file or a key opens its step here, as a command run from the palette does
 * (`openInsertStep`). The file types an item lists are its language's `accepts`.
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
  run: (command: StateCommand) => boolean;
  files: readonly ProjectFile[];
  currentFile: string;
  /** The text of a project file (for the CSV copy and the citation keys). */
  readFile: (path: string) => Promise<string | undefined>;
} = $props();

interface Entry {
  command: Command;
  item: InsertItem;
}

const groups = $derived(
  menuOf("insert", { inserts: insertsOf(lang) }, target).map((g) => ({
    group: g.group,
    caption: locationOf("insert")?.groups.find((x) => x.id === g.group)?.caption,
    entries: g.items.flatMap((command): Entry[] => {
      const item = insertItemOf(command.id);
      return item ? [{ command, item }] : [];
    }),
  })),
);

/** The description under an item: the language's own (Markdown's frozen copy), else the command's. */
function description(e: Entry): string {
  const own = lang.describe?.[e.item];
  if (own) return t(own);
  return e.command.tooltip ? textOf(e.command, e.command.tooltip) : "";
}

type Step =
  | { kind: "files"; item: InsertItem }
  | { kind: "key"; item: InsertItem }
  | { kind: "nobib" };

let open = $state(false);
let step = $state<Step | undefined>();
let keys = $state<{ key: string; hint: string }[]>([]);
let typed = $state("");
let busy = $state(false);

const doc = $derived(editorState.doc.toString());
const selected = $derived(
  editorState.sliceDoc(editorState.selection.main.from, editorState.selection.main.to),
);
const hasBibliography = $derived(/#bibliography\(/.test(doc));

function reset() {
  step = undefined;
  keys = [];
  typed = "";
  insertStepTaken();
}

/** An item chosen: inserted at once, or its step (the file, the key, a bibliography first). */
function go(item: InsertItem, close: (refocus?: boolean) => void) {
  const input = INSERT_INPUT[item];
  if (input === "nothing") {
    run(insert(lang, item));
    close();
  } else if (input === "file") {
    step = { kind: "files", item };
  } else if (insertPrecondition(lang, item, { hasBibliography })) {
    step = { kind: "nobib" };
  } else {
    typed = "";
    step = { kind: "key", item };
    void loadKeys(item);
  }
}

// A command run from the palette or a key: the menu opens at that item's step.
onMount(() =>
  onInsertStep((item) => {
    insertStepTaken();
    open = true;
    go(item, () => (open = false));
  }),
);

async function loadKeys(item: InsertItem) {
  if (item === "ref") {
    keys = typstLabels(doc).map((key) => ({ key, hint: "" }));
    return;
  }
  const found: { key: string; hint: string }[] = [];
  for (const f of candidates("bibliography")) {
    const text = await readFile(f.path);
    for (const key of text ? bibKeys(text) : []) found.push({ key, hint: f.path });
  }
  keys = found;
}

async function pick(item: InsertItem, file: ProjectFile, close: (refocus?: boolean) => void) {
  busy = true;
  try {
    const args: InsertArgs = { path: file.path, currentFile };
    if (item === "link") {
      args.url = relativePath(currentFile, file.path);
      args.text = selected || file.path.slice(file.path.lastIndexOf("/") + 1);
    } else if (lang.needsRows?.has(item)) {
      const text = await readFile(file.path);
      args.rows = text ? parseCsv(text) : [];
    }
    run(insert(lang, item, args));
    close();
  } finally {
    busy = false;
  }
}

function useKey(item: InsertItem, key: string, close: (refocus?: boolean) => void) {
  if (!key.trim()) return;
  run(insert(lang, item, { key: key.trim() }));
  close();
}

/** The project's files an item takes, by path: the types its language accepts, else every one. */
const candidates = (item: InsertItem) => {
  const types = lang.accepts[item];
  return files
    .filter((f) => !types || types.includes(registry.typeOf(f.path)?.id ?? ""))
    .sort((a, b) => a.path.localeCompare(b.path));
};

const iconOf = (item: InsertItem) =>
  groups.flatMap((g) => g.entries).find((e) => e.item === item)?.command.icon;

/** What to say when the project has no file of the kind an item needs. */
function kindHint(item: InsertItem): MessageKey {
  switch (item) {
    case "include":
      return "insert.empty.typst";
    case "bibliography":
      return "insert.empty.bib";
    case "csvTable":
      return "insert.empty.csv";
    case "data":
      return "insert.empty.data";
    case "image":
    case "figure":
      return "insert.empty.image";
    default:
      return "insert.empty.any";
  }
}
</script>

<MenuButton bind:open label={t("insert.menu")} width={520} align="start" onclose={reset}>
  {#snippet trigger()}
    <Icon name="plus" />
    <span>{t("insert.button")}</span>
    <Icon name="down" size={12} />
  {/snippet}
  {#snippet children({ close })}
    {#if !step}
      {#each groups as g, gi (g.group)}
        {#if gi > 0}<div class="sep"></div>{/if}
        {#if g.caption}<div class="cap">{t(g.caption as MessageKey)}</div>{/if}
        {#each g.entries as e (e.command.id)}
          {@const desc = description(e)}
          {@const syntax = lang.insertSyntax[e.item]}
          {@const described = [desc && `insert-desc-${e.item}`, syntax && `insert-syntax-${e.item}`]}
          <!-- The item is its command (named by its label, WCAG 2.5.3); its description and syntax
               sit beside it in the row, described by them, and the row is its hit area. -->
          <div class="mi two entry" role="none">
            <CommandIcon icon={e.command.icon} lead />
            <span class="text">
              <button
                type="button"
                class="hit"
                role="menuitem"
                {...cmdAttrs(e.command.id)}
                title={undefined}
                aria-describedby={described.filter(Boolean).join(" ") || undefined}
                data-insert={e.item}
                onclick={() => go(e.item, close)}
              >{labelOf(e.command)}</button>
              {#if desc}<span class="sub" id="insert-desc-{e.item}">{desc}</span>{/if}
            </span>
            {#if syntax}<span class="k mono" id="insert-syntax-{e.item}">{syntax}</span>{/if}
          </div>
        {/each}
      {/each}
      {#if lang.note}
        <div class="sep"></div>
        <p class="note sub" data-fixed="insert-note">{t(lang.note)}</p>
      {/if}
    {:else}
      <button type="button" class="mi back" role="menuitem" data-fixed="insert-back" onclick={reset}>
        <Icon name="back" />
        <span>{t("insert.back")}</span>
      </button>
      <div class="sep"></div>
      {#if step.kind === "nobib"}
        <p class="note" data-fixed="insert-nobib">{t("insert.nobib")}</p>
        <button
          type="button"
          class="mi"
          role="menuitem"
          data-fixed="insert-add-bib"
          onclick={() => (step = { kind: "files", item: "bibliography" })}
        >
          <Icon name="book" />
          <span>{t("insert.addBibliography")}</span>
        </button>
      {:else if step.kind === "files"}
        {@const item = step.item}
        <div class="cap">{t("insert.choose")}</div>
        {#each candidates(item) as f (f.path)}
          <button type="button" class="mi" role="menuitem" data-fixed="insert-file" disabled={busy} onclick={() => pick(item, f, close)}>
            <FileIcon path={f.path} />
            <span class="text">{f.path}</span>
          </button>
        {:else}
          <p class="note sub" data-fixed="insert-note">{t(kindHint(item))}</p>
        {/each}
      {:else}
        {@const item = step.item}
        <form
          class="keyform"
          data-fixed="insert-keyform"
          onsubmit={(e) => {
            e.preventDefault();
            useKey(item, typed, close);
          }}
        >
          <label>
            <span class="cap">{t(item === "cite" ? "insert.key.cite" : "insert.key.ref")}</span>
            <input bind:value={typed} spellcheck="false" autocomplete="off" />
          </label>
          <button type="submit" class="btn" disabled={!typed.trim()}>{t("insert.key.insert")}</button>
        </form>
        {#each keys as k (k.key)}
          <button type="button" class="mi" role="menuitem" data-fixed="insert-key" onclick={() => useKey(item, k.key, close)}>
            <CommandIcon icon={iconOf(item)} lead />
            <span class="text mono">{k.key}</span>
            {#if k.hint}<span class="k">{k.hint}</span>{/if}
          </button>
        {/each}
      {/if}
    {/if}
  {/snippet}
</MenuButton>

<style>
  .note {
    margin: 0;
    padding: 6px 10px 8px;
    font-size: 12.5px;
    line-height: 1.45;
  }
  .keyform {
    display: grid;
    gap: 6px;
    padding: 0 10px 8px;
  }
  .keyform label {
    display: grid;
  }
  .keyform .cap {
    padding: 0 0 4px;
  }
  input {
    min-height: var(--target-bar);
    padding: 0 10px;
    border: 0;
    border-radius: var(--radius-button);
    background: var(--btn-bg);
    color: inherit;
    font: inherit;
    font-family: var(--font-mono);
  }
  .entry {
    position: relative;
  }
  .hit {
    padding: 0;
    border: 0;
    background: none;
    color: inherit;
    font: inherit;
    text-align: start;
    cursor: default;
  }
  /* The whole row takes the pointer; the focus shows on the row. */
  .hit::after {
    content: "";
    position: absolute;
    inset: 0;
  }
  .hit:focus {
    outline: none;
  }
  .entry:has(.hit:focus) {
    background: var(--hover-bg);
  }
  .entry:has(.hit:focus-visible) {
    outline: var(--focus-ring);
    outline-offset: -2px;
  }
  .mi :global(.ico) {
    color: var(--fg);
    opacity: 0.85;
  }
</style>
