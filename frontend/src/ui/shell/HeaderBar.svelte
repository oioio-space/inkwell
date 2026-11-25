<script lang="ts">
import { availability, menuOf } from "../../extensions/commands";
import { t } from "../../i18n";
import { statusLabel } from "../../net/syncLabel";
import type { Status } from "../../workspace/workspace.svelte";
import CommandButton from "../command/CommandButton.svelte";
import CommandMenuItem from "../command/CommandMenuItem.svelte";
import Icon from "../icons/Icon.svelte";
import PeopleStack from "../PeopleStack.svelte";
import MenuButton from "../popover/MenuButton.svelte";

/**
 * The header bar: the panels' toggles, presenting, people and sharing as commands of the registry
 * (ADR 0027), and the main menu drawn from its `menus` (groups view, project, document, app).
 */
let {
  title,
  subtitle,
  unsaved = false,
  status,
  sending = 0,
  slow = false,
  me,
  peers,
}: {
  title: string;
  /** The file being edited, under the title. */
  subtitle: string;
  /** A dot before the file name: its text is not in the file yet. */
  unsaved?: boolean;
  status: Status;
  /** n of « Envoi en cours (n) »: the connected people who lack some of my changes. */
  sending?: number;
  /** A link has held a message for more than 2 s: « Lien lent » is added. */
  slow?: boolean;
  me: { id: string; name: string };
  peers: string[];
} = $props();

let menuOpen = $state(false);
const canPresent = $derived(availability("typst.present").ok);
const groups = $derived(menuOpen ? menuOf("main") : []);
</script>

<header class="hb">
  <div class="start">
    <CommandButton id="app.sidebar"><Icon name="side" /></CommandButton>
    <span class="status" data-status={status} role="status">
      <span class="dot" aria-hidden="true"></span>{statusLabel(status, sending, slow)}
    </span>
  </div>
  <div class="ttl">
    <h1>{title}</h1>
    <small>{#if unsaved}<span class="unsaved" data-testid="unsaved-dot" role="img" aria-label={t("save.unsaved")} title={t("save.unsaved")}></span>{/if}{subtitle}</small>
  </div>
  <div class="end">
    <CommandButton id="app.preview"><Icon name="eye" /></CommandButton>
    {#if canPresent}
      <CommandButton id="typst.present" testid="present-start"><Icon name="present" /></CommandButton>
    {/if}
    <PeopleStack {me} {peers} />
    <CommandButton id="app.share" class="sug">{t("share.button")}</CommandButton>
    <CommandButton id="app.review" testid="history-toggle"><Icon name="branch" /></CommandButton>
    <MenuButton bind:open={menuOpen} label={t("menu.main")} name={t("menu.main")} align="end" width={300}>
      {#snippet trigger()}<Icon name="menu" />{/snippet}
      {#snippet children({ close })}
        {#each groups as g, i (g.group)}
          {#if i > 0}<div class="sep" role="separator"></div>{/if}
          {#each g.items as c (c.id)}
            <CommandMenuItem id={c.id} {close} />
          {/each}
        {/each}
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
  [data-status="connecting"] .dot,
  [data-status="checking"] .dot {
    background: var(--accent-bg);
    animation: pulse 1.2s ease-in-out infinite;
  }
  [data-status="sending"] .dot {
    background: var(--accent-bg);
  }
  [data-status="behind"] .dot {
    background: var(--warning);
  }
  @keyframes pulse {
    50% {
      opacity: 0.3;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    [data-status="connecting"] .dot,
    [data-status="checking"] .dot {
      animation: none;
    }
  }
</style>
