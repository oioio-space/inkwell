<script lang="ts">
import type { Snippet } from "svelte";
import { t } from "../i18n";
import Icon from "./icons/Icon.svelte";

/**
 * An AdwToast: a dark pill at the bottom of the window with one sentence and its actions.
 * `role="alert"` interrupts a screen reader (something needs an answer); "status" waits its turn.
 */
let {
  role = "status",
  children,
  actions,
  onclose,
}: {
  role?: "alert" | "status";
  children: Snippet;
  actions?: Snippet;
  /** Shows the close button when given. */
  onclose?: () => void;
} = $props();
</script>

<div class="toast" {role}>
  <span class="msg">{@render children()}</span>
  {#if actions}{@render actions()}{/if}
  {#if onclose}
    <button type="button" class="fb dismiss" aria-label={t("toast.close")} onclick={onclose}>
      <Icon name="close" size={12} />
    </button>
  {/if}
</div>

<style>
  .toast {
    position: fixed;
    inset-block-end: 56px;
    inset-inline: 0;
    z-index: 40;
    margin-inline: auto;
    width: fit-content;
    max-width: calc(100vw - 32px);
    box-sizing: border-box;
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 6px 6px 18px;
    border-radius: 999px;
    background: rgb(0 0 6 / 84%);
    color: #fff;
    box-shadow: 0 4px 16px rgb(0 0 6 / 25%);
  }
  .msg {
    margin-inline-end: 6px;
  }
  .toast :global(.fb) {
    height: 32px;
    padding: 0 12px;
    color: #fff;
    font-weight: 700;
  }
  .toast :global(.fb:hover:not(:disabled)) {
    background: rgb(255 255 255 / 14%);
  }
  .toast :global(.fb.dismiss) {
    min-width: 32px;
    padding: 0;
    border-radius: 50%;
  }
</style>
