<script lang="ts">
import { t } from "../../../../i18n";
import { preview } from "../../../../preview/preview.svelte";

/** The page the preview shows, while it is open on the main document (`main && previewOpen`). */
</script>

{#if preview.pageCount > 0}
  <span class="fb page" data-testid="page-indicator">
    {t("status.page", { page: Math.max(1, preview.currentPage), total: preview.pageCount })}
  </span>
{/if}
