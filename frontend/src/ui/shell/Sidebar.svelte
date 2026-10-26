<script lang="ts">
import { t } from "../../i18n";
import type { ProjectRuntime } from "../../project/runtime.svelte";
import FileManager from "../files/FileManager.svelte";

/**
 * The left panel. Its views are tabs in the manner of an AdwViewSwitcher: the file manager is
 * the first (« Fichiers »), and « Sommaire » and « Tâches » (plan B10) join it by adding an entry
 * to `views`, not by changing the panel. Arrows move between tabs; the panel under the switcher
 * is the selected one.
 */
let {
  runtime,
  onOpen,
}: {
  runtime: ProjectRuntime | undefined;
  onOpen: (path: string) => void;
} = $props();

const views = [{ id: "files", label: "sidebar.title" as const }];
let selected = $state("files");

function onKey(e: KeyboardEvent) {
  const i = views.findIndex((v) => v.id === selected);
  const target: Record<string, number> = {
    ArrowRight: i + 1,
    ArrowLeft: i - 1,
    Home: 0,
    End: views.length - 1,
  };
  const at = target[e.key];
  if (at === undefined) return;
  const to = views[(at + views.length) % views.length];
  if (!to) return;
  e.preventDefault();
  selected = to.id;
  document.getElementById(`tab-${to.id}`)?.focus();
}
</script>

<aside class="side" aria-label={t("sidebar.label")}>
  <div class="head">
    <div class="switcher" role="tablist" aria-label={t("sidebar.label")}>
      {#each views as v (v.id)}
        <button
          type="button"
          id="tab-{v.id}"
          class="tab"
          role="tab"
          aria-selected={selected === v.id}
          aria-controls="panel-{v.id}"
          tabindex={selected === v.id ? 0 : -1}
          onclick={() => (selected = v.id)}
          onkeydown={onKey}
        >
          {t(v.label)}
        </button>
      {/each}
    </div>
  </div>
  <div class="panel" id="panel-files" role="tabpanel" aria-labelledby="tab-files" hidden={selected !== "files"}>
    {#if runtime}
      <FileManager {runtime} onOpen={() => onOpen(runtime.currentPath)} />
    {/if}
  </div>
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
  /* The switcher: a pill of tabs, the selected one raised, as in libadwaita's inline switcher. */
  .switcher {
    display: flex;
    gap: 2px;
    padding: 3px;
    border-radius: var(--radius-button);
    background: var(--btn-bg);
  }
  .tab {
    min-width: 72px;
    height: 28px;
    padding: 0 14px;
    border: 0;
    border-radius: calc(var(--radius-button) - 2px);
    background: transparent;
    color: inherit;
    font: 700 13.5px/1 var(--font-ui);
    cursor: default;
  }
  .tab:hover {
    background: var(--hover-bg);
  }
  .tab[aria-selected="true"] {
    background: var(--view-bg);
    box-shadow: var(--shadow-card);
  }
  .tab:focus-visible {
    outline: var(--focus-ring);
    outline-offset: 1px;
  }
  .panel {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
  }
  .panel[hidden] {
    display: none;
  }
</style>
