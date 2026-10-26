<script lang="ts">
import { lang, t } from "../i18n";
import type { Page } from "../preview/preview.svelte";
import Banner from "../ui/Banner.svelte";
import Icon from "../ui/icons/Icon.svelte";
import { formatElapsed } from "./clock";
import type { Presentation } from "./present.svelte";
import SlideStage from "./SlideStage.svelte";

/**
 * The presenter's view, over the editor: the current slide, the next step or slide, the speaker
 * notes, the timer and the clock; and, when several people are in the project, who pilots and how
 * to take or hand over the controls. The same view serves a follower, who watches and may browse
 * freely. The public window shows none of this.
 */
let {
  p,
  pages,
  notesSize,
  publicOpen,
  onExit,
  onTogglePublic,
  onNotesSize,
}: {
  p: Presentation;
  pages: ReadonlyMap<number, Page>;
  notesSize: number;
  publicOpen: boolean;
  onExit: () => void;
  onTogglePublic: () => void;
  onNotesSize: (size: number) => void;
} = $props();

const here = $derived(pages.get(p.page));

// The editor behind is inert while a presentation shows: the keyboard starts here, at the top.
let root: HTMLElement | undefined = $state();
$effect(() => root?.focus({ preventScroll: true }));
const nextPage = $derived(
  p.upcoming.kind === "end" ? undefined : pages.get(p.deck.pageOf(p.upcoming.pos)),
);
const clock = $derived(
  new Date(p.now).toLocaleTimeString(lang, { hour: "2-digit", minute: "2-digit" }),
);
const elapsed = $derived(p.now >= 0 ? formatElapsed(p.stopwatch.elapsedMs()) : ""); // `now` ticks each second
const counter = $derived(
  t("present.slideOf", { n: p.index, count: p.deck.count }) +
    (p.pos.step > 0 ? ` · ${t("present.step", { step: p.pos.step + 1 })}` : ""),
);

/** A click must not keep the focus: Space then goes to the slides, not to the button again. */
const click = (fn: () => void) => (e: MouseEvent) => {
  fn();
  (e.currentTarget as HTMLElement).blur();
};

function pace(): string {
  const v = p.pace;
  if (!v) return "";
  if (v.kind === "onTime") return t("present.pace.onTime");
  return t(v.kind === "ahead" ? "present.pace.ahead" : "present.pace.behind", {
    minutes: v.minutes,
  });
}
</script>

