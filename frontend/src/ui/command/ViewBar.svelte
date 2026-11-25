<script lang="ts">
import { menuOf } from "../../extensions/commands";
import type { Viewer } from "../../extensions/registry";
import { type MessageKey, t } from "../../i18n";
import LocationBar from "./LocationBar.svelte";

/**
 * The view bar (EX0c2): the `view` location around its two fixed widgets. ‹offer-table› offers a
 * text that reads as a table as one; ‹default-viewer› goes back to the type's default viewer, and
 * shows only while another viewer is shown (a grid has no view bar, today's rule). A pick of a
 * viewer is no command (VS Code's « Reopen With » is a pick): only the extensions' items are.
 */
let {
  offer,
  onoffer,
  defaultViewer,
  shown,
  ondefault,
}: {
  /** The file reads as a table and is not shown as one: « Ouvrir comme tableau ? ». */
  offer: boolean;
  onoffer: () => void;
  /** The type's default viewer, and the one showing the file now. */
  defaultViewer: Viewer | undefined;
  shown: Viewer | undefined;
  ondefault: () => void;
} = $props();

/** The default viewer is offered only while another one shows the file (P-9). */
const viewer = $derived(
  defaultViewer && shown && shown !== defaultViewer ? defaultViewer : undefined,
);
const extensions = $derived(menuOf("view").length > 0);
</script>

{#if offer || viewer || extensions}
  <div class="viewbar">
    {#if offer}
      <button type="button" class="fb lbl" data-fixed="offer-table" onclick={onoffer}>
        {t("sheet.openAsTable")}
      </button>
    {/if}
    {#if viewer}
      <button type="button" class="fb lbl" data-fixed="default-viewer" data-viewer={viewer.id} onclick={ondefault}>
        {t(viewer.label as MessageKey)}
      </button>
    {/if}
    <LocationBar location="view" />
  </div>
{/if}

<style>
  .viewbar {
    display: flex;
    align-items: center;
    min-height: 40px;
    padding: 3px 6px;
    background: var(--headerbar-bg);
    box-shadow: 0 1px var(--shade);
  }
</style>
