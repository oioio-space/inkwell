<script lang="ts">
import { t } from "../../i18n";
import Banner from "../Banner.svelte";

/**
 * Shown when the index holds more files than the limit: none is hidden or dropped, but no new one
 * is accepted until some are removed.
 */
let { excess }: { excess: number } = $props();
</script>

{#if excess > 0}
  <Banner tone="warning" role="status">{t("files.limit.over", { count: excess })}</Banner>
{/if}
