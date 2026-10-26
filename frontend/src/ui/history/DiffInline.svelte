<script lang="ts">
import type { Frag, Hunk } from "../../history/diffview";
import { layoutDiff } from "../../history/layout";
import type { Author } from "../../history/people";
import { t, tn } from "../../i18n";

/**
 * Changes at the word: added text on pale green, removed text struck on pale red, a moved block
 * with a label, the letters that really differ underlined. Each run of changes names who made it.
 */
let {
  frags,
  hunks,
  current = -1,
  author,
  when,
  moves,
  showAuthors = false,
  label,
}: {
  frags: readonly Frag[];
  hunks: readonly Hunk[];
  /** The run F7 is on. */
  current?: number;
  author: (actor: string) => Author;
  /** When a stretch's session was, in words; undefined if unknown. */
  when?: (frag: Frag) => string | undefined;
  /** Where each moved block left and went, by section title. */
  moves?: ReadonlyMap<number, { from: string; to: string }>;
  /** Name the author before every run (a comparison), not only in the header (one entry). */
  showAuthors?: boolean;
  label: string;
} = $props();

const items = $derived(layoutDiff(frags, hunks));

/** The text of a stretch cut at its underlined letters. */
function parts(f: Frag): { text: string; strong: boolean }[] {
  if (!f.inner) return [{ text: f.text, strong: false }];
  const [a, b] = f.inner;
  return [
    { text: f.text.slice(0, a), strong: false },
    { text: f.text.slice(a, b), strong: true },
    { text: f.text.slice(b), strong: false },
  ].filter((p) => p.text !== "");
}

function movedTitle(f: Frag): string {
  const m = f.moved === undefined ? undefined : moves?.get(f.moved);
  if (!m?.from && !m?.to) return t("diff.moved");
  return t("diff.moved.from", { from: m?.from || "…", to: m?.to || "…" });
}

/** The people of a run, once each. */
function who(index: number): Author[] {
  const h = hunks[index];
  return (h?.actors ?? []).map((a) => author(a));
}
</script>

<div class="diff" role="document" aria-label={label} data-zoomable>
  {#each items as item}
    {#if item.t === "eq"}
      <span class="eq">{item.text}</span>
    {:else if item.t === "gap"}
      <span class="gap" role="separator">{tn("diff.gap", item.lines)}</span>
    {:else}
      <span class="hunk" class:current={item.index === current} data-hunk={item.index}>
        {#if showAuthors}
          {#each who(item.index) as a}
            <span class="chip" style:--who="var(--person-{a.index})" title={a.name}>
              <span class="dot" aria-hidden="true"></span>{a.name}
            </span>
          {/each}
        {/if}
        {#each item.frags as f}
          {#if f.kind === "eq"}
            <span class="eq">{f.text}</span>
          {:else}
            <svelte:element
              this={f.kind === "add" ? "ins" : "del"}
              class={f.kind}
              class:moved={f.moved !== undefined}
              title={f.moved !== undefined ? movedTitle(f) : f.actor ? `${author(f.actor).name}${when?.(f) ? ` · ${when(f)}` : ""}` : undefined}
            >
              {#if f.moved !== undefined}<span class="badge">{t("diff.moved")}</span>{/if}
              {#each parts(f) as p}{#if p.strong}<u>{p.text}</u>{:else}{p.text}{/if}{/each}
            </svelte:element>
          {/if}
        {/each}
      </span>
    {/if}
  {/each}
</div>

<style>
  .diff {
    box-sizing: border-box;
    padding: 16px 20px;
    font: calc(15px * var(--text-zoom, 1)) / 1.75 var(--font-mono);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    color: var(--fg);
  }
  .gap {
    display: block;
    margin: 6px 0;
    padding: 2px 10px;
    border-radius: var(--radius-small);
    background: var(--hover-bg);
    color: var(--fg-dim);
    font: 12.5px/1.6 var(--font-ui);
    text-align: center;
    user-select: none;
  }
  ins,
  del {
    border-radius: 3px;
    padding: 0 1px;
    text-decoration: none;
  }
  ins.add {
    background: var(--diff-add-bg);
    color: var(--diff-add-fg);
  }
  del.del {
    background: var(--diff-del-bg);
    color: var(--diff-del-fg);
    text-decoration: line-through;
    text-decoration-thickness: 1.5px;
  }
  u {
    text-decoration: underline;
    text-decoration-thickness: 2px;
    text-underline-offset: 3px;
    font-weight: 700;
  }
  del u {
    text-decoration: underline line-through;
  }
  .moved {
    box-shadow: 0 0 0 1px currentColor inset;
  }
  .badge {
    margin-right: 6px;
    padding: 0 6px;
    border-radius: 99px;
    background: var(--card-bg);
    color: var(--fg);
    font: 700 11px/1.6 var(--font-ui);
    text-decoration: none;
  }
  .hunk.current {
    outline: 2px solid var(--accent-bg);
    outline-offset: 2px;
    border-radius: 4px;
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    margin-right: 6px;
    padding: 0 8px 0 6px;
    border-radius: 99px;
    background: color-mix(in srgb, var(--who) 22%, var(--view-bg));
    color: var(--fg);
    font: 600 11.5px/1.7 var(--font-ui);
    vertical-align: 1px;
    white-space: nowrap;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--who);
  }
</style>
