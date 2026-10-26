<script lang="ts">
import { Session } from "../../../bindings/github.com/oioio-space/inkwell/internal/app/index";
import { lang, t } from "../../i18n";
import { workspace } from "../../workspace/workspace.svelte";
import Icon from "../icons/Icon.svelte";
import { notice } from "../notice.svelte";
import { personColor, personIndex } from "../people";
import { type Member, type Role, type Row, rowsOf } from "./roster";

let { open = $bindable(false) }: { open: boolean } = $props();
let dialog: HTMLDialogElement | undefined = $state();
/** The person being disconnected, or handed the project to: the row asks first. */
let asking = $state<{ id: string; what: "suspend" | "transfer" } | undefined>();
let error = $state("");
let busy = $state(false);

const rows = $derived(rowsOf(workspace.roster, new Set(workspace.peers)));

$effect(() => {
  if (open && !dialog?.open) {
    dialog?.showModal();
    void workspace.refreshRoster();
  }
  if (!open && dialog?.open) dialog.close();
});

/** Who the person is for the eye: the name they announced, else the one an admin gave. */
function nameOf(m: Member): string {
  if (m.self) return workspace.myName;
  for (const d of m.devices) {
    const announced = workspace.knownName(d.peer);
    if (announced) return announced;
  }
  return m.name || workspace.nameOf(m.devices[0]?.peer ?? m.id);
}

const clock = (iso: string | undefined) =>
  iso ? new Date(iso).toLocaleTimeString(lang, { hour: "2-digit", minute: "2-digit" }) : undefined;

function stateOf(row: Row): string {
  const m = row.member;
  if (!m.suspended) return t(row.online ? "people.online" : "people.offline");
  const byPeer = m.suspendedByPeer;
  const mine = byPeer !== undefined && byPeer === workspace.myId;
  const by = byPeer ? (workspace.knownName(byPeer) ?? "") : "";
  const name = mine ? t("people.you") : by || m.suspendedBy || t("access.suspended.by");
  const time = clock(m.suspendedAt);
  return time ? t("people.suspended", { name, time }) : t("people.suspended.unknown", { name });
}

/** Runs one action of Go on a person, then says what happened. */
async function run(act: () => Promise<void>, done: string) {
  error = "";
  busy = true;
  try {
    await act();
    notice.show(done);
  } catch (err) {
    error = t("people.error", { reason: String(err instanceof Error ? err.message : err) });
  } finally {
    asking = undefined;
    busy = false;
    await workspace.refreshRoster();
  }
}

const setRole = (m: Member, role: Role) =>
  run(
    () => Session.SetRole(m.id, role),
    t("people.done.role", { name: nameOf(m), role: t(`people.role.${role}`) }),
  );
const suspend = (m: Member) =>
  run(() => Session.Suspend(m.id), t("people.done.suspend", { name: nameOf(m) }));
const resume = (m: Member) =>
  run(() => Session.Resume(m.id), t("people.done.resume", { name: nameOf(m) }));
/** The creator makes the person an admin, then steps down (ADR 0017: a handover). */
const transfer = (m: Member) =>
  run(
    async () => {
      await Session.SetRole(m.id, "admin");
      await Session.SetRole(workspace.roster.find((p) => p.self)?.id ?? "", "writer");
    },
    t("people.done.transfer", { name: nameOf(m) }),
  );

const initial = (s: string) => s.trim().charAt(0).toUpperCase() || "?";
const manages = $derived(
  rows.some((r) => r.actions.roles.length > 0 || r.actions.suspend || r.actions.resume),
);
</script>

