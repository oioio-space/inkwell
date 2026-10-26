<script lang="ts">
import { Stream } from "@wailsio/runtime";
import { onMount } from "svelte";
import { t } from "../i18n";
import type { SocketLike } from "../net/WailsNetworkAdapter";
import Icon from "../ui/icons/Icon.svelte";
import { toggleFullscreen } from "./fullscreen";
import { PublicModel } from "./public.svelte";
import SlideStage from "./SlideStage.svelte";

/**
 * The public window: the slide and nothing else. It is the page that opens at `/?view=public`; it
 * holds no document and no stream but `present`, and the presenter window sends it only the page
 * and the view. The person drags it to the screen they want and presses F11 (or the button that
 * shows while the mouse moves); Esc leaves full screen.
 */
const model = new PublicModel();
let full = $state(false);
let reveal = $state(false);
let hide: ReturnType<typeof setTimeout> | undefined;
let leftFullscreenAt = 0;

onMount(() => {
  document.title = t("present.public.title");
  model.start(() => Stream("present") as unknown as SocketLike);
  const changed = () => {
    full = document.fullscreenElement !== null;
    if (!full) leftFullscreenAt = Date.now();
  };
  document.addEventListener("fullscreenchange", changed);
  return () => {
    document.removeEventListener("fullscreenchange", changed);
    clearTimeout(hide);
    model.stop();
  };
});

async function toggle() {
  full = await toggleFullscreen(document);
}

function onKey(e: KeyboardEvent) {
  if (e.key === "F11") {
    e.preventDefault();
    void toggle();
    return;
  }
  // Esc that has just left full screen is not "end the presentation".
  if (e.key === "Escape" && (full || Date.now() - leftFullscreenAt < 400)) return;
  if (model.key(e)) e.preventDefault();
}

/** The button shows while the mouse moves, then goes: at rest the window is the slide alone. */
function moved() {
  reveal = true;
  clearTimeout(hide);
  hide = setTimeout(() => (reveal = false), 2500);
}
</script>

<svelte:window onkeydown={onKey} />

<main class="public" data-testid="public-view" onpointermove={moved}>
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
  {#if reveal}
    <button
      type="button"
      class="fs"
      data-testid="public-fullscreen"
      aria-label={t(full ? "present.fullscreen.exit" : "present.fullscreen")}
      title={t(full ? "present.fullscreen.exit" : "present.fullscreen")}
      onclick={toggle}
    >
      <Icon name="fullscreen" />
    </button>
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
  }
  .public:not(:hover) {
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
  .fs {
    position: absolute;
    right: 12px;
    top: 12px;
    display: grid;
    place-items: center;
    width: 44px;
    height: 44px;
    border: 0;
    border-radius: 999px;
    background: rgb(0 0 0 / 55%);
    color: rgb(255 255 255 / 90%);
  }
  .fs:hover {
    background: rgb(0 0 0 / 75%);
  }
</style>
