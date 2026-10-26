<script lang="ts">
import { Stream } from "@wailsio/runtime";
import { onMount } from "svelte";
import { t } from "../i18n";
import type { SocketLike } from "../net/WailsNetworkAdapter";
import { PublicModel } from "./public.svelte";
import SlideStage from "./SlideStage.svelte";

/**
 * The public window: the slide, fullscreen, and nothing else. It is the page that opens at
 * `/?view=public`; it holds no document and no stream but `present`, and the presenter window
 * sends it only the page and the view.
 */
const model = new PublicModel();

onMount(() => {
  document.title = t("present.public.title");
  model.start(() => Stream("present") as unknown as SocketLike);
  return () => model.stop();
});

function onKey(e: KeyboardEvent) {
  if (model.key(e)) e.preventDefault();
}
</script>

<svelte:window onkeydown={onKey} />

<main class="public" data-testid="public-view">
  <SlideStage
    src={model.current?.src}
    width={model.current?.width}
    height={model.current?.height}
    screen={model.screen}
    laser={model.laser}
    away={model.away}
    alt={t("present.slideAlt", { n: model.page })}
  />
  {#if model.page === 0}
    <p class="wait">{t("present.public.waiting")}</p>
  {/if}
</main>

<style>
  :global(html),
  :global(body) {
    margin: 0;
    background: #000;
  }
  .public {
    position: fixed;
    inset: 0;
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
