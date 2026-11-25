<script lang="ts">
import { t } from "../../../../i18n";
import type { StatusItemProps } from "../../../../ui/command/status.svelte";

/** The text is not in the file yet: said until it is (protected, never hidden). */
let { bar }: StatusItemProps = $props();
</script>

{#if bar.unsaved}
  <span class="fb unsaved" data-testid="unsaved" title={t("save.unsaved.hint")}>{t("save.unsaved")}</span>
{/if}
