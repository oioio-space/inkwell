<script lang="ts">
import { t } from "../i18n";
import { workspace } from "../workspace/workspace.svelte";
import { personColor, personIndex } from "./people";

let {
  me,
  peers,
  onOpen,
}: { me: { id: string; name: string }; peers: string[]; onOpen: () => void } = $props();
const initial = (s: string) => s.trim().charAt(0).toUpperCase() || "?";
</script>

<button
  type="button"
  class="open"
  aria-label={t("people.open")}
  title={t("people.title")}
  data-testid="people-open"
  onclick={onOpen}
>
<ul class="stack" aria-label={t("people.label", { count: peers.length + 1 })}>
  {#each peers as peer (peer)}
    {@const name = workspace.names[peer] ?? ""}
    <li style:--who={personColor(peer)} data-person={personIndex(peer)} title={name} aria-label={name || undefined}>{initial(name)}</li>
  {/each}
  <li
    class="me"
    style:--who={personColor(me.id)}
    data-person={personIndex(me.id)}
    title={`${me.name} (${t("people.you")})`}
  >
    {initial(me.name)}
  </li>
</ul>
</button>

<style>
  /* An avatar stack that is a button: no chrome of its own, the hover is the header bar's. */
  .open {
    display: flex;
    align-items: center;
    min-height: var(--target-bar);
    padding: 0 6px;
    border: 0;
    border-radius: var(--radius-button);
    background: transparent;
    color: inherit;
  }
  .open:hover {
    background: var(--btn-bg);
  }
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
    width: 28px;
    height: 28px;
    margin-left: -6px;
    border-radius: 50%;
    box-shadow: 0 0 0 2px var(--headerbar-bg);
    background: var(--who);
    /* Dark ink on the light colours, white on the dark blue: each pair clears AA. */
    color: #000;
    font: 700 12px/1 var(--font-ui);
  }
  li[data-person="5"] {
    color: #fff;
  }
  .me {
    margin-left: 0;
  }
</style>
