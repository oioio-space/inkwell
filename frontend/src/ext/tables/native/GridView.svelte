<script lang="ts">
import { tableType } from "../../../editor/csv/dialect";
import type { NativeViewerProps } from "../../../extensions/context";
import Sheet from "../../../ui/sheet/Sheet.svelte";

/** A delimited table as a spreadsheet, its delimiter from the file type's options. */
let {
  opening,
  handle,
  big,
  me,
  path,
  readOnly,
  readOnlyReason,
  startOffset,
  onshowtext,
}: NativeViewerProps = $props();
// A type the person chose says nothing about the content: nothing is reported against its name.
const table = $derived(tableType(opening.type.options, !opening.chosen));
</script>

<Sheet {handle} {big} {me} {path} {table} {readOnly} {readOnlyReason} {startOffset} {onshowtext} />
