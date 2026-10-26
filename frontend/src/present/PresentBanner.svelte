<script lang="ts">
import { t } from "../i18n";
import Banner from "../ui/Banner.svelte";
import { presentSession as session } from "./session.svelte";

/** "Paul présente." with a button to follow: shown under the toolbar while someone else presents. */
// Watching the project's presenters lasts as long as this strip exists, whatever it shows.
$effect(() => session.watch());
</script>

{#if session.announced && !session.current}
  <div data-testid="present-banner">
    <Banner role="status">
      {t("present.banner", { name: session.announced.name })}
      {#snippet action()}
        <button type="button" class="sug" data-testid="present-banner-follow" onclick={() => session.start("follower")}>
          {t("present.banner.follow")}
        </button>
      {/snippet}
    </Banner>
  </div>
{/if}
