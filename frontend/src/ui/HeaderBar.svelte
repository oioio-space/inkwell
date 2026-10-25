<script lang="ts">
import { t } from "../i18n";
import type { Status } from "../workspace/workspace.svelte";
import PeopleStack from "./PeopleStack.svelte";

let {
  title,
  status,
  me,
  peers,
  onShare,
}: {
  title: string;
  status: Status;
  me: { id: string; name: string };
  peers: string[];
  onShare: () => void;
} = $props();
</script>

<header class="headerbar">
  <div class="start">
    <span class="status" data-status={status} role="status">
      <span class="dot" aria-hidden="true"></span>{t(`status.${status}`)}
    </span>
  </div>
  <h1 class="title">{title}</h1>
  <div class="end">
    <PeopleStack {me} {peers} />
    <button type="button" class="suggested" onclick={onShare}>{t("share.button")}</button>
  </div>
</header>

<style>
  .headerbar {
    display: grid;
    grid-template-columns: 1fr auto 1fr;
    align-items: center;
    gap: var(--space-3);
    min-height: 47px;
    padding: 0 var(--space-2);
    background: var(--headerbar-bg);
    border-bottom: 1px solid var(--border);
  }
  .title {
    margin: 0;
    font: 700 15px/1 var(--font-ui);
  }
  .end {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: var(--space-3);
  }
  .status {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: 0 var(--space-3);
    min-height: 30px;
    border-radius: 999px;
    color: var(--fg-dim);
    font-size: 13px;
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
  [data-status="connecting"] .dot {
    background: var(--accent);
    animation: pulse 1.2s ease-in-out infinite;
  }
  @keyframes pulse {
    50% {
      opacity: 0.3;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    [data-status="connecting"] .dot {
      animation: none;
    }
  }
  .suggested {
    min-height: 34px;
    padding: 0 var(--space-4);
    border: 0;
    border-radius: var(--radius-small);
    background: var(--accent);
    color: var(--accent-fg);
    font: 700 14px/1 var(--font-ui);
    cursor: pointer;
  }
  .suggested:hover {
    background: color-mix(in srgb, var(--accent) 88%, #000);
  }
</style>
