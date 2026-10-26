<script lang="ts">
import { resolve } from "../../filetypes";
import { t } from "../../i18n";
import { workspace } from "../../workspace/workspace.svelte";
import Banner from "../Banner.svelte";
import Editor from "../Editor.svelte";

/**
 * The area of the editors: one per open file (each with its own caret, scroll and undo history,
 * or its card when the file is not text), and the banner when the file shown was put in the
 * trash by someone while it was open (PRJ-06, FM-09).
 */
let { me, at }: { me: { id: string; name: string }; at?: number | undefined } = $props();

const rt = $derived(workspace.runtime);
const trashed = $derived(rt?.trashedCurrent);
const who = (peer: string) =>
  peer === workspace.myId
    ? workspace.myName
    : (workspace.names[peer] ?? `Personne ${peer.slice(0, 4)}`);

/** The open files, each with the type that says how it opens. */
const editors = $derived.by(() => {
  void rt?.ready;
  return (rt?.openIds ?? []).flatMap((id) => {
    const entry = rt?.tree.byId.get(id);
    const isText = entry ? entry.type === "text" : true;
    const handle = isText ? rt?.handleOf(id) : undefined;
    if (isText && !handle) return [];
    const path = entry?.path ?? (id === rt?.currentId ? rt?.currentPath : "") ?? "";
    const name = path.slice(path.lastIndexOf("/") + 1);
    const type = resolve({ name, override: entry?.ftype, isText });
    return [{ id, handle, path, name, type, size: entry?.size ?? 0 }];
  });
});
</script>

<div class="pane">
  {#if trashed && rt}
    <Banner tone="warning" role="status">
      {t("files.banner.trashed", { name: trashed.name, who: who(trashed.by) })}
      {#snippet action()}
        {#if rt.role === "admin" || rt.index.mode === "free"}
          <button type="button" class="btn" onclick={() => rt.index.restore([trashed.id])}>
            {t("files.banner.restore")}
          </button>
        {/if}
      {/snippet}
    </Banner>
  {/if}
  <div class="stack">
    {#each editors as ed (ed.id)}
      <Editor
        handle={ed.handle}
        {me}
        type={ed.type}
        name={ed.name}
        path={ed.path}
        size={ed.size}
        active={ed.id === rt?.currentId}
        at={ed.id === rt?.currentId ? at : undefined}
      />
    {/each}
  </div>
</div>

<style>
  .pane {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .stack {
    flex: 1;
    min-height: 0;
  }
</style>
