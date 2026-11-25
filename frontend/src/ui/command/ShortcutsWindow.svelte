<script lang="ts">
import { onMount, untrack } from "svelte";
import { lang, t, tn } from "../../i18n";
import Icon from "../icons/Icon.svelte";
import { composing, giveFocusBack, runOwnChord, sayLater } from "./dialogKeys";
import type { DialogProps } from "./dialogs.svelte";
import { classOf, prepare, query } from "./rank";
import { type ShortcutRow, shortcutSections } from "./shortcuts";

/**
 * The shortcuts window (spec §4.26, after libadwaita's AdwShortcutsDialog): the chords in force on
 * this system by category, then the writing keys, with a search that folds and matches as the
 * palette does. Escape gives the focus back to where it was.
 */
let { target, again, onclose }: DialogProps = $props();

let dialog: HTMLDialogElement | undefined = $state();
let field: HTMLInputElement | undefined = $state();
let text = $state("");
let said = $state("");

const goal = untrack(() => target);
const sections = shortcutSections();
const entries = prepare(
  sections.flatMap((s) => s.rows),
  (row) => row.label,
  (row) => row.keywords,
  lang,
);
const shown = $derived.by(() => {
  const q = query(text);
  if (q.s === "") return sections;
  const kept = new Set<ShortcutRow>(
    entries.entries.filter((e) => classOf(e, q) > 0).map((e) => e.item),
  );
  return sections
    .map((s) => ({ ...s, rows: s.rows.filter((row) => kept.has(row)) }))
    .filter((s) => s.rows.length > 0);
});

onMount(() => dialog?.showModal());

// What the search leaves, said once the typing stops (as the palette says its count).
$effect(() => {
  const n = shown.reduce((k, s) => k + s.rows.length, 0);
  return sayLater((x) => (said = x), n === 0 ? t("shortcuts.none") : tn("shortcuts.count", n));
});

$effect(() => {
  void again;
  untrack(() => {
    field?.focus();
    field?.select();
  });
});

let closed = false;
function finish(): void {
  if (closed) return;
  closed = true;
  dialog?.close();
  giveFocusBack(goal);
  onclose();
}

function onKey(e: KeyboardEvent): void {
  if (composing(e)) return;
  if (e.key === "Escape") {
    e.preventDefault();
    finish();
    return;
  }
  runOwnChord(e);
}
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<dialog
  bind:this={dialog}
  class="shortcuts"
  aria-labelledby="shortcuts-title"
  data-testid="shortcuts-window"
  oncancel={(e) => {
    e.preventDefault();
    finish();
  }}
  onkeydown={onKey}
>
  <header>
    <h2 id="shortcuts-title">{t("shortcuts.title")}</h2>
    <button type="button" class="close" aria-label={t("shortcuts.close")} onclick={finish}>
      <span class="disc"><Icon name="close" size={12} /></span>
    </button>
  </header>
  <input
    bind:this={field}
    class="field"
    type="search"
    aria-label={t("shortcuts.search")}
    placeholder={t("shortcuts.search")}
    autocomplete="off"
    spellcheck="false"
    bind:value={text}
  />
  <div class="status" role="status">{said}</div>
  <div class="body">
    {#each shown as s (s.id)}
      <section aria-labelledby="shortcuts-{s.id}">
        <h3 id="shortcuts-{s.id}">{s.title}</h3>
        <div class="card boxed">
          {#each s.rows as row (row.id)}
            <div class="row" data-shortcut={row.id}>
              <span class="text">
                <span>{row.label}</span>
                {#if row.marks.length}<span class="sub">{row.marks.join(" · ")}</span>{/if}
              </span>
              <span class="keys">
                {#each row.chords as chord (chord)}<kbd>{chord}</kbd>{/each}
              </span>
            </div>
          {/each}
        </div>
      </section>
    {:else}
      <p class="none">{t("shortcuts.none")}</p>
    {/each}
  </div>
</dialog>

<style>
  dialog {
    box-sizing: border-box;
    width: min(760px, calc(100vw - 32px));
    max-height: calc(100vh - 32px);
    padding: 0 22px 22px;
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
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 30%);
  }
  header {
    position: relative;
    display: grid;
    place-items: center;
    height: 56px;
    margin: 0 -22px;
  }
  h2 {
    margin: 0;
    font: 700 14.5px/1.3 var(--font-ui);
  }
  .close {
    position: absolute;
    inset-inline-end: 8px;
    display: grid;
    place-items: center;
    width: var(--target-min);
    height: var(--target-min);
    padding: 0;
    border: 0;
    background: transparent;
    color: var(--fg);
  }
  .disc {
    display: grid;
    place-items: center;
    width: 26px;
    height: 26px;
    border-radius: 50%;
    background: var(--btn-bg);
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
  .body {
    overflow-y: auto;
  }
  h3 {
    margin: 20px 4px 8px;
    font: 700 14.5px/1.3 var(--font-ui);
  }
  .row {
    justify-content: space-between;
  }
  .text {
    display: flex;
    flex-direction: column;
    line-height: 1.35;
  }
  .keys {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 6px;
  }
  kbd {
    padding: 2px 8px;
    border-radius: var(--radius-small);
    background: var(--btn-bg);
    font: 600 13px/1.4 var(--font-ui);
    white-space: nowrap;
  }
  .status {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
  .none {
    margin: 20px 4px;
    color: var(--fg-dim);
  }
</style>
