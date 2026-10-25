<script lang="ts">
import { t } from "../i18n";
import { workspace } from "../workspace/workspace.svelte";
import Banner from "./Banner.svelte";
</script>

{#if workspace.projectionFailed}
  <Banner tone="warning" role="alert">{t("projection.failed")}</Banner>
{/if}
