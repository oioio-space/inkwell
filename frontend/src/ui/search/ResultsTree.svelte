<script lang="ts">
import type { Hit, ResolvedHit } from "../../editor/search/project";
import { t } from "../../i18n";
import type { SearchRun } from "./results";

let {
  run,
  locate,
  active = "",
  ontoggle,
  onactivate,
  label,
}: {
  run: SearchRun;
  /** Where a hit is now; a stale one is greyed out and cannot be jumped to. */
  locate: (path: string, hit: Hit) => ResolvedHit;
  /** Key of the current result ("path:index"), set by F3 / F4 / clicks. */
  active?: string;
  ontoggle: (path: string) => void;
  onactivate: (path: string, index: number) => void;
  label: string;
} = $props();

let root: HTMLUListElement;
let focusKey = $state("");

const fileKey = (path: string) => `f:${path}`;
const hitKey = (path: string, i: number) => `h:${path}:${i}`;

/** The keys of the rows that are on screen, top to bottom. */
const visible = $derived(
  run.results.files.flatMap((f) => [
    fileKey(f.path),
    ...(run.collapsed.has(f.path) ? [] : f.hits.map((_, i) => hitKey(f.path, i))),
  ]),
);
const tabbable = $derived(visible.includes(focusKey) ? focusKey : (visible[0] ?? ""));

function focusRow(key: string | undefined) {
  if (key === undefined) return;
  focusKey = key;
  root.querySelector<HTMLElement>(`[data-key="${CSS.escape(key)}"]`)?.focus();
}

function move(key: string, delta: number) {
  const at = visible.indexOf(key);
  focusRow(visible[Math.min(visible.length - 1, Math.max(0, at + delta))]);
}

function keydown(e: KeyboardEvent, key: string, file: string, index: number) {
  if (e.target !== e.currentTarget) return;
  const isFile = index < 0;
  const open = !run.collapsed.has(file);
  switch (e.key) {
    case "ArrowDown":
      move(key, 1);
      break;
    case "ArrowUp":
      move(key, -1);
      break;
    case "Home":
      focusRow(visible[0]);
      break;
    case "End":
      focusRow(visible[visible.length - 1]);
      break;
    case "ArrowRight":
      if (isFile && !open) ontoggle(file);
      else if (isFile) move(key, 1);
      break;
    case "ArrowLeft":
      if (isFile && open) ontoggle(file);
      else if (!isFile) focusRow(fileKey(file));
      break;
    case "Enter":
    case " ":
      if (isFile) ontoggle(file);
      else onactivate(file, index);
      break;
    default:
      return;
  }
  e.preventDefault();
}

function click(e: MouseEvent, key: string, file: string, index: number) {
  if ((e.target as HTMLElement).closest('[role="treeitem"]') !== e.currentTarget) return;
  focusKey = key;
  if (index < 0) ontoggle(file);
  else onactivate(file, index);
}
</script>

<ul class="tree" role="tree" aria-label={label} bind:this={root}>
  {#each run.results.files as f (f.path)}
    {@const open = !run.collapsed.has(f.path)}
    {@const fk = fileKey(f.path)}
    <li role="none">
      <div
        class="item file"
        role="treeitem"
        aria-level="1"
        aria-selected="false"
        aria-expanded={open}
        data-key={fk}
        tabindex={tabbable === fk ? 0 : -1}
        onkeydown={(e) => keydown(e, fk, f.path, -1)}
        onclick={(e) => click(e, fk, f.path, -1)}
      >
        <span class="head">
          <span class="chev" class:open aria-hidden="true">›</span>
          <span class="path">{f.path}</span>
          <span class="badge">{f.hits.length}{f.truncated ? "+" : ""}</span>
        </span>
        {#if open}
          <ul role="group">
            {#each f.hits as hit, i (i)}
              {@const hk = hitKey(f.path, i)}
              {@const at = locate(f.path, hit)}
              <li role="none">
                <div
                  class="item hit"
                  class:stale={at.stale}
                  class:active={active === `${f.path}:${i}`}
                  role="treeitem"
                  aria-level="2"
                  aria-selected={active === `${f.path}:${i}`}
                  aria-disabled={at.stale}
                  data-key={hk}
                  title={at.stale ? t("search.stale") : undefined}
                  tabindex={tabbable === hk ? 0 : -1}
                  onkeydown={(e) => keydown(e, hk, f.path, i)}
                  onclick={(e) => click(e, hk, f.path, i)}
                >
                  <span class="ln">{hit.line}</span>
                  <span class="txt"
                    >{hit.lineText.slice(0, hit.hitFrom)}<mark>{hit.lineText.slice(hit.hitFrom, hit.hitTo)}</mark
                    >{hit.lineText.slice(hit.hitTo)}</span
                  >
                </div>
              </li>
            {/each}
          </ul>
        {/if}
      </div>
    </li>
  {/each}
</ul>

<style>
  .tree,
  ul[role="group"] {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .item {
    border-radius: var(--radius-small);
    cursor: default;
  }
  .item:focus-visible {
    outline: none;
  }
  .file:focus-visible > .head,
  .hit:focus-visible {
    outline: var(--focus-ring);
    outline-offset: -2px;
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-height: 34px;
    padding: 0 var(--space-2);
    border-radius: var(--radius-small);
    font-weight: 700;
  }
  .head:hover,
  .hit:hover {
    background: color-mix(in srgb, var(--fg) 7%, transparent);
  }
  .chev {
    display: inline-block;
    width: 1em;
    color: var(--fg-dim);
    transition: transform var(--duration);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  .path {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .badge {
    min-width: 1.6em;
    padding: 0 var(--space-2);
    border-radius: 999px;
    background: color-mix(in srgb, var(--fg) 10%, transparent);
    font-size: 12px;
    font-weight: 600;
    text-align: center;
    font-variant-numeric: tabular-nums;
  }
  .hit {
    display: flex;
    gap: var(--space-3);
    align-items: baseline;
    min-height: 30px;
    padding: 3px var(--space-2) 3px var(--space-4);
    font: 13px/1.5 var(--font-mono);
  }
  .hit.active {
    background: color-mix(in srgb, var(--accent) 16%, transparent);
  }
  .ln {
    flex: none;
    min-width: 3ch;
    text-align: right;
    color: var(--fg-dim);
    font-variant-numeric: tabular-nums;
  }
  .txt {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: pre;
  }
  mark {
    background: color-mix(in srgb, var(--warning) 40%, transparent);
    color: inherit;
    border-radius: 2px;
  }
  .stale {
    opacity: 0.45;
    text-decoration: line-through;
    text-decoration-thickness: 1px;
  }
  .stale mark {
    background: transparent;
  }
</style>
