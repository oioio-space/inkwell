<script lang="ts">
import { tick } from "svelte";
import { t } from "../../i18n";
import { typeOfEntry } from "../../project/kinds";
import type { Entry } from "../../project/materialize";
import FileIcon from "../icons/FileIcon.svelte";
import { fuzzy } from "./fuzzy";

/**
 * Ctrl+P: type part of a name, Enter opens the best match (FM-20). With nothing typed, the files
 * opened most recently come first.
 */
let {
  open = $bindable(false),
  entries,
  recent,
  onopen,
}: {
  open: boolean;
  entries: readonly Entry[];
  /** Ids of the files opened lately, the latest last. */
  recent: readonly string[];
  onopen: (id: string) => void;
} = $props();

let dialog: HTMLDialogElement | undefined = $state();
let input: HTMLInputElement | undefined = $state();
let query = $state("");
let active = $state(0);

const files = $derived(entries.filter((e) => e.type !== "dir"));
const results = $derived.by(() => {
  if (query.trim() === "") {
    const rank = (id: string) => recent.lastIndexOf(id);
    const ordered = [...files].sort(
      (a, b) => rank(b.id) - rank(a.id) || (a.path < b.path ? -1 : 1),
    );
    return ordered.slice(0, 50).map((item) => ({ item, at: [] as number[] }));
  }
  return fuzzy(query, files, (e) => e.path);
});

$effect(() => {
  if (open && !dialog?.open) {
    query = "";
    active = 0;
    dialog?.showModal();
    void tick().then(() => input?.focus());
  }
  if (!open && dialog?.open) dialog.close();
});

$effect(() => {
  void query;
  active = 0;
});

function choose(id: string) {
  open = false;
  onopen(id);
}

function onKey(e: KeyboardEvent) {
  if (e.key === "ArrowDown") {
    e.preventDefault();
    active = Math.min(active + 1, results.length - 1);
  } else if (e.key === "ArrowUp") {
    e.preventDefault();
    active = Math.max(active - 1, 0);
  } else if (e.key === "Enter") {
    e.preventDefault();
    const hit = results[active];
    if (hit) choose(hit.item.id);
  }
}

$effect(() => {
  if (open) document.getElementById(`qo-${active}`)?.scrollIntoView?.({ block: "nearest" });
});

/** The path with the matched characters marked. */
function parts(path: string, at: readonly number[]): { text: string; hit: boolean }[] {
  if (at.length === 0) return [{ text: path, hit: false }];
  const marked = new Set(at);
  const out: { text: string; hit: boolean }[] = [];
  for (let i = 0; i < path.length; i++) {
    const hit = marked.has(i);
    const last = out[out.length - 1];
    if (last && last.hit === hit) last.text += path[i];
    else out.push({ text: path[i] as string, hit });
  }
  return out;
}
</script>

<dialog bind:this={dialog} onclose={() => (open = false)} aria-label={t("quickopen.title")}>
  <input
    bind:this={input}
    bind:value={query}
    class="q"
    role="combobox"
    aria-expanded="true"
    aria-controls="qo-list"
    aria-activedescendant={results.length > 0 ? `qo-${active}` : undefined}
    aria-autocomplete="list"
    aria-label={t("quickopen.title")}
    placeholder={t("quickopen.placeholder")}
    spellcheck="false"
    autocomplete="off"
    onkeydown={onKey}
  />
  <ul id="qo-list" role="listbox" aria-label={t("quickopen.results")}>
    {#each results as hit, i (hit.item.id)}
      <li
        id="qo-{i}"
        role="option"
        aria-selected={i === active}
        class:active={i === active}
        onpointerdown={(e) => e.preventDefault()}
        onclick={() => choose(hit.item.id)}
        onkeydown={() => {}}
      >
        <FileIcon mark={typeOfEntry(hit.item).icon} />
        <span class="path">
          {#each parts(hit.item.path, hit.at) as p, k (k)}<span class:hit={p.hit}>{p.text}</span>{/each}
        </span>
      </li>
    {:else}
      <li class="none" role="presentation">{t("quickopen.none")}</li>
    {/each}
  </ul>
  <p class="hint">{t("quickopen.hint")}</p>
</dialog>

<style>
  dialog {
    box-sizing: border-box;
    width: min(560px, calc(100vw - 32px));
    margin-block-start: 12vh;
    padding: var(--space-2);
    border: 0;
    border-radius: var(--radius);
    background: var(--popover-bg);
    color: var(--fg);
    box-shadow: var(--shadow-popover);
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 25%);
  }
  .q {
    box-sizing: border-box;
    width: 100%;
    min-height: var(--target-min);
    padding: 0 var(--space-3);
    border: 0;
    border-radius: var(--radius-button);
    background: var(--btn-bg);
    color: inherit;
    font: 15px var(--font-ui);
  }
  ul {
    max-height: min(50vh, 360px);
    margin: var(--space-2) 0 0;
    padding: 0;
    overflow-y: auto;
    list-style: none;
  }
  li {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-height: var(--target-bar);
    padding: 0 var(--space-2);
    border-radius: var(--radius-small);
    cursor: default;
  }
  li.active {
    background: var(--selected-bg);
  }
  li.none {
    color: var(--fg-dim);
  }
  .path {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .hit {
    color: var(--accent);
    font-weight: 700;
  }
  .hint {
    margin: var(--space-2) var(--space-2) 0;
    color: var(--fg-dim);
    font-size: 12px;
  }
</style>
