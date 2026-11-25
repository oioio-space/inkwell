<script lang="ts">
import { tick } from "svelte";
import { markOpen } from "../../../../perf/openMarks";
import { session } from "../../../../ui/editorSession.svelte";
import Sidebar from "../../../../ui/shell/Sidebar.svelte";
import { viewChoice } from "../../../../workspace/viewChoice.svelte";
import { workspace } from "../../../../workspace/workspace.svelte";

/**
 * The left side's panel `app.files` (EX0c2): the project's file tree. A file opened from it takes
 * the keyboard, unless it was opened with Space (the focus stays on the tree), as before.
 */
function open(path: string, keepFocus = false) {
  markOpen(path);
  viewChoice.arrive();
  void workspace
    .openFile(path)
    .then(() => tick())
    .then(() => {
      if (keepFocus) return;
      if (session.view) session.focus();
      else document.querySelector<HTMLElement>(".file-view, [role=grid]")?.focus();
    });
}
</script>

<Sidebar files={workspace.files} current={workspace.current} onOpen={open} />
