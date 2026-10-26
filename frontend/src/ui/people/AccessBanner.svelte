<script lang="ts">
import { lang, t } from "../../i18n";
import { workspace } from "../../workspace/workspace.svelte";
import Banner from "../Banner.svelte";

/**
 * What a person who cannot edit is told, calmly: why, and how to come back. A reader has her
 * role; a disconnected person has the admin who did it and when (ADR 0021).
 */
const access = $derived(workspace.access);
const me = $derived(workspace.roster.find((m) => m.self));
const text = $derived.by(() => {
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
  <Banner role="status"><span data-testid="access-banner" data-access={access}>{text}</span></Banner>
{/if}
