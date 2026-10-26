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
  unsaved = false,
  status,
  me,
  peers,
  sidebarOpen,
  toolbarVisible,
  canToggleToolbar,
  previewOpen,
  onShare,
  onToggleSidebar,
  onToggleToolbar,
  onPreferences,
  onTogglePreview,
  onExport,
  onAbout,
  canPresent = false,
  onPresent = () => {},
}: {
  title: string;
  /** The file being edited, under the title. */
  subtitle: string;
  /** A dot before the file name: its text is not in the file yet. */
  unsaved?: boolean;
  status: Status;
  me: { id: string; name: string };
  peers: string[];
  sidebarOpen: boolean;
  toolbarVisible: boolean;
  /** False for files that have no formatting bar. */
  canToggleToolbar: boolean;
  /** Whether the preview pane is shown. */
  previewOpen: boolean;
  onShare: () => void;
  onToggleSidebar: () => void;
  onToggleToolbar: () => void;
  onPreferences: () => void;
  onTogglePreview: () => void;
  /** "Exporter en PDF". */
  onExport: () => void;
  onAbout: () => void;
  /** A Typst document can be presented. */
  canPresent?: boolean;
  /** "Présenter" (from the first slide), from this slide, or only as the audience view of this screen. */
  onPresent?: (how: "start" | "here" | "public") => void;
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
    <small>{#if unsaved}<span class="unsaved" data-testid="unsaved-dot" role="img" aria-label={t("save.unsaved")} title={t("save.unsaved")}></span>{/if}{subtitle}</small>
  </div>
  <div class="end">
    <button
      type="button"
      class="fb"
      class:on={previewOpen}
      aria-pressed={previewOpen}
      aria-label={t(previewOpen ? "preview.hide" : "preview.show")}
      title={t(previewOpen ? "preview.hide" : "preview.show")}
      onclick={onTogglePreview}
    >
      <Icon name="eye" />
    </button>
    {#if canPresent}
      <button
        type="button"
        class="fb"
        aria-label={t("present.start")}
        title="{t('present.start')} ({shortcutLabel('F5')})"
        data-testid="present-start"
        onclick={() => onPresent("start")}
      >
        <Icon name="present" />
      </button>
    {/if}
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
            <span class="tick" class:off={!toolbarVisible}><Icon name="check" size={14} /></span>
          </button>
        {/if}
        <button
          type="button"
          class="mi"
          role="menuitemcheckbox"
          aria-checked={previewOpen}
          onclick={() => {
            onTogglePreview();
            close();
          }}
        >
          <span>{t("menu.preview")}</span>
          <span class="tick" class:off={!previewOpen}><Icon name="check" size={14} /></span>
        </button>
        {#if canPresent}
          <button type="button" class="mi" role="menuitem" onclick={() => { close(false); onPresent("start"); }}>
            <span>{t("present.start")}</span>
            <span class="k">{shortcutLabel("F5")}</span>
          </button>
          <button type="button" class="mi" role="menuitem" onclick={() => { close(false); onPresent("here"); }}>
            <span>{t("present.startHere")}</span>
            <span class="k">{shortcutLabel("Shift-F5")}</span>
          </button>
          <button type="button" class="mi" role="menuitem" data-testid="present-as-public" onclick={() => { close(false); onPresent("public"); }}>
            <span>{t("present.showPublic")}</span>
          </button>
        {/if}
        <button
          type="button"
          class="mi"
          role="menuitem"
          onclick={() => {
            close(false);
            onExport();
          }}
        >
          <span>{t("menu.export")}</span>
        </button>
        <div class="sep" role="separator"></div>
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
        <button
          type="button"
          class="mi"
          role="menuitem"
          onclick={() => {
            close(false);
            onAbout();
          }}
        >
          <span>{t("menu.about")}</span>
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
  .tick {
    margin-inline-start: auto;
    display: inline-flex;
  }
  .tick.off {
    visibility: hidden;
  }
  .mi .k + .tick {
    margin-inline-start: 8px;
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
  .unsaved {
    display: inline-block;
    width: 8px;
    height: 8px;
    margin-inline-end: 6px;
    border-radius: 50%;
    background: var(--fg);
    vertical-align: 0.5px;
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
