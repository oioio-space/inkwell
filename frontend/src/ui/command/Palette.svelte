<script lang="ts">
import { onMount, tick, untrack } from "svelte";
import {
  announce,
  commandRegistry,
  labelOf,
  run,
  runnableOn,
  shortcutOf,
} from "../../extensions/commands";
import type { Command } from "../../extensions/registry";
import { lang, type MessageKey, t, tn } from "../../i18n";
import { composing, giveFocusBack, runOwnChord, sayLater } from "./dialogKeys";
import type { DialogProps } from "./dialogs.svelte";
import { paletteEntries } from "./entries";
import { query, rank } from "./rank";
import { readRecents, recordRecent } from "./recent";

/**
 * The command palette (spec §4.24, plan docs/plans/2026-11-25-ex0b2-palette.md): an APG editable
 * combobox over the registry's commands, ranked by `rank.ts`. Availability is read once, against
 * the target, when it opens; Enter gives the focus back to the target, then runs the command there.
 */
let { target, again, start, onclose }: DialogProps = $props();

/** Rows drawn at once around the active one, and how far PageUp and PageDown move. */
const WINDOW = 50;
const PAGE = 10;

let dialog: HTMLDialogElement | undefined = $state();
let field: HTMLInputElement | undefined = $state();
let text = $state("");
let active = $state(0);
let said = $state("");

/** The target does not change while the palette is open. */
const goal = untrack(() => target);
const registry = commandRegistry();
const entries = registry ? paletteEntries(registry, lang) : undefined;
// Read once per opening, never per keystroke (P-5).
const avail = new Map<string, { ok: boolean; reason: string }>();
const runnableHere = runnableOn(goal);
for (const e of entries?.entries ?? []) avail.set(e.item.id, runnableHere(e.item.id));
const recents = new Map(
  readRecents((id) => registry?.command(id) !== undefined).map((id, i) => [id, i]),
);

const order: Command[] = $derived(
  entries
    ? rank(entries, query(text), {
        available: (c) => avail.get(c.id)?.ok === true,
        recency: (c) => recents.get(c.id) ?? -1,
      })
    : [],
);
const first = $derived(
  Math.min(Math.max(0, active - WINDOW / 2), Math.max(0, order.length - WINDOW)),
);
const rows = $derived(order.slice(first, first + WINDOW));
const current = $derived(order[active]);
const optionId = (c: Command) => `palette-${c.id}`;

/** Runs after the frame that shows what changed (PERF-PALETTE marks, e2e builds only). */
function measure(name: string, from: number): void {
  if (import.meta.env.VITE_E2E_OBSERVE !== "1") return;
  requestAnimationFrame(() => setTimeout(() => performance.measure(name, { start: from }), 0));
}

onMount(() => {
  dialog?.showModal();
  measure("ex0b2:palette-open", start);
});

// The palette's own command again: the field takes the focus and its text is selected.
$effect(() => {
  void again;
  untrack(() => {
    field?.focus();
    field?.select();
  });
});

// The count, once the person stops typing.
$effect(() => {
  const n = order.length;
  return sayLater((s) => (said = s), n === 0 ? t("palette.none") : tn("palette.count", n));
});

// The active row stays in view as the keys move it.
$effect(() => {
  const id = current ? optionId(current) : undefined;
  untrack(() => {
    if (id) document.getElementById(id)?.scrollIntoView?.({ block: "nearest" });
  });
});

let closed = false;
/** Closes, and gives the focus back to the target (the editor when there is none). */
function finish(): void {
  if (closed) return;
  closed = true;
  dialog?.close();
  giveFocusBack(goal);
  onclose();
}

function choose(c: Command | undefined): void {
  if (!c) return;
  const s = avail.get(c.id);
  if (!s?.ok) {
    // Said by the application, and in the dialog's own status: the rest of the page is inert.
    // Emptied first, so that the same reason twice is said twice.
    const reason = s?.reason ?? t("cmd.unavailable.here");
    said = "";
    void tick().then(() => (said = reason));
    announce(reason);
    return;
  }
  finish();
  recordRecent(c.id);
  run(c.id, goal);
}

