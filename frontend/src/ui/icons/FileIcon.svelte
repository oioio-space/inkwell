<script lang="ts">
import { type MessageKey, t } from "../../i18n";
import { workspace } from "../../workspace/workspace.svelte";
import { FILE_MARKS, type FileMark } from "./paths";

/**
 * The icon of a project file by its type (ADR 0027), or of a folder. The type's label is also the
 * icon's. A name no type claims has the generic icon until it is opened.
 */
let {
  path = "",
  folder = false,
  size = 16,
}: { path?: string; folder?: boolean; size?: number } = $props();

const type = $derived(folder ? undefined : workspace.fileTypeOf(path));
const mark = $derived<FileMark>(
  folder
    ? "folder"
    : type && Object.hasOwn(FILE_MARKS, type.icon)
      ? (type.icon as FileMark)
      : "other",
);
const label = $derived(
  folder ? t("kind.folder") : type ? t(type.label as MessageKey) : t("kind.other"),
);
</script>

<svg
  class="ico"
  viewBox="0 0 16 16"
  width={size}
  height={size}
  role="img"
  aria-label={label}
  data-kind={mark}
>
  {@html FILE_MARKS[mark]}
</svg>

<style>
  .ico {
    flex: none;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  /* Only the inner mark is coloured; the outline stays neutral. */
  .ico :global(.mk-typst) {
    stroke: var(--mark-typst);
  }
  .ico :global(.mk-markdown) {
    stroke: var(--mark-markdown);
  }
  .ico :global(.mk-csv) {
    stroke: var(--mark-csv);
  }
  .ico :global(.mk-bib) {
    stroke: var(--mark-bib);
  }
  .ico :global(.mk-image) {
    stroke: var(--mark-image);
  }
  .ico :global(.mk-pdf) {
    stroke: var(--mark-pdf);
  }
  .ico :global(.mk-data) {
    stroke: var(--mark-data);
  }
</style>
