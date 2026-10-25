<script lang="ts">
import { t } from "../i18n";
import { personColor } from "./people";

let { me, peers }: { me: { id: string; name: string }; peers: string[] } = $props();
const initial = (s: string) => s.trim().charAt(0).toUpperCase() || "?";
</script>

<ul class="stack" aria-label={t("people.label", { count: peers.length + 1 })}>
  {#each peers as peer (peer)}
    <li style:--who={personColor(peer)} title={peer}>{initial(peer)}</li>
  {/each}
  <li class="me" style:--who={personColor(me.id)} title={`${me.name} (${t("people.you")})`}>
    {initial(me.name)}
  </li>
</ul>

<style>
  .stack {
    display: flex;
    flex-direction: row-reverse;
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li {
    display: grid;
    place-items: center;
    width: 30px;
    height: 30px;
    margin-left: -6px;
    border-radius: 50%;
    border: 2px solid var(--headerbar-bg);
    background: var(--who);
    color: #000;
    font: 600 13px/1 var(--font-ui);
  }
  .me {
    margin-left: 0;
  }
</style>
