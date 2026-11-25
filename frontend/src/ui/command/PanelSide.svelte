<script lang="ts">
import { panelsOf, returnTarget } from "../../extensions/commands";
import { SIDES, type Side } from "../../extensions/manifest";
import { type MessageKey, t } from "../../i18n";
import { selectedOf, selection } from "./panelSelection.svelte";
import { bodyOf, loadSide, panelFailed, sideReady } from "./panels.svelte";

/**
 * A side of the window drawn from the `panels` point (EX0c2, plan §1): an `aside` named by its
 * side. One panel shows its body alone; two or more get a tab list as the WAI-ARIA tabs pattern
 * has it (roving focus, Left and Right wrap, Home and End, automatic activation), every panel
 * mounted once loaded and the others hidden, so each tab's `aria-controls` names an element. The
 * side draws nothing until its bodies are loaded. Opening it leaves the focus where it was; closing
 * it while it holds the focus gives the focus back to where the person was, never to the page.
 */
let { side }: { side: Side } = $props();

const shown = $derived(panelsOf(side));
const ready = $derived(sideReady(side));
const current = $derived(selectedOf(side, shown));
const label = $derived(t(SIDES[side].label as MessageKey));
const slug = (id: string) => `panel-${id.replace(/\./g, "-")}`;

$effect(() => {
  if (!ready) void loadSide(side);
});

let aside: HTMLElement | undefined = $state();
let tabs: HTMLElement | undefined = $state();
let holdsFocus = false;

$effect(() => {
  return () => {
    // Closing a side that holds the focus: the focus goes back where the person was (P-5). A
    // browser may blur the focused tab as the side leaves the page: the page then has the focus.
    const active = document.activeElement;
    const lost = holdsFocus || active === null || active === document.body;
    if (!lost) return;
    const back =
      returnTarget() ?? document.querySelector<HTMLElement>(".cm-content, [role=grid], .file-view");
    if (back instanceof HTMLElement) back.focus();
  };
});

function choose(id: string, focus: boolean) {
  selection[side] = id;
  if (focus) tabs?.querySelector<HTMLElement>(`#tab-${slug(id)}`)?.focus();
}

/** The tab a key goes to (APG tabs): Left and Right wrap, Home and End go to the ends. */
const MOVES: Record<string, (at: number, n: number) => number> = {
  ArrowRight: (at, n) => (at + 1) % n,
  ArrowLeft: (at, n) => (at - 1 + n) % n,
  Home: () => 0,
  End: (_, n) => n - 1,
};

function keys(e: KeyboardEvent) {
  const move = MOVES[e.key];
  if (!move) return;
  const next =
    shown[
      move(
        shown.findIndex((p) => p.id === current?.id),
        shown.length,
      )
    ];
  if (!next) return;
  e.preventDefault();
  choose(next.id, true);
}

/** A panel whose first content takes no focus is itself in the tab sequence (APG). */
function reachable(node: HTMLElement) {
  const update = () => {
    const focusable = node.querySelector(
      'button, input, select, textarea, a[href], [tabindex]:not([tabindex="-1"])',
    );
    if (focusable) node.removeAttribute("tabindex");
    else node.setAttribute("tabindex", "0");
  };
  update();
  const observer = new MutationObserver(update);
  // The body's own children arrive once it is mounted: a change inside a list is no new first child.
  observer.observe(node, { childList: true });
  return { destroy: () => observer.disconnect() };
}
</script>

{#if ready}
  <aside
    bind:this={aside}
    class={side === "end" ? "drawer" : "side"}
    aria-label={label}
    data-testid={side === "end" ? "history-drawer" : undefined}
    data-side={side}
    onfocusin={() => (holdsFocus = true)}
    onfocusout={(e) => (holdsFocus = !!aside && aside.contains(e.relatedTarget as Node | null))}
  >
    {#if shown.length === 1 && shown[0]}
      {@const Body = bodyOf(shown[0])}
      {#if Body}<Body />{:else if panelFailed(shown[0])}<p class="failed" role="status">{t("panel.unavailable")}</p>{/if}
    {:else}
      <div class="tg tabs" role="tablist" aria-label={label} bind:this={tabs}>
        {#each shown as p (p.id)}
          {@const on = p.id === current?.id}
          <button
            type="button"
            role="tab"
            id="tab-{slug(p.id)}"
            aria-selected={on}
            aria-controls={slug(p.id)}
            tabindex={on ? 0 : -1}
            data-panel={p.id}
            onclick={() => choose(p.id, false)}
            onkeydown={keys}
          >
            {t(p.label as MessageKey)}
          </button>
        {/each}
      </div>
      {#each shown as p (p.id)}
        {@const Body = bodyOf(p)}
        <div class="panel" role="tabpanel" id={slug(p.id)} aria-labelledby="tab-{slug(p.id)}" hidden={p.id !== current?.id} use:reachable>
          {#if Body}<Body />{:else if panelFailed(p)}<p class="failed" role="status">{t("panel.unavailable")}</p>{/if}
        </div>
      {/each}
    {/if}
  </aside>
{/if}

<style>
  .side {
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    width: 264px;
    min-height: 0;
    background: var(--sidebar-bg);
    box-shadow: inset -1px 0 var(--card-shade);
  }
  .drawer {
    box-sizing: border-box;
    display: flex;
    flex: none;
    flex-direction: column;
    gap: 8px;
    width: min(380px, 42vw);
    min-height: 0;
    padding: 10px 12px;
    overflow: hidden;
    background: var(--sidebar-bg);
    box-shadow: -1px 0 var(--shade);
  }
  .tabs {
    align-self: center;
  }
  .tabs button[aria-selected="true"] {
    background: var(--popover-bg);
    box-shadow: 0 1px 2px rgb(0 0 6 / 25%);
  }
  .panel {
    display: flex;
    flex: 1 1 auto;
    flex-direction: column;
    gap: 8px;
    min-height: 0;
  }
  .panel[hidden] {
    display: none;
  }
  .failed {
    margin: 0;
    color: var(--fg-dim);
    font-size: 12.5px;
  }
</style>
