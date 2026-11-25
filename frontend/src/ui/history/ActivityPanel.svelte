<script lang="ts">
import { stamp } from "../../history/stamp";
import { t } from "../../i18n";
import { history } from "./history.svelte";

/** The drawer's panel `app.activity` (EX0c2): what happened besides the text, newest first. */
/** Events other than text changes, newest first: the feed of the « Activité » tab. */
const activity = $derived.by(() => {
  const doc = history.doc;
  if (!doc) return [];
  const out: { key: string; text: string; when: number }[] = [];
  for (const v of doc.meta?.versions ?? []) {
    if (!v.auto)
      out.push({
        key: `v${v.at}${v.name}`,
        text: t("history.act.version", { name: history.author(v.by).name, version: v.name }),
        when: v.at,
      });
  }
  for (const v of doc.meta?.variants ?? []) {
    out.push({
      key: `b${v.url}`,
      text: t("history.act.variant", { name: history.author(v.by).name, variant: v.name }),
      when: v.at,
    });
  }
  for (const s of history.entries) {
    if (s.action)
      out.push({
        key: `s${s.id}`,
        text: `${history.author(s.actor).name} · ${history.actionTitle(s)}`,
        when: s.end,
      });
  }
  return out.sort((a, b) => b.when - a.when);
});
</script>

  <ul class="feed" aria-label={t("history.tab.activity")}>
    {#each activity as a (a.key)}
      <li><span>{a.text}</span><small>{stamp(a.when, false, "")}</small></li>
    {:else}
      <li class="empty">{t("history.activity.empty")}</li>
    {/each}
  </ul>

<style>
  .empty {
    flex: none;
    margin: 0;
    color: var(--fg-dim);
    font-size: 12.5px;
    line-height: 1.5;
  }
  .feed {
    margin: 0;
    padding: 0;
    overflow: auto;
    list-style: none;
  }
  .feed li {
    display: flex;
    flex-direction: column;
    padding: 8px 4px;
    box-shadow: inset 0 -1px var(--card-shade);
  }
  .feed small {
    color: var(--fg-dim);
  }
</style>
