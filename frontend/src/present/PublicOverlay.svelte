<script lang="ts">
import { t } from "../i18n";
import type { Page } from "../preview/preview.svelte";
import type { Presentation } from "./present.svelte";
import SlideStage from "./SlideStage.svelte";

/**
 * This machine is the audience view (the one plugged to the projector): the slide, fullscreen,
 * driven by whoever pilots from another machine. No control is drawn; Esc leaves.
 */
let { p, pages }: { p: Presentation; pages: ReadonlyMap<number, Page> } = $props();

const here = $derived(pages.get(p.page));
</script>

<section class="overlay" aria-label={t("present.public.title")} data-testid="public-overlay">
  <SlideStage
    src={here?.src}
    width={here?.width}
    height={here?.height}
    screen={p.screen}
    laser={p.laser}
    away={p.away}
    alt={t("present.slideAlt", { n: p.index })}
  />
  {#if p.waiting}
    <p class="wait">{t("present.public.waiting")}</p>
  {/if}
</section>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    z-index: 60;
    background: #000;
    cursor: none;
  }
  .wait {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    margin: 0;
    color: rgb(255 255 255 / 55%);
    font: 15px var(--font-ui);
    pointer-events: none;
  }
</style>
