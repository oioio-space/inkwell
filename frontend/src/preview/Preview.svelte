<script lang="ts">
import { onDestroy } from "svelte";
import { t } from "../i18n";
import Icon from "../ui/icons/Icon.svelte";
import {
  clampZoom,
  currentPage,
  fitWidth,
  PX_PER_PT,
  scrollTarget,
  stack,
  stepZoom,
  visiblePages,
} from "./layout";
import { preview } from "./preview.svelte";

/**
 * The right-hand pane: the pages of the compiled document, drawn lazily as they come into view.
 * Each page is an image (see svg.ts), never markup. A click on a page asks the shell to put the
 * caret where that page's text comes from.
 */
let {
  onSourceJump,
  onCursorJump,
}: {
  /** The person clicked at (x, y) points of `page`. */
  onSourceJump: (page: number, x: number, y: number) => void;
  /** The person asked for the page of the caret. */
  onCursorJump: () => void;
} = $props();

const PAD = 16;
const A4 = { width: 595.28, height: 841.89 };

let scroller: HTMLDivElement | undefined = $state();
let viewW = $state(0);
let viewH = $state(0);
let scrollTop = $state(0);
let mode = $state<"fit" | number>("fit");
let markShown = $state(0);

const known = $derived(preview.pages.get(1) ?? [...preview.pages.values()][0]);
const fallback = $derived(known ? { width: known.width, height: known.height } : A4);
const zoom = $derived(
  mode === "fit" ? fitWidth(Math.max(0, viewW - 2 * PAD), fallback.width) : clampZoom(mode),
);
const scale = $derived(zoom * PX_PER_PT);
const layout = $derived(stack(preview.pageCount, preview.box, fallback, zoom));
const widest = $derived(
  Math.max(fallback.width, ...[...preview.pages.values()].map((p) => p.width)) * scale,
);
const stackW = $derived(Math.max(viewW, widest + 2 * PAD));
const rendered = $derived(visiblePages(layout, scrollTop, viewH, viewH));
const shown = $derived(visiblePages(layout, scrollTop, viewH));
const numbers = $derived(
  rendered
    ? Array.from({ length: rendered.last - rendered.first + 1 }, (_, i) => rendered.first + i)
    : [],
);
const percent = $derived(Math.round(zoom * 100));
const showStale = $derived(preview.failure !== undefined && preview.pageCount > 0);

// What is in view decides what Go renders; what fills the view is the current page.
$effect(() => {
  if (shown) preview.setVisible(shown.first, shown.last);
});
$effect(() => {
  preview.currentPage = preview.pageCount > 0 ? currentPage(layout, scrollTop, viewH) : 0;
});
onDestroy(() => {
  preview.currentPage = 0;
});

// A jump from the source: scroll to the place and flash a mark there.
$effect(() => {
  const m = preview.marker;
  if (!m || !scroller) return;
  const top = scrollTarget(layout, m.page, m.y, zoom, viewH);
  const smooth = !matchMedia("(prefers-reduced-motion: reduce)").matches;
  scroller.scrollTo({ top, behavior: smooth ? "smooth" : "auto" });
  markShown = m.id;
  const timer = setTimeout(() => (markShown = 0), 1600);
  return () => clearTimeout(timer);
});

function onScroll(e: Event) {
  scrollTop = (e.currentTarget as HTMLElement).scrollTop;
}

function setZoom(z: number | "fit") {
  mode = z === "fit" ? "fit" : clampZoom(z);
}

function pageClick(e: MouseEvent, n: number) {
  if (e.detail === 0) return onSourceJump(n, 0, 0); // the keyboard: the top of the page
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
  const page = preview.pages.get(n);
  if (!page || rect.width === 0) return;
  const k = rect.width / page.width; // CSS pixels per point
  onSourceJump(n, (e.clientX - rect.left) / k, (e.clientY - rect.top) / k);
}

const packages = $derived(preview.failure?.packages.join(", ") ?? "");
</script>

