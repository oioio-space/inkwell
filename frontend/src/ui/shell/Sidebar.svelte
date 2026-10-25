<script lang="ts">
import { SvelteSet } from "svelte/reactivity";
import { t } from "../../i18n";
import type { ProjectFile } from "../../store/files";
import FileIcon from "../icons/FileIcon.svelte";
import Icon from "../icons/Icon.svelte";
import { buildTree, visibleNodes } from "./tree";

/** The project's files as a tree; folders fold, the main document is marked. */
let {
  files,
  current,
  onOpen,
}: {
  files: readonly ProjectFile[];
  current: string;
  onOpen: (path: string) => void;
} = $props();

const collapsed = new SvelteSet<string>();
const nodes = $derived(visibleNodes(buildTree(files), collapsed));

function toggle(path: string) {
  if (collapsed.has(path)) collapsed.delete(path);
  else collapsed.add(path);
}
</script>

<aside class="side" aria-label={t("sidebar.label")}>
  <div class="head"><h2>{t("sidebar.title")}</h2></div>
  <nav aria-label={t("sidebar.files")}>
    <ul>
      {#each nodes as node (node.path)}
        <li style:--depth={node.depth}>
          {#if node.type === "folder"}
            {@const open = !collapsed.has(node.path)}
            <button type="button" class="row folder" aria-expanded={open} onclick={() => toggle(node.path)}>
              <span class="chev" class:open><Icon name="right" size={12} /></span>
              <FileIcon kind="folder" />
              <span class="name">{node.name}</span>
            </button>
          {:else}
            <button
              type="button"
              class="row"
              class:sel={node.path === current}
              aria-current={node.path === current ? "page" : undefined}
              data-path={node.path}
              onclick={() => onOpen(node.path)}
            >
              <span class="chev spacer"></span>
              <FileIcon kind={node.file.kind} />
              <span class="name">{node.name}</span>
              {#if node.file.main}<span class="tag">{t("sidebar.main")}</span>{/if}
            </button>
          {/if}
        </li>
      {/each}
    </ul>
  </nav>
</aside>

<style>
  .side {
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    width: 264px;
    min-height: 0;
    background: var(--sidebar-bg);
    box-shadow: inset -1px 0 var(--card-shade);
  }
  .head {
    display: grid;
    place-items: center;
    height: 47px;
    flex: none;
  }
  h2 {
    margin: 0;
    font: 700 14.5px/1 var(--font-ui);
  }
  nav {
    flex: 1;
    min-height: 0;
    padding: 6px;
    overflow-y: auto;
  }
  ul {
    display: flex;
    flex-direction: column;
    gap: 2px;
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .row {
    box-sizing: border-box;
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    height: 36px;
    padding: 0 10px 0 calc(6px + var(--depth, 0) * 20px);
    border: 0;
    border-radius: var(--radius-small);
    background: transparent;
    color: inherit;
    font: inherit;
    text-align: start;
    cursor: default;
  }
  .row:hover {
    background: var(--hover-bg);
  }
  .row.sel {
    background: var(--selected-bg);
    font-weight: 600;
  }
  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .tag {
    color: var(--fg-dim);
    font-size: 12px;
    font-weight: 400;
  }
  .row.sel .tag {
    color: var(--fg);
  }
  .chev {
    display: inline-grid;
    place-items: center;
    width: 12px;
    color: var(--fg-dim);
    transition: transform var(--duration);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  @media (prefers-reduced-motion: reduce) {
    .chev {
      transition: none;
    }
  }
</style>
