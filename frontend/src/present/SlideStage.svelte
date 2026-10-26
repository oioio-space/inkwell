<script lang="ts">
import { t } from "../i18n";
import type { ScreenMode } from "./wire";

/**
 * One slide on its own, as the audience sees it: the page image fitted to the room it has, or a
 * black or white screen. It holds no control and no text of the presenter (notes, comments), so
 * the same component serves the public window, the "next" thumbnail and the follower's view.
 */
let {
  src = "",
  width = 960,
  height = 540,
  screen = "show",
  laser = null,
  away = false,
  alt,
  onpointermove,
}: {
  /** A `data:` image of the page; empty while it has not arrived. */
  src?: string | undefined;
  width?: number | undefined;
  height?: number | undefined;
  screen?: ScreenMode;
  /** The laser pointer, as fractions of the slide. */
  laser?: { x: number; y: number } | null;
  /** The presenter is disconnected: a discreet mention is shown. */
  away?: boolean;
  alt: string;
  /** The mouse moved over the slide (fractions), for the laser of the presenter. */
  onpointermove?: ((x: number, y: number) => void) | undefined;
} = $props();

function move(e: PointerEvent) {
  if (!onpointermove) return;
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
  if (r.width > 0 && r.height > 0)
    onpointermove((e.clientX - r.left) / r.width, (e.clientY - r.top) / r.height);
}
</script>

<div class="stage" data-screen={screen} data-testid="stage">
  {#if screen === "show" && src}
    <div class="slide" style:--ratio={width / height} onpointermove={move} role="presentation">
      <img {src} {alt} draggable="false" />
      {#if laser}
        <span class="laser" data-testid="laser" style:left="{laser.x * 100}%" style:top="{laser.y * 100}%"></span>
      {/if}
    </div>
  {/if}
  {#if away}
    <span class="away" role="status" data-testid="away">{t("present.pilot.away")}</span>
  {/if}
</div>

<style>
  .stage {
    position: relative;
    display: grid;
    place-items: center;
    box-sizing: border-box;
    width: 100%;
    height: 100%;
    background: #000;
    container-type: size;
    overflow: hidden;
  }
  .stage[data-screen="white"] {
    background: #fff;
  }
  .stage[data-screen="black"] {
    background: #000;
  }
  .slide {
    position: relative;
    width: min(100cqw, 100cqh * var(--ratio));
    aspect-ratio: var(--ratio);
    background: #fff;
  }
  img {
    display: block;
    width: 100%;
    height: 100%;
    user-select: none;
  }
  .laser {
    position: absolute;
    width: 16px;
    height: 16px;
    margin: -8px 0 0 -8px;
    border-radius: 50%;
    background: #e01b24;
    box-shadow: 0 0 10px 3px rgb(224 27 36 / 70%);
    pointer-events: none;
  }
  .away {
    position: absolute;
    right: 12px;
    bottom: 12px;
    padding: 4px 10px;
    border-radius: 999px;
    background: rgb(0 0 0 / 55%);
    color: rgb(255 255 255 / 85%);
    font: 12px/1.4 var(--font-ui);
    pointer-events: none;
  }
</style>
