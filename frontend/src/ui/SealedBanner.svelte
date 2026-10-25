<script lang="ts">
import { t } from "../i18n";
import { workspace } from "../workspace/workspace.svelte";
import Banner from "./Banner.svelte";
</script>

{#if workspace.sealed}
  <Banner tone="warning" role="alert">{t("sealed.message")}</Banner>
{/if}
