<script lang="ts">
import type { BaseStats, Stats } from "../../editor/stats/stats";
import { t } from "../../i18n";
import { count } from "../shell/count";

/** Document and selection statistics, side by side. The selection column is empty without one. */
let { stats }: { stats: Stats } = $props();

const ROWS = [
  ["words", "mot", "stats.words"],
  ["chars", "caractère", "stats.chars"],
  ["charsNoSpaces", "caractère", "stats.charsNoSpaces"],
  ["sentences", "phrase", "stats.sentences"],
  ["paragraphs", "paragraphe", "stats.paragraphs"],
] as const;

const cell = (s: BaseStats | undefined, key: (typeof ROWS)[number][0]) =>
  s ? new Intl.NumberFormat().format(s[key]) : "—";
const reading = (s: BaseStats | undefined) =>
  s ? (s.readingMinutes === 0 ? "—" : count(s.readingMinutes, "minute")) : "—";
</script>

<table class="stats">
  <thead>
    <tr>
      <th scope="col"><span class="sr-only">{t("stats.measure")}</span></th>
      <th scope="col">{t("stats.document")}</th>
      <th scope="col">{t("stats.selection")}</th>
    </tr>
  </thead>
  <tbody>
    {#each ROWS as [key, , label] (key)}
      <tr>
        <th scope="row">{t(label)}</th>
        <td>{cell(stats, key)}</td>
        <td class:none={!stats.selection}>{cell(stats.selection, key)}</td>
      </tr>
    {/each}
    <tr>
      <th scope="row">{t("stats.reading")}</th>
      <td>{reading(stats)}</td>
      <td class:none={!stats.selection}>{reading(stats.selection)}</td>
    </tr>
  </tbody>
</table>

<style>
  .stats {
    border-collapse: collapse;
    margin: 4px 6px;
    font-size: 13.5px;
    font-variant-numeric: tabular-nums;
  }
  th,
  td {
    padding: 5px 10px;
    text-align: end;
    white-space: nowrap;
  }
  thead th {
    color: var(--fg-dim);
    font-size: 12.5px;
    font-weight: 700;
  }
  tbody th {
    color: var(--fg-dim);
    font-weight: 400;
    text-align: start;
  }
  .none {
    color: var(--fg-dim);
  }
</style>
