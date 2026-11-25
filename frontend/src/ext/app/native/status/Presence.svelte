<script lang="ts">
import { t, tn } from "../../../../i18n";
import type { StatusItemProps } from "../../../../ui/command/status.svelte";

/** Who is here: the people connected and the state of the link, at the start of the bar. */
let { bar }: StatusItemProps = $props();
</script>

<span class="people" data-status={bar.status}>
  <span class="dot" aria-hidden="true"></span>
  {bar.peerCount === 0 ? t(bar.status === "connecting" ? "status.connecting" : "status.alone") : tn("people.count", bar.peerCount + 1)}
</span>

<style>
  .people {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 0 10px;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--fg-dim);
  }
  [data-status="synced"] .dot {
    background: var(--success);
  }
  [data-status="checking"] .dot,
  [data-status="sending"] .dot {
    background: var(--accent-bg);
  }
  [data-status="behind"] .dot {
    background: var(--warning);
  }
</style>
