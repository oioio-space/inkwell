<script lang="ts">
import type { Snippet } from "svelte";

/**
 * An AdwBanner: a full-width strip under the toolbar with a sentence and, optionally, a
 * button. `role="alert"` is only for what needs an answer now.
 */
let {
  tone = "info",
  role,
  children,
  action,
}: {
  tone?: "info" | "warning";
  role?: "alert" | "status";
  children: Snippet;
  action?: Snippet;
} = $props();
</script>

<div class="banner {tone}" {role}>
  <span class="msg">{@render children()}</span>
  {#if action}{@render action()}{/if}
</div>

<style>
  .banner {
    box-sizing: border-box;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-wrap: wrap;
    gap: 12px;
    min-height: 45px;
    padding: 6px 16px;
    color: var(--fg);
  }
  .info {
    background: var(--info-bg);
  }
  .warning {
    background: var(--warning-bg);
  }
  .banner :global(.btn) {
    min-height: 32px;
  }
</style>
