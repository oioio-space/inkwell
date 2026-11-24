<script lang="ts">
import type { NativeViewerProps } from "../../../extensions/context";
import { t } from "../../../i18n";

/** A picture of the project. Bytes that do not decode as one get the file card's words (ND-7). */
let { path, readImage }: NativeViewerProps = $props();
let src = $state<string | undefined>();
let failed = $state(false);

$effect(() => {
  const p = path;
  let live = true;
  src = undefined;
  failed = false;
  void readImage(p).then((url) => {
    if (!live) return;
    if (url) src = url;
    else failed = true;
  });
  return () => {
    live = false;
  };
});
</script>

{#if failed}
  <div class="file-view" tabindex="-1" role="status"><h2>{path}</h2><p>{t("file.unavailable")}</p></div>
{:else if src}
  <div class="file-view" tabindex="-1"><img {src} alt={path} onerror={() => (failed = true)} /></div>
{:else}
  <div class="file-view" tabindex="-1" role="status"><h2>{path}</h2><p>{t("file.loading")}</p></div>
{/if}

<style>
  .file-view {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 24px;
    color: var(--fg-dim);
    background: var(--view-bg);
  }
  .file-view img {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
  }
</style>
