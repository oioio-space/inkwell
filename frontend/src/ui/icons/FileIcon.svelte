<script lang="ts">
import { t } from "../../i18n";
import { FileKind } from "../../store/files";
import { FILE_MARKS, type FileMark } from "./paths";

/** The icon of a project file by its kind, or of a folder. The kind is also its label. */
let { kind, size = 16 }: { kind: FileKind | "folder"; size?: number } = $props();

const MARK: Record<FileKind | "folder", FileMark> = {
  [FileKind.other]: "other",
  [FileKind.typst]: "typst",
  [FileKind.markdown]: "markdown",
  [FileKind.text]: "text",
  [FileKind.csv]: "csv",
  [FileKind.bib]: "bib",
  [FileKind.image]: "image",
  [FileKind.pdf]: "pdf",
  [FileKind.data]: "data",
  folder: "folder",
};
const mark = $derived(MARK[kind]);
</script>

<svg
  class="ico"
  viewBox="0 0 16 16"
  width={size}
  height={size}
  role="img"
  aria-label={t(`kind.${mark}`)}
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
