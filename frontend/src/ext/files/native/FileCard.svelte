<script lang="ts">
import type { NativeViewerProps } from "../../../extensions/context";
import { t } from "../../../i18n";

/** The file card: what shows a file no other viewer can, its name and why there is no preview. */
let { opening, path, failed, large }: NativeViewerProps = $props();
const why = $derived(
  failed ? "file.unavailable" : opening.notice && !large ? "file.notText" : "file.info",
);
</script>

<div class="file-view" tabindex="-1" role="status"><h2>{path}</h2><p>{t(why)}</p></div>

<style>
  .file-view {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 24px;
    color: var(--fg-dim);
    background: var(--view-bg);
  }
</style>
