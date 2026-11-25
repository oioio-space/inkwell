<script lang="ts">
import { cursorPosition } from "../../../../editor/stats/position";
import { t } from "../../../../i18n";
import type { StatusItemProps } from "../../../../ui/command/status.svelte";

/** The caret's line and column. */
let { bar }: StatusItemProps = $props();
const position = $derived(
  bar.editorState ? cursorPosition(bar.editorState) : { line: 1, column: 1 },
);
</script>

<span class="fb pos" data-testid="position">{t("status.position", { line: position.line, column: position.column })}</span>
