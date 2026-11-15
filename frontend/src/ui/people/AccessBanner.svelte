<script lang="ts">
import { lang, t } from "../../i18n";
import { workspace } from "../../workspace/workspace.svelte";
import Banner from "../Banner.svelte";
import TakeOverDialog from "./TakeOverDialog.svelte";

/**
 * What a person who cannot edit is told, calmly: why, and how to come back. A reader has her
 * role; a disconnected person has the admin who did it and when (ADR 0021); a device the member
 * list does not name (W58a) can read, ask for an invitation, or, where Inkwell can restart itself
 * (residual R-8), take the document over here (PC1a, rule N-9): one confirmation, then the page
 * leaves and Inkwell restarts.
 *
 * @param onAsk opens the join part of the share dialog.
 */
let { onAsk = () => {} }: { onAsk?: () => void } = $props();
const access = $derived(workspace.access);
const foreign = $derived(access === "notNamed" || access === "removed");
/** An invitation was used and the list that names this device has not arrived: nothing to ask again. */
const waiting = $derived(access === "notNamed" && workspace.listPending);
/** True while the take-over waits for its one confirmation. */
let confirming = $state(false);

function takeOver() {
  confirming = false;
  void workspace.leave("restart.html");
}
const me = $derived(workspace.roster.find((m) => m.self));
const text = $derived.by(() => {
  if (waiting) return t("foreign.joinWaiting");
  if (access === "notNamed") return t("foreign.notNamed");
  if (access === "removed") return t("foreign.removed");
  if (access === "reader") return t("access.reader");
  if (access !== "suspended") return "";
  const peer = me?.suspendedByPeer;
  const name =
    (peer ? workspace.knownName(peer) : undefined) || me?.suspendedBy || t("access.suspended.by");
  const at = me?.suspendedAt
    ? new Date(me.suspendedAt).toLocaleTimeString(lang, { hour: "2-digit", minute: "2-digit" })
    : "";
  return t("access.suspended", { name, time: at ? t("access.suspended.at", { time: at }) : "" });
});
</script>

{#if access}
  <Banner role="status">
    <span data-testid="access-banner" data-access={access}>{text}</span>
    {#snippet action()}
      {#if foreign && !waiting}
        <button type="button" class="btn" data-testid="ask-invitation" onclick={onAsk}>
          {t("foreign.ask")}
        </button>
        {#if workspace.canTakeOver}
          <button type="button" class="btn" data-testid="take-over" disabled={workspace.leaving} onclick={() => (confirming = true)}>
            {t("foreign.takeOver")}
          </button>
        {/if}
      {/if}
    {/snippet}
  </Banner>
  {#if confirming}
    <TakeOverDialog onyes={takeOver} onno={() => (confirming = false)} />
  {/if}
{/if}