function onKey(e: KeyboardEvent): void {
  if (composing(e)) return;
  const last = order.length - 1;
  const moves: Record<string, number> = {
    ArrowDown: Math.min(last, active + 1),
    ArrowUp: Math.max(0, active - 1),
    PageDown: Math.min(last, active + PAGE),
    PageUp: Math.max(0, active - PAGE),
  };
  const to = moves[e.key];
  if (to !== undefined) {
    e.preventDefault();
    active = Math.max(0, to);
  } else if (e.key === "Enter") {
    e.preventDefault();
    choose(current);
  } else if (e.key === "Escape") {
    e.preventDefault();
    finish();
  } else runOwnChord(e);
}
</script>

<dialog
  bind:this={dialog}
  class="palette"
  aria-label={t("palette.label")}
  oncancel={(e) => {
    e.preventDefault();
    finish();
  }}
>
  <input
    bind:this={field}
    class="field"
    type="text"
    role="combobox"
    aria-expanded="true"
    aria-controls="palette-list"
    aria-autocomplete="list"
    aria-activedescendant={current ? optionId(current) : undefined}
    aria-label={t("palette.label")}
    placeholder={t("palette.label")}
    autocomplete="off"
    spellcheck="false"
    value={text}
    oninput={(e) => {
      const at = performance.now();
      text = e.currentTarget.value;
      active = 0;
      measure("ex0b2:palette-filter", at);
    }}
    onkeydown={onKey}
  />
  <div id="palette-list" class="list" role="listbox" aria-label={t("palette.list")}>
    {#each rows as c, i (c.id)}
      {@const s = avail.get(c.id)}
      {@const chord = shortcutOf(c.id)}
      <!-- APG combobox: the options take no focus (aria-activedescendant) and the field's keys drive them. -->
      <!-- svelte-ignore a11y_interactive_supports_focus, a11y_click_events_have_key_events -->
      <div
        id={optionId(c)}
        class="row"
        role="option"
        aria-selected={first + i === active}
        aria-disabled={s?.ok ? undefined : "true"}
        aria-describedby={s?.ok ? undefined : `${optionId(c)}-why`}
        onmousedown={(e) => e.preventDefault()}
        onclick={() => {
          active = first + i;
          choose(c);
        }}
      >
        <span class="text">
          <span class="name">{labelOf(c)}</span>
          {#if !s?.ok}<span class="sub" id="{optionId(c)}-why">{s?.reason}</span>{/if}
        </span>
        {#if chord}<span class="k">{chord}</span>{/if}
        <span class="cat">{t(`cmd.category.${c.category}` as MessageKey)}</span>
      </div>
    {/each}
  </div>
  <div class="status" role="status">{said}</div>
</dialog>

<style>
  dialog {
    box-sizing: border-box;
    width: min(520px, calc(100vw - 32px));
    max-height: min(560px, calc(100vh - 96px));
    margin: 64px auto auto;
    padding: 12px;
    border: 0;
    border-radius: var(--radius-window);
    background: var(--window-bg);
    color: var(--fg);
    box-shadow:
      0 0 0 1px var(--border),
      0 12px 40px rgb(0 0 0 / 30%);
  }
  dialog[open] {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 30%);
  }
  .field {
    box-sizing: border-box;
    width: 100%;
    min-height: var(--target-min);
    padding: 0 12px;
    border: 0;
    border-radius: var(--radius-button);
    background: var(--btn-bg);
    color: var(--fg);
    font: inherit;
  }
  .field:focus {
    outline: 2px solid var(--focus-ring);
    outline-offset: -2px;
  }
  .list {
    overflow-y: auto;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 40px;
    padding: 4px 12px;
    border-radius: var(--radius-small);
    cursor: default;
  }
  .row:hover {
    background: var(--hover-bg);
  }
  .row[aria-selected="true"] {
    background: var(--selected-bg);
  }
  .row[aria-disabled="true"] .name {
    opacity: 0.55;
  }
  .text {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-width: 0;
  }
  .k,
  .cat {
    color: var(--fg-dim);
    font-size: 13px;
    white-space: nowrap;
  }
  .status {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
</style>
