<script lang="ts">
import { t } from "../../i18n";
import type { Status } from "../../workspace/workspace.svelte";
import Icon from "../icons/Icon.svelte";
import PeopleStack from "../PeopleStack.svelte";
import MenuButton from "../popover/MenuButton.svelte";
import { shortcutLabel } from "../toolbar/spec";

let {
  title,
  subtitle,
  status,
  me,
  peers,
  sidebarOpen,
  toolbarVisible,
  canToggleToolbar,
  onShare,
  onToggleSidebar,
  onToggleToolbar,
  onPreferences,
}: {
  title: string;
  /** The file being edited, under the title. */
  subtitle: string;
  status: Status;
  me: { id: string; name: string };
  peers: string[];
  sidebarOpen: boolean;
  toolbarVisible: boolean;
  /** False for files that have no formatting bar. */
  canToggleToolbar: boolean;
  onShare: () => void;
  onToggleSidebar: () => void;
  onToggleToolbar: () => void;
  onPreferences: () => void;
} = $props();

let menuOpen = $state(false);
</script>

<header class="hb">
  <div class="start">
    <button
      type="button"
      class="fb"
      class:on={sidebarOpen}
      aria-pressed={sidebarOpen}
      aria-label={t(sidebarOpen ? "sidebar.hide" : "sidebar.show")}
      title={t(sidebarOpen ? "sidebar.hide" : "sidebar.show")}
      onclick={onToggleSidebar}
    >
      <Icon name="side" />
    </button>
    <span class="status" data-status={status} role="status">
      <span class="dot" aria-hidden="true"></span>{t(`status.${status}`)}
    </span>
  </div>
  <div class="ttl">
    <h1>{title}</h1>
    <small>{subtitle}</small>
  </div>
  <div class="end">
    <PeopleStack {me} {peers} />
    <button type="button" class="sug" onclick={onShare}>{t("share.button")}</button>
    <MenuButton bind:open={menuOpen} label={t("menu.main")} name={t("menu.main")} align="end" width={300}>
      {#snippet trigger()}<Icon name="menu" />{/snippet}
      {#snippet children({ close })}
        {#if canToggleToolbar}
          <button
            type="button"
            class="mi"
            role="menuitemcheckbox"
            aria-checked={toolbarVisible}
            onclick={() => {
              onToggleToolbar();
              close();
            }}
          >
            <span>{t("menu.toolbar")}</span>
            <span class="k">{shortcutLabel("Mod-Shift-t")}</span>
          </button>
        {/if}
        <button
          type="button"
          class="mi"
          role="menuitem"
          onclick={() => {
            close(false);
            onPreferences();
          }}
        >
          <span>{t("menu.preferences")}</span>
        </button>
      {/snippet}
    </MenuButton>
  </div>
</header>

<style>
  .hb {
    box-sizing: border-box;
    display: grid;
    grid-template-columns: 1fr auto 1fr;
    align-items: center;
    gap: 6px;
    height: 47px;
    padding: 0 6px;
    background: var(--headerbar-bg);
  }
  .start,
  .end {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
  }
  .end {
    justify-content: flex-end;
  }
  .ttl {
    text-align: center;
    line-height: 1.25;
    white-space: nowrap;
  }
  h1 {
    margin: 0;
    font: 700 14.5px/1.25 var(--font-ui);
  }
  small {
    display: block;
    color: var(--fg-dim);
    font-size: 12px;
  }
  .status {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 0 8px;
    color: var(--fg-dim);
    font-size: 13px;
    white-space: nowrap;
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
    background: var(--accent-bg);
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
</style>