<dialog bind:this={dialog} onclose={() => (open = false)} aria-labelledby="people-title" data-testid="people-dialog">
  <header>
    <h2 id="people-title">{t("people.title")}</h2>
    <button type="button" class="close" aria-label={t("people.close")} onclick={() => (open = false)}>
      <span class="disc"><Icon name="close" size={12} /></span>
    </button>
  </header>

  {#if rows.length === 0}
    <p class="note">{t("people.empty")}</p>
  {:else}
    <ul class="boxed card" aria-label={t("people.title")}>
      {#each rows as row (row.member.id)}
        {@const m = row.member}
        {@const name = nameOf(m)}
        {@const peer = m.devices[0]?.peer ?? m.id}
        <li data-person-row={m.id} data-suspended={m.suspended}>
          <div class="line">
            <span
              class="avatar"
              style:--who={personColor(peer)}
              data-person={personIndex(peer)}
              aria-hidden="true">{initial(name)}</span
            >
            <span class="who">
              <span class="name">{name}{#if m.self}&nbsp;<span class="dim">({t("people.you")})</span>{/if}</span>
              <span class="state" class:online={row.online}>{stateOf(row)}</span>
            </span>
            <span class="controls">
            {#if row.actions.roles.length > 0}
              <select
                aria-label={t("people.role.label", { name })}
                disabled={busy}
                value={m.role}
onchange={async (e) => {
  const select = e.currentTarget;
  await setRole(m, select.value as Role);
  // A refused change must not leave the choice on screen: show what the list says.
  select.value = workspace.roster.find((p) => p.id === m.id)?.role ?? m.role;
}}
              >
                {#each ["reader", "writer", "admin"] as const as r (r)}
                  {#if r === m.role || row.actions.roles.includes(r)}
                    <option value={r}>{t(`people.role.${r}`)}</option>
                  {/if}
                {/each}
              </select>
            {:else}
              <span class="role">{t(`people.role.${m.role}`)}</span>
            {/if}
            {#if row.actions.suspend}
              <button type="button" class="btn" disabled={busy} aria-label={t("people.suspend.for", { name })} onclick={() => (asking = { id: m.id, what: "suspend" })}>
                {t("people.suspend")}
              </button>
            {/if}
            {#if row.actions.resume}
              <button type="button" class="btn" disabled={busy} aria-label={t("people.resume.for", { name })} onclick={() => resume(m)}>
                {t("people.resume")}
              </button>
            {/if}
            {#if row.actions.transfer}
              <button type="button" class="btn flat" disabled={busy} aria-label={t("people.transfer.for", { name })} onclick={() => (asking = { id: m.id, what: "transfer" })}>
                {t("people.transfer")}
              </button>
            {/if}
            </span>
          </div>
          {#if asking?.id === m.id}
            <div class="ask" role="group" aria-label={name}>
              <p>{t(asking.what === "suspend" ? "people.suspend.ask" : "people.transfer.ask", { name })}</p>
              <div class="buttons">
                <button type="button" class="btn flat" onclick={() => (asking = undefined)}>{t("people.cancel")}</button>
                <button
                  type="button"
                  class="btn danger"
                  disabled={busy}
                  onclick={() => (asking?.what === "suspend" ? suspend(m) : transfer(m))}
                >
                  {t(asking.what === "suspend" ? "people.suspend" : "people.transfer")}
                </button>
              </div>
            </div>
          {/if}
        </li>
      {/each}
    </ul>
    {#if !manages}
      <p class="note">{t("people.admin.only")}</p>
    {/if}
  {/if}
  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}
</dialog>

<style>
  dialog {
    box-sizing: border-box;
    width: min(600px, calc(100vw - 32px));
    padding: var(--space-4);
    border: 0;
    border-radius: var(--radius);
    background: var(--window-bg);
    color: var(--fg);
    box-shadow:
      0 0 0 1px var(--border),
      0 12px 32px rgb(0 0 0 / 25%);
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 30%);
  }
  header {
    position: relative;
    display: grid;
    place-items: center;
    min-height: 34px;
    margin-bottom: var(--space-3);
  }
  h2 {
    margin: 0;
    font: 700 14.5px/1.3 var(--font-ui);
  }
  .close {
    position: absolute;
    inset-inline-end: -10px;
    display: grid;
    place-items: center;
    width: var(--target-min);
    height: var(--target-min);
    padding: 0;
    border: 0;
    background: transparent;
    color: var(--fg);
  }
  .disc {
    display: grid;
    place-items: center;
    width: 26px;
    height: 26px;
    border-radius: 50%;
    background: var(--btn-bg);
  }
  ul {
    margin: var(--space-3) 0;
    padding: 0;
    list-style: none;
  }
  li {
    flex-direction: column;
    align-items: stretch;
  }
  .line {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-2) var(--space-3);
    min-height: var(--target-min);
  }
  .avatar {
    display: grid;
    flex: none;
    place-items: center;
    width: 32px;
    height: 32px;
    border-radius: 50%;
    background: var(--who);
    color: #000;
    font: 700 13px/1 var(--font-ui);
  }
  .avatar[data-person="5"] {
    color: #fff;
  }
  .controls {
    display: flex;
    flex: 0 1 auto;
    flex-wrap: wrap;
    align-items: center;
    justify-content: flex-end;
    gap: var(--space-2);
    margin-inline-start: auto;
  }
  .who {
    display: flex;
    flex: 1 1 100px;
    flex-direction: column;
    min-width: 0;
  }
  .name {
    overflow: hidden;
    font-size: 14px;
    font-weight: 700;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dim,
  .state,
  .role {
    color: var(--fg-dim);
    font-size: 13px;
    font-weight: 400;
  }
  .role {
    margin-inline-start: auto;
  }
  .state.online::before {
    content: "";
    display: inline-block;
    width: 8px;
    height: 8px;
    margin-inline-end: 6px;
    border-radius: 50%;
    background: var(--success);
  }
  [data-suspended="true"] .state {
    color: var(--warning);
  }
  select {
    box-sizing: border-box;
    min-height: var(--target-min);
    padding: 0 var(--space-2);
    border: 0;
    border-radius: var(--radius-button);
    background: var(--btn-bg);
    color: inherit;
    font: 14px var(--font-ui);
  }
  dialog :global(.btn) {
    min-height: var(--target-min);
  }
  .ask {
    margin-top: var(--space-2);
    padding: var(--space-3);
    border-radius: var(--radius-button);
    background: var(--warning-bg);
  }
  .ask p {
    margin: 0 0 var(--space-3);
    font-size: 14px;
  }
  .buttons {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
  }
  .danger {
    background: var(--error);
    color: light-dark(#fff, #1e1e1e);
  }
  .danger:hover:not(:disabled) {
    background: color-mix(in srgb, var(--error) 88%, #000);
  }
  .error {
    color: var(--error);
    font-size: 14px;
  }
  .note {
    margin: var(--space-2) 0 0;
    color: var(--fg-dim);
    font-size: 13px;
  }
</style>
