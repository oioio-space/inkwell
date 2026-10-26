<script lang="ts">
import type { Row } from "../../history/graph";
import { stamp } from "../../history/stamp";
import { t, tn } from "../../i18n";
import { history, ROW_HEIGHT } from "./history.svelte";

let {
  row,
  first,
  last,
  selected,
  onpick,
}: { row: Row; first: boolean; last: boolean; selected: boolean; onpick: () => void } = $props();

const H = ROW_HEIGHT;
const mid = H / 2;
const laneX = (lane: number) => 14 + lane * 16;
const width = $derived(
  laneX(Math.max(row.lane, row.forkFrom ?? 0, row.mergeFrom ?? 0, ...row.through)) + 14,
);
const color = (lane: number) =>
  lane === 0 ? "var(--fg-dim)" : `var(--person-${((lane * 3) % 8) + 1})`;

/** A curve from the point (x1, y1) to (x2, y2): the branch leaving or joining the main line. */
const curve = (x1: number, y1: number, x2: number, y2: number) =>
  `M${x1} ${y1} C${x1} ${(y1 + y2) / 2} ${x2} ${(y1 + y2) / 2} ${x2} ${y2}`;

const session = $derived(row.session);
const author = $derived(session ? history.authorOf(session) : undefined);
const variantAuthor = $derived(row.variant ? history.author(row.variant.by) : undefined);
const versionAuthor = $derived(row.version ? history.versionBy(row.version) : undefined);

const title = $derived.by(() => {
  if (row.kind === "now") return t("history.now");
  if (row.kind === "session" && session) return history.entryTitle(session);
  if (row.kind === "version" && row.version)
    return t("history.row.version", { name: row.version.name });
  if (row.kind === "variant" && row.variant)
    return t("history.row.variant", { name: row.variant.name });
  if (row.kind === "merge" && row.variant)
    return t("history.row.merged", { name: row.variant.name });
  return "";
});

const sub = $derived.by(() => {
  if (row.kind === "now") return t("history.now.sub");
  if (row.kind === "session" && session && author) {
    const when = stamp(session.end, session.skewed, author.name);
    return session.action
      ? `${author.mine ? t("history.you") : author.name} · ${when}`
      : `${when} · ${tn("history.count", session.count)}`;
  }
  if (row.kind === "version" && row.version && versionAuthor) {
    return t("history.row.version.sub", {
      when: stamp(row.version.at, false, versionAuthor.name),
      name: versionAuthor.name,
    });
  }
  if (row.kind === "variant" && row.variant && variantAuthor) {
    return `${variantAuthor.name} · ${t(`history.variant.${row.variant.state}`)}`;
  }
  if (row.kind === "merge") return t("history.row.merged.sub");
  return "";
});

const hint = $derived(
  row.kind === "version" && versionAuthor
    ? t("history.device.hint", { name: versionAuthor.name })
    : undefined,
);

const nodeColor = $derived(row.kind === "now" ? "var(--accent-bg)" : color(row.lane));
</script>

<div
  class="row"
  class:selected
  class:abandoned={row.variant?.state === "abandoned" && row.kind === "variant"}
  role="option"
  aria-selected={selected}
  tabindex="-1"
  data-key={row.key}
  data-kind={row.kind}
  style:height="{ROW_HEIGHT}px"
  onclick={onpick}
  onkeydown={(e) => (e.key === "Enter" || e.key === " ") && onpick()}
>
  <svg class="graph" {width} height={H} viewBox="0 0 {width} {H}" aria-hidden="true">
    {#if !(first && row.lane === 0 && row.kind === "now")}
      <!-- the main line, above the node -->
      {#if !first}<path d="M{laneX(0)} 0V{mid}" stroke={color(0)} stroke-width="2" fill="none" />{/if}
    {/if}
    {#if !last}<path d="M{laneX(0)} {mid}V{H}" stroke={color(0)} stroke-width="2" fill="none" />{/if}
    {#each row.through as lane}
      <path d="M{laneX(lane)} 0V{H}" stroke={color(lane)} stroke-width="2" fill="none" />
    {/each}
    {#if row.forkFrom !== undefined}
      <path d={curve(laneX(row.forkFrom), 0, laneX(0), mid)} stroke={color(row.forkFrom)} stroke-width="2" fill="none" />
    {/if}
    {#if row.mergeFrom !== undefined}
      <path d={curve(laneX(row.mergeFrom), H, laneX(0), mid)} stroke={color(row.mergeFrom)} stroke-width="2" fill="none" />
    {/if}
    {#if row.kind === "variant"}
      {#if row.down}<path d="M{laneX(row.lane)} {mid}V{H}" stroke={color(row.lane)} stroke-width="2" fill="none" />{/if}
      {#if row.up}<path d="M{laneX(row.lane)} 0V{mid}" stroke={color(row.lane)} stroke-width="2" fill="none" />{/if}
    {/if}
    {#if row.kind === "version"}
      <rect x={laneX(row.lane) - 6} y={mid - 6} width="12" height="12" transform="rotate(45 {laneX(row.lane)} {mid})" fill={nodeColor} stroke="var(--sidebar-bg)" stroke-width="2" />
    {:else if row.kind === "now"}
      <circle cx={laneX(row.lane)} cy={mid} r="6" fill={nodeColor} />
    {:else}
      <circle cx={laneX(row.lane)} cy={mid} r="6" fill="var(--sidebar-bg)" stroke={author && row.kind === "session" ? `var(--person-${author.index})` : nodeColor} stroke-width="2.5" />
    {/if}
  </svg>
  <div class="txt" title={hint}>
    <span class="t">{title}</span>
    <span class="s">{sub}</span>
  </div>
</div>

<style>
  .row {
    box-sizing: border-box;
    display: flex;
    align-items: center;
    gap: 4px;
    padding-right: 10px;
    border-radius: var(--radius-button);
    cursor: default;
  }
  .row:hover {
    background: var(--hover-bg);
  }
  .row.selected {
    background: var(--selected-bg);
  }
  .row.abandoned {
    opacity: 0.55;
  }
  .graph {
    flex: none;
    overflow: visible;
  }
  .txt {
    display: flex;
    flex-direction: column;
    min-width: 0;
    flex: 1;
  }
  .t {
    overflow: hidden;
    font-weight: 700;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .s {
    overflow: hidden;
    color: var(--fg-dim);
    font-size: 12.5px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