<section class="present" aria-label={t("present.label")} data-testid="presenter-view" tabindex="-1" bind:this={root}>
  <header class="hb">
    <div class="start">
      <button type="button" class="fb lbl" onclick={click(onExit)} data-testid="present-exit">
        <Icon name="back" /><span class="txt">{t("present.exit")}</span>
      </button>
    </div>
    <div class="ttl" role="status" data-testid="pilot-status">
      {#if p.isPilot}
        {t("present.pilot.you")}
      {:else if p.waiting}
        {t("present.pilot.waiting")}
      {:else}
        {t("present.pilot.other", { name: p.pilotName })}
      {/if}
    </div>
    <div class="end">
      <span class="timer" title={t("present.timer")}>
        <span class="time" data-testid="timer">{elapsed}</span>
        {#if p.timerRunning}
          <button type="button" class="fb" aria-label={t("present.timer.pause")} title={t("present.timer.pause")} onclick={click(() => p.pauseTimer())}>
            <Icon name="pause" />
          </button>
        {:else}
          <button type="button" class="fb" aria-label={t("present.timer.start")} title={t("present.timer.start")} onclick={click(() => p.startTimer())}>
            <Icon name="play" />
          </button>
        {/if}
        <button type="button" class="fb" aria-label={t("present.timer.reset")} title={t("present.timer.reset")} onclick={click(() => p.resetTimer())}>
          <Icon name="undo" />
        </button>
      </span>
      <span class="clock" title={t("present.clock")} data-testid="clock">{clock}</span>
      <button
        type="button"
        class="fb"
        class:on={p.laserOn}
        aria-pressed={p.laserOn}
        aria-label={t("present.laser")}
        title={t("present.laser")}
        disabled={!p.isPilot}
        onclick={click(() => p.toggleLaser())}
      >
        <Icon name="target" />
      </button>
      <button
        type="button"
        class="fb"
        class:on={publicOpen}
        aria-pressed={publicOpen}
        aria-label={t(publicOpen ? "present.public.close" : "present.public.open")}
        title={publicOpen ? t("present.public.hint") : t("present.public.open")}
        data-testid="present-public"
        onclick={click(onTogglePublic)}
      >
        <Icon name="present" />
      </button>
    </div>
  </header>

  {#if !p.mayPilot && !p.isPilot}
    <Banner role="status">{t("present.reader")}</Banner>
  {/if}
  {#each p.requests as r (r.peer)}
    <Banner role="alert">
      {t("present.request", { name: r.name })}
      {#snippet action()}
        <button type="button" class="btn" onclick={click(() => p.deny(r.peer))}>{t("present.refuse")}</button>
        <button type="button" class="sug" onclick={click(() => p.grant(r.peer))}>{t("present.grant")}</button>
      {/snippet}
    </Banner>
  {/each}
  {#if !p.isPilot && p.mayPilot}
    {#if p.waiting || p.away || p.ended}
      {#if p.canTake}
        <Banner role="status">
          {p.ended ? t("present.pilot.ended") : p.away ? t("present.pilot.away") : t("present.pilot.waiting")}
          {#snippet action()}
            <button type="button" class="sug" data-testid="present-take" onclick={click(() => p.take())}>{t("present.take")}</button>
          {/snippet}
        </Banner>
      {/if}
    {:else}
      <Banner role="status">
        {t("present.pilot.other", { name: p.pilotName })}
        {#snippet action()}
          {#if p.asking}
            <span class="dim">{t("present.asking")}</span>
          {:else}
            <button type="button" class="sug" data-testid="present-ask" disabled={!p.canAsk} onclick={click(() => p.ask())}>{t("present.take")}</button>
          {/if}
        {/snippet}
      </Banner>
      {#if p.denied}
        <Banner tone="warning" role="status">{t("present.denied")}</Banner>
      {/if}
    {/if}
  {/if}
  {#if !p.isPilot && !p.waiting}
    <div class="followbar">
      <div class="seg" role="group" aria-label={t("present.label")}>
        <button type="button" class="fb lbl" aria-pressed={p.follow === "follow"} data-testid="present-follow" onclick={click(() => p.setFollow("follow"))}>
          {p.follow === "free" ? t("present.back") : t("present.follow")}
        </button>
        <button type="button" class="fb lbl" aria-pressed={p.follow === "free"} data-testid="present-free" onclick={click(() => p.setFollow("free"))}>
          {t("present.free")}
        </button>
      </div>
    </div>
  {/if}

  <div class="body">
    <div class="cur" aria-label={t("present.current")}>
      {#if p.deck.empty}
        <p class="empty">{t("present.empty")}</p>
      {:else}
        <SlideStage
          src={here?.src}
          width={here?.width}
          height={here?.height}
          screen={p.screen}
          laser={p.laser}
          away={p.away}
          alt={t("present.slideAlt", { n: p.index })}
          onpointermove={p.isPilot && p.laserOn ? (x, y) => p.moveLaser(x, y) : undefined}
        />
      {/if}
    </div>
    <aside class="side">
      <section class="card next" aria-label={t("present.next")}>
        <h2>{p.upcoming.kind === "step" ? t("present.nextStep") : p.upcoming.kind === "slide" ? t("present.nextSlide") : t("present.next")}</h2>
        <div class="thumb">
          {#if p.upcoming.kind === "end"}
            <p class="end-note" data-testid="present-last">{t("present.last")}</p>
          {:else}
            <SlideStage src={nextPage?.src} width={nextPage?.width} height={nextPage?.height} alt={t("present.nextSlide")} />
          {/if}
        </div>
      </section>
      <section class="card notes" aria-label={t("present.notes")}>
        <div class="notes-head">
          <h2>{t("present.notes")}</h2>
          <span>
            <button type="button" class="fb" aria-label={t("present.notes.smaller")} title={t("present.notes.smaller")} data-testid="notes-smaller" onclick={click(() => onNotesSize(notesSize - 2))}>{t("present.notes.minus")}</button>
            <button type="button" class="fb" aria-label={t("present.notes.larger")} title={t("present.notes.larger")} data-testid="notes-larger" onclick={click(() => onNotesSize(notesSize + 2))}>{t("present.notes.plus")}</button>
          </span>
        </div>
        <div class="notes-text" style:font-size="{notesSize}px" data-testid="notes">
          {#if p.notes}{p.notes}{:else}<span class="dim">{t("present.notes.none")}</span>{/if}
        </div>
      </section>
    </aside>
  </div>

  <footer class="foot">
    <span data-testid="present-counter">{counter}</span>
    {#if p.typed}<span class="typed" data-testid="present-typed">{t("present.goto", { n: p.typed })}</span>{/if}
    {#if p.pace}<span class="pace" data-pace={p.pace.kind} data-testid="present-pace">{pace()}</span>{/if}
    <span class="keys dim">{t("present.keys")}</span>
  </footer>
</section>

<style>
  .present {
    position: fixed;
    inset: 0;
    z-index: 50;
    display: flex;
    flex-direction: column;
    background: var(--window-bg);
    color: var(--fg);
  }
  .present:focus {
    outline: none;
  }
  .hb {
    box-sizing: border-box;
    display: grid;
    grid-template-columns: 1fr auto 1fr;
    align-items: center;
    gap: 6px;
    height: 47px;
    padding: 0 6px;
    background: var(--headerbar-bg);
    box-shadow: 0 1px var(--shade);
  }
  .start,
  .end {
    display: flex;
    align-items: center;
    gap: 4px;
    min-width: 0;
  }
  .end {
    justify-content: flex-end;
  }
  .ttl {
    font: 700 14.5px/1.25 var(--font-ui);
    white-space: nowrap;
  }
  .timer {
    display: inline-flex;
    align-items: center;
  }
  .time,
  .clock {
    font-variant-numeric: tabular-nums;
    padding: 0 8px;
  }
  .clock {
    color: var(--fg-dim);
  }
  .dim {
    color: var(--fg-dim);
  }
  .followbar {
    display: flex;
    justify-content: center;
    padding: 6px;
    background: var(--headerbar-bg);
  }
  .seg {
    display: inline-flex;
    padding: 2px;
    border-radius: var(--radius-button);
    background: var(--btn-bg);
  }
  .body {
    flex: 1;
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(260px, 30%);
    gap: 12px;
    min-height: 0;
    padding: 12px;
  }
  .cur {
    min-width: 0;
    min-height: 0;
    border-radius: var(--radius-card, 12px);
    overflow: hidden;
    box-shadow: 0 0 0 1px var(--card-shade);
  }
  .empty {
    display: grid;
    place-items: center;
    height: 100%;
    margin: 0;
    background: var(--view-bg);
    color: var(--fg-dim);
  }
  .side {
    display: flex;
    flex-direction: column;
    gap: 12px;
    min-width: 0;
    min-height: 0;
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 10px;
    border-radius: var(--radius-card, 12px);
    background: var(--card-bg);
    box-shadow: 0 0 0 1px var(--card-shade);
  }
  .next {
    flex: none;
  }
  .notes {
    flex: 1;
    min-height: 0;
  }
  h2 {
    margin: 0;
    font: 700 13px/1.3 var(--font-ui);
    color: var(--fg-dim);
  }
  .thumb {
    aspect-ratio: 16 / 9;
    border-radius: 8px;
    overflow: hidden;
  }
  .end-note {
    display: grid;
    place-items: center;
    height: 100%;
    margin: 0;
    background: var(--btn-bg);
    color: var(--fg-dim);
  }
  .notes-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .notes-text {
    flex: 1;
    min-height: 0;
    overflow: auto;
    white-space: pre-wrap;
    line-height: 1.4;
  }
  .foot {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 6px 14px;
    background: var(--headerbar-bg);
    box-shadow: 0 -1px var(--shade);
    font-size: 13px;
  }
  .keys {
    margin-inline-start: auto;
  }
  .typed {
    font-weight: 700;
    color: var(--accent);
  }
  .pace[data-pace="behind"] {
    color: var(--error);
  }
  .pace[data-pace="ahead"] {
    color: var(--success);
  }
  @media (max-width: 760px) {
    /* The title gives way (it is cut with "…"), then the clock and the label of the exit button:
       at 200 % zoom or in a narrow window nothing may overlap. */
    .hb {
      grid-template-columns: auto minmax(0, 1fr) auto;
    }
    .ttl {
      min-width: 0;
      overflow: hidden;
      text-align: center;
      text-overflow: ellipsis;
    }
    .clock {
      display: none;
    }
    .txt {
      position: absolute;
      width: 1px;
      height: 1px;
      overflow: hidden;
      clip-path: inset(50%);
      white-space: nowrap;
    }
    .body {
      grid-template-columns: 1fr;
      grid-auto-rows: minmax(0, 1fr);
    }
    .keys {
      display: none;
    }
  }
</style>
