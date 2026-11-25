<script lang="ts">
import { t } from "../../i18n";
import { statusLabel } from "../../net/syncLabel";
import type { Status } from "../../workspace/workspace.svelte";
import LocationBar from "../command/LocationBar.svelte";
import LocationMenu from "../command/LocationMenu.svelte";
import Icon from "../icons/Icon.svelte";
import PeopleStack from "../PeopleStack.svelte";

/**
 * The header bar (EX0c1): the `header/start` location, then the sync state; the title; the
 * `header/end` location around its fixed widgets (view, ‹people›, document, review, ‹main menu›,
 * today's order);
 * the main menu is the `main` location. Every button is a command of the registry (ADR 0027).
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
</script>

<header class="hb">
  <div class="start">
    <LocationBar location="header/start" />
    <span class="status" data-status={status} role="status">
      <span class="dot" aria-hidden="true"></span>{statusLabel(status, sending, slow)}
    </span>
  </div>
  <div class="ttl">
    <h1>{title}</h1>
    <small>{#if unsaved}<span class="unsaved" data-testid="unsaved-dot" role="img" aria-label={t("save.unsaved")} title={t("save.unsaved")}></span>{/if}{subtitle}</small>
  </div>
  <div class="end">
    <LocationBar location="header/end" groups={["view"]} />
    <PeopleStack {me} {peers} />
    <LocationBar location="header/end" groups={["document", "review"]} />
    <LocationMenu location="main" label={t("menu.main")} name={t("menu.main")}>
      {#snippet trigger()}<Icon name="menu" />{/snippet}
    </LocationMenu>
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