<section class="preview" aria-label={t("preview.label")}>
  <div class="tools" role="group" aria-label={t("preview.tools")}>
    <button type="button" class="fb" aria-label={t("preview.zoomOut")} title={t("preview.zoomOut")} onclick={() => setZoom(stepZoom(zoom, -1))}>
      <Icon name="minus" />
    </button>
    <button
      type="button"
      class="fb lbl zoom"
      aria-label={t("preview.zoomFit")}
      title={t("preview.zoomFit")}
      aria-pressed={mode === "fit"}
      onclick={() => setZoom("fit")}
    >
      <span data-testid="zoom">{percent} %</span>
    </button>
    <button type="button" class="fb" aria-label={t("preview.zoomIn")} title={t("preview.zoomIn")} onclick={() => setZoom(stepZoom(zoom, 1))}>
      <Icon name="plus" />
    </button>
    <button
      type="button"
      class="fb"
      aria-label={t("preview.jumpToCursor")}
      title={t("preview.jumpToCursor")}
      disabled={preview.pageCount === 0}
      onclick={onCursorJump}
    >
      <Icon name="target" />
    </button>
    <span class="sp"></span>
    {#if preview.compiling}
      <span class="busy" aria-live="polite">{t("preview.compiling")}</span>
    {/if}
  </div>

  {#if showStale}
    <p class="notice" aria-live="polite">
      <Icon name="warn" />{preview.failure?.kind === "errors"
        ? t("preview.stale")
        : t(`preview.fail.${preview.failure?.kind ?? "internal"}`, { packages })}
    </p>
  {/if}

  {#if preview.pageCount > 0}
    <!-- A scrollable region must take focus, or the keyboard cannot scroll it. -->
    <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
    <div
      class="scroller"
      bind:this={scroller}
      bind:clientWidth={viewW}
      bind:clientHeight={viewH}
      onscroll={onScroll}
      role="group"
      aria-label={t("preview.pages")}
      tabindex="0"
      data-testid="pages"
    >
      <div class="stack" style:width="{stackW}px" style:height="{layout.total + 2 * PAD}px">
        {#each numbers as n (n)}
          {@const page = preview.pages.get(n)}
          {@const box = preview.box(n) ?? fallback}
          <div
            class="page"
            data-page={n}
            style:top="{(layout.tops[n - 1] ?? 0) + PAD}px"
            style:left="{(stackW - box.width * scale) / 2}px"
            style:width="{box.width * scale}px"
            style:height="{box.height * scale}px"
          >
            {#if page?.src}
              <button type="button" class="pagebtn" onclick={(e) => pageClick(e, n)}>
                <img
                  src={page.src}
                  alt={t("preview.pageAlt", { page: n, count: preview.pageCount })}
                  draggable="false"
                />
              </button>
            {:else if page}
              <span class="blank">{t("preview.pageUnavailable")}</span>
            {:else}
              <span class="blank skeleton" aria-hidden="true"></span>
            {/if}
            {#if preview.marker && preview.marker.page === n && markShown === preview.marker.id}
              <span class="mark" style:top="{preview.marker.y * scale}px" aria-hidden="true"></span>
            {/if}
          </div>
        {/each}
      </div>
    </div>
  {:else}
    <div class="empty" aria-live="polite" data-testid="preview-empty">
      {#if !preview.hasResult}
        <p>{t("preview.starting")}</p>
      {:else if preview.failure}
        <p>
          {t(`preview.fail.${preview.failure.kind}`, { packages })}
        </p>
      {/if}
    </div>
  {/if}
</section>

<style>
  .preview {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-width: 0;
    background: var(--sidebar-bg);
    box-shadow: inset 1px 0 var(--card-shade);
  }
  .tools {
    display: flex;
    align-items: center;
    gap: 2px;
    flex: none;
    height: 40px;
    padding: 0 6px;
    background: var(--headerbar-bg);
    box-shadow: 0 1px var(--shade);
  }
  .tools .sp {
    flex: 1;
  }
  .zoom {
    min-width: 64px;
    font-variant-numeric: tabular-nums;
    font-size: 12.5px;
  }
  .busy {
    padding: 0 10px;
    color: var(--fg-dim);
    font-size: 12.5px;
  }
  .notice {
    display: flex;
    align-items: center;
    gap: 8px;
    margin: 0;
    padding: 8px 12px;
    background: var(--warning-bg);
    color: var(--warning);
    font-size: 13px;
  }
  .scroller {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
  .scroller:focus-visible {
    outline: var(--focus-ring);
    outline-offset: -2px;
  }
  .stack {
    position: relative;
  }
  .page {
    position: absolute;
    background: #fff;
    box-shadow:
      0 0 0 1px var(--shade),
      0 2px 8px rgb(0 0 6 / 18%);
  }
  .pagebtn {
    display: block;
    width: 100%;
    height: 100%;
    padding: 0;
    border: 0;
    background: none;
    cursor: pointer;
  }
  .pagebtn:focus-visible {
    outline: var(--focus-ring);
    outline-offset: 2px;
  }
  .pagebtn img {
    display: block;
    width: 100%;
    height: 100%;
    user-select: none;
  }
  .blank {
    display: grid;
    place-items: center;
    height: 100%;
    padding: 12px;
    color: #55555a;
    font-size: 13px;
    text-align: center;
  }
  .skeleton {
    background: linear-gradient(90deg, #f4f4f6, #fafafb, #f4f4f6);
    background-size: 200% 100%;
    animation: shimmer 1.4s linear infinite;
  }
  @keyframes shimmer {
    to {
      background-position: -200% 0;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .skeleton {
      animation: none;
    }
  }
  .mark {
    position: absolute;
    inset-inline: 0;
    height: 0;
    border-top: 3px solid var(--accent-bg);
    background: color-mix(in srgb, var(--accent-bg) 18%, transparent);
    pointer-events: none;
  }
  .empty {
    display: grid;
    flex: 1;
    place-items: center;
    padding: 24px;
    color: var(--fg-dim);
    text-align: center;
  }
  .empty p {
    max-width: 34ch;
    margin: 0;
    line-height: 1.45;
  }
</style>
