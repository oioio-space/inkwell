<script module lang="ts">
import type { Component } from "svelte";
import type { FacadeProps, NativeViewerProps } from "./context";
import type { Viewer } from "./registry";

/** What mounts: the viewer that shows (the one asked for, or its stand-in) and its component. */
type Shown = { viewer: Viewer; component: Component<NativeViewerProps | FacadeProps> };

/** The viewers already loaded, by the viewer asked for: shown at once, without a frame of nothing. */
const ready = new Map<Viewer, Shown>();
</script>

<script lang="ts">
import { t } from "../i18n";
import { notice } from "../ui/notice.svelte";
import { type ViewerModule, viewerProps } from "./context";
import { registry } from "./index";

/**
 * Shows a file in `viewer`, the default of its opening or one of its alternatives. The viewer's
 * code is loaded on first use; one that fails to load is replaced by the essential viewer of the
 * file's content, and a toast says so once. What the viewer receives is decided here, from its
 * capabilities: native props for `ui.native`, the facade for any other.
 */
let { viewer, ...props }: NativeViewerProps & { viewer: Viewer } = $props();
let shown = $state.raw<Shown | undefined>();

// Before the page draws: a viewer already loaded shows in the same frame.
$effect.pre(() => {
  const v = viewer;
  const opening = props.opening;
  shown = ready.get(v);
  if (shown) return;
  let live = true;
  registry
    .loadViewer({ ...opening, viewer: v }, () => notice.queue(t("viewer.failed"), "status"))
    .then(({ viewer: got, module }) => {
      const component = (module as ViewerModule).component as Shown["component"];
      const next = { viewer: got, component };
      ready.set(v, next);
      if (live) shown = next;
    })
    .catch((error: unknown) => console.error("viewer:", error instanceof Error ? error.message : error));
  return () => {
    live = false;
  };
});
</script>

{#if shown}
  {@const View = shown.component}
  <View {...viewerProps(shown.viewer, props)} />
{/if}
