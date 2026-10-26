<script lang="ts">
import { untrack } from "svelte";
import { preview } from "../preview/preview.svelte";
import { workspace } from "../workspace/workspace.svelte";
import PresenterView from "./PresenterView.svelte";
import PublicOverlay from "./PublicOverlay.svelte";
import { presentSession as session } from "./session.svelte";

/**
 * Everything that makes a presentation run in this window: the keys, the pages asked of Go, the
 * feed of the public window, the links that go down. It draws the presenter's view (or the bare
 * slide, when this machine is the audience view) over the editor.
 */
let { onExit }: { onExit: (page: number) => void } = $props();

const p = $derived(session.current);

// What Go renders is the slide in view and the next ones: a step is on screen before it is asked.
$effect(() => {
  if (!p) return;
  const w = p.wantedPages();
  preview.setVisible(w.first, w.last);
});

// "Présenter depuis cette diapositive": the page is turned into a slide as soon as the deck is known.
$effect(() => {
  if (p?.pendingPage !== undefined && preview.deck) p.resolvePage();
});

// Rights change (a writer demoted, a member suspended): whoever may no longer pilot loses the controls.
$effect(() => {
  void workspace.rolesVersion;
  untrack(() => p?.revalidate());
});

// The public window shows what this one shows.
$effect(() => {
  if (p) session.feed(p);
});

// A presentation that ended by itself (Esc, a command from the public window) closes here too.
$effect(() => {
  if (p && p.mode === "off") onExit(session.stop());
});

// A link that goes down: if it was the pilot's, the slide stays and says so.
let known: string[] = [];
$effect(() => {
  const now = [...workspace.peers];
  untrack(() => {
    for (const gone of known) if (!now.includes(gone)) p?.peerGone(gone);
    known = now;
  });
});

function onKey(e: KeyboardEvent) {
  if (!p || p.mode === "off") return;
  if (p.key(e)) {
    e.preventDefault();
    e.stopPropagation();
  }
}
</script>

<svelte:window onkeydowncapture={onKey} />

{#if p}
  {#if p.mode === "public"}
    <PublicOverlay {p} pages={preview.pages} />
  {:else}
    <PresenterView
      {p}
      pages={preview.pages}
      notesSize={session.notesSize}
      publicOpen={session.publicOpen}
      onExit={() => p.stop()}
      onTogglePublic={() => void (session.publicOpen ? session.closePublic() : session.openPublic())}
      onNotesSize={(n) => session.setNotesSize(n)}
    />
  {/if}
{/if}
