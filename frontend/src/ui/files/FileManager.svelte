<script lang="ts">
import { tick, untrack } from "svelte";
import { SvelteSet } from "svelte/reactivity";
import { byFileName, resolve } from "../../filetypes";
import { t } from "../../i18n";
import { typeOfEntry } from "../../project/kinds";
import type { Entry } from "../../project/materialize";
import type { OpError } from "../../project/ops";
import type { Outcome } from "../../project/ProjectIndex";
import type { ProjectRuntime } from "../../project/runtime.svelte";
import { workspace } from "../../workspace/workspace.svelte";
import FileIcon from "../icons/FileIcon.svelte";
import Icon from "../icons/Icon.svelte";
import MenuButton from "../popover/MenuButton.svelte";
import Toast from "../Toast.svelte";
import ContextMenu, { type MenuItem } from "./ContextMenu.svelte";
import { bringsFiles, bringsProjectFiles, collectDropped, fromList, INTERNAL_DRAG } from "./drop";
import { explain, formatSize } from "./explain";
import LimitNotice from "./LimitNotice.svelte";
import NewFileDialog, { type Template } from "./NewFileDialog.svelte";
import PurgeDialog from "./PurgeDialog.svelte";
import ReceptionProgress from "./ReceptionProgress.svelte";
import { buildRows, type Row, range, topmost } from "./rows";

let {
  runtime,
  onOpen,
}: {
  runtime: ProjectRuntime;
  /** A file was opened from the list: the editor takes focus back. */
  onOpen: () => void;
} = $props();

const ROW = 34;
const OVERSCAN = 8;

const tree = $derived(runtime.tree);
const role = $derived(runtime.role);
const canWrite = $derived(role === "writer" || role === "admin");
const isAdmin = $derived(role === "admin");
const direct = $derived(isAdmin || runtime.index.mode === "free");

const collapsed = new SvelteSet<string>();
const rows = $derived(buildRows(tree.entries, collapsed));
const byId = $derived(tree.byId);

let selected = $state.raw<ReadonlySet<string>>(new Set());
let anchor = $state<string | null>(null);
let focusId = $state<string | null>(null);
let renaming = $state<{ id: string; error: string } | null>(null);
let renameValue = $state("");
let menu = $state<{ x: number; y: number; ids: string[]; where: string | null } | null>(null);
let dropOn = $state<string | "root" | null>(null);
let external = $state(false);
let cut = $state.raw<readonly string[]>([]);
let trashView = $state(false);
let newFile = $state(false);
let newFileParent = $state<string | null>(null);
let purging = $state(false);
let notice = $state<{ text: string; tone: "status" | "alert" } | null>(null);
let noticeTimer: ReturnType<typeof setTimeout> | undefined;
let scroller: HTMLElement | undefined = $state();
let viewH = $state(0);
let scrollTop = $state(0);
let filePicker: HTMLInputElement | undefined = $state();
let folderPicker: HTMLInputElement | undefined = $state();
let moreOpen = $state(false);
let newOpen = $state(false);
let dragCount = 0;

const names = $derived(workspace.names);
const who = (peer: string) =>
  peer === workspace.myId ? workspace.myName : (names[peer] ?? `Personne ${peer.slice(0, 4)}`);

function say(text: string, tone: "status" | "alert" = "status") {
  clearTimeout(noticeTimer);
  notice = { text, tone };
  noticeTimer = setTimeout(() => (notice = null), tone === "alert" ? 9000 : 5000);
}

/** Shows why an operation was refused, and returns the text. */
function refused(error: OpError): string {
  const text = explain(error);
  say(text, "alert");
  return text;
}

// — Virtual list: only the rows in view are drawn, with room around —

const first = $derived(viewH > 0 ? Math.max(0, Math.floor(scrollTop / ROW) - OVERSCAN) : 0);
const last = $derived(
  viewH > 0 ? Math.min(rows.length, Math.ceil((scrollTop + viewH) / ROW) + OVERSCAN) : rows.length,
);
const shown = $derived(rows.slice(first, last));

function reveal(id: string) {
  const i = rows.findIndex((r) => r.entry.id === id);
  if (i < 0 || !scroller || viewH === 0) return;
  const top = i * ROW;
  if (top < scroller.scrollTop) scroller.scrollTop = top;
  else if (top + ROW > scroller.scrollTop + viewH) scroller.scrollTop = top + ROW - viewH;
  scrollTop = scroller.scrollTop;
}

async function focusRow(id: string) {
  focusId = id;
  reveal(id);
  await tick();
  scroller?.querySelector<HTMLElement>(`[data-id="${id}"]`)?.focus();
}

/** Unfolds the folders above `id`, so that it is on screen. */
function unfoldTo(id: string) {
  for (let p = byId.get(id)?.parentId ?? null; p !== null; p = byId.get(p)?.parentId ?? null) {
    collapsed.delete(p);
  }
}

// What is selected or focused must still exist.
$effect(() => {
  const ids = byId;
  untrack(() => {
    if ([...selected].some((id) => !ids.has(id))) {
      selected = new Set([...selected].filter((id) => ids.has(id)));
    }
    if (focusId !== null && !ids.has(focusId)) focusId = null;
  });
});

// The file shown is the one the list marks, and it stays visible.
$effect(() => {
  const id = runtime.currentId;
  if (id && byId.has(id)) untrack(() => unfoldTo(id));
});

const current = $derived(runtime.currentId);
const tabbable = $derived(
  focusId && rows.some((r) => r.entry.id === focusId)
    ? focusId
    : (rows.find((r) => r.entry.id === current)?.entry.id ?? rows[0]?.entry.id ?? null),
);

// — Selection —

function select(ids: Iterable<string>, newAnchor?: string) {
  selected = new Set(ids);
  if (newAnchor !== undefined) anchor = newAnchor;
}

/** What an action applies to: the selection if the row is in it, else the row alone. */
function targets(id: string): string[] {
  return selected.has(id) ? topmost(rows, selected) : [id];
}

function onRowClick(e: MouseEvent, row: Row) {
  const id = row.entry.id;
  focusId = id;
  if (e.shiftKey && anchor) {
    select(range(rows, anchor, id));
    return;
  }
  if (e.ctrlKey || e.metaKey) {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    select(next, id);
    return;
  }
  select([id], id);
  if (row.entry.type === "dir") toggle(id);
  else open(id);
}

function toggle(id: string, open?: boolean) {
  const want = open ?? collapsed.has(id);
  if (want) collapsed.delete(id);
  else collapsed.add(id);
}

function open(id: string) {
  runtime.openFile(id);
  onOpen();
}

// — Keyboard —

function move(delta: number, extend: boolean) {
  const at = rows.findIndex((r) => r.entry.id === (focusId ?? tabbable));
  const to = Math.max(0, Math.min(rows.length - 1, (at < 0 ? 0 : at) + delta));
  const row = rows[to];
  if (!row) return;
  void focusRow(row.entry.id);
  if (extend) select(range(rows, anchor ?? row.entry.id, row.entry.id));
}

function onKey(e: KeyboardEvent) {
  if (renaming || e.target instanceof HTMLInputElement) return;
  const id = focusId ?? tabbable;
  const row = rows.find((r) => r.entry.id === id);
  const mod = e.ctrlKey || e.metaKey;
  switch (e.key) {
    case "ArrowDown":
      e.preventDefault();
      move(1, e.shiftKey);
      return;
    case "ArrowUp":
      e.preventDefault();
      move(-1, e.shiftKey);
      return;
    case "Home":
      e.preventDefault();
      move(-rows.length, e.shiftKey);
      return;
    case "End":
      e.preventDefault();
      move(rows.length, e.shiftKey);
      return;
    case "ArrowRight":
      if (!row) return;
      e.preventDefault();
      if (row.entry.type === "dir") {
        if (!row.expanded) toggle(row.entry.id, true);
        else if (row.hasChildren) move(1, false);
      }
      return;
    case "ArrowLeft": {
      if (!row) return;
      e.preventDefault();
      if (row.entry.type === "dir" && row.expanded) toggle(row.entry.id, false);
      else if (row.entry.parentId) void focusRow(row.entry.parentId);
      return;
    }
    case "Enter":
      if (!row) return;
      e.preventDefault();
      if (row.entry.type === "dir") toggle(row.entry.id);
      else open(row.entry.id);
      return;
    case " ":
      if (!row) return;
      e.preventDefault();
      {
        const next = new Set(selected);
        if (next.has(row.entry.id)) next.delete(row.entry.id);
        else next.add(row.entry.id);
        select(next, row.entry.id);
      }
      return;
    case "F2":
      if (row && canWrite) {
        e.preventDefault();
        startRename(row.entry.id);
      }
      return;
    case "Delete":
      if (row && canWrite) {
        e.preventDefault();
        remove(targets(row.entry.id));
      }
      return;
    case "F10":
      if (e.shiftKey && row) {
        e.preventDefault();
        openMenuAt(row.entry.id);
      }
      return;
    case "ContextMenu":
      if (row) {
        e.preventDefault();
        openMenuAt(row.entry.id);
      }
      return;
    case "Escape":
      if (cut.length > 0) cut = [];
      else select([]);
      return;
    case "a":
      if (mod) {
        e.preventDefault();
        select(rows.map((r) => r.entry.id));
      }
      return;
    case "x":
      if (mod && canWrite && row) {
        e.preventDefault();
        cut = targets(row.entry.id);
      }
      return;
    case "v":
      if (mod && canWrite && cut.length > 0) {
        e.preventDefault();
        const dest = row ? (row.entry.type === "dir" ? row.entry.id : row.entry.parentId) : null;
        moveTo(cut, dest);
        cut = [];
      }
      return;
  }
}

function openMenuAt(id: string) {
  const el = scroller?.querySelector<HTMLElement>(`[data-id="${id}"]`);
  const box = el?.getBoundingClientRect();
  menu = {
    x: (box?.left ?? 0) + 24,
    y: (box?.bottom ?? 0) - 4,
    ids: targets(id),
    where: id,
  };
}

// — Actions —

function startRename(id: string) {
  const e = byId.get(id);
  if (!e || !canWrite) return;
  renaming = { id, error: "" };
  renameValue = e.name;
  void tick().then(() => {
    const input = scroller?.querySelector<HTMLInputElement>("input.rename");
    input?.focus();
    const dot = e.type === "dir" ? -1 : e.name.lastIndexOf(".");
    input?.setSelectionRange(0, dot > 0 ? dot : e.name.length);
  });
}

function commitRename(): boolean {
  if (!renaming) return true;
  const e = byId.get(renaming.id);
  if (!e || renameValue === e.name) {
    renaming = null;
    return true;
  }
  const out = runtime.index.rename(renaming.id, renameValue);
  if (!out.ok) {
    renaming = { ...renaming, error: explain(out.error) };
    return false;
  }
  const id = renaming.id;
  renaming = null;
  void focusRow(id);
  return true;
}

function cancelRename() {
  const id = renaming?.id;
  renaming = null;
  if (id) void focusRow(id);
}

function result(out: Outcome): boolean {
  if (out.ok) return true;
  refused(out.error);
  return false;
}

function remove(ids: string[]) {
  if (ids.length === 0) return;
  const out = runtime.index.remove(ids);
  if (!result(out)) return;
  say(t(direct ? "files.deletion.done" : "files.deletion.asked"));
}

function moveTo(ids: readonly string[], parent: string | null): string | undefined {
  const out = runtime.index.move([...ids], parent);
  if (out.ok) {
    if (parent !== null) toggle(parent, true);
    return undefined;
  }
  return refused(out.error);
}

async function copyPath(id: string) {
  const e = byId.get(id);
  if (!e) return;
  try {
    await navigator.clipboard.writeText(e.path);
    say(t("files.copied"));
  } catch {
    say(e.path);
  }
}

function newFolder(parent: string | null) {
  const name = t("files.name.folder");
  let free = name;
  const taken = new Set(
    tree.entries.filter((e) => e.parentId === parent).map((e) => e.name.toLowerCase()),
  );
  for (let n = 2; taken.has(free.toLowerCase()); n++) free = `${name}-${n}`;
  const out = runtime.index.createFolder(parent, free);
  if (!result(out) || !out.ok) return;
  const id = out.ids[0] as string;
  if (parent !== null) toggle(parent, true);
  select([id], id);
  void tick().then(() => {
    focusId = id;
    startRename(id);
  });
}

function askNewFile(parent: string | null) {
  newFileParent = parent;
  newFile = true;
}

function createFile(name: string, template: Template): string | undefined {
  // A name that says what the file is (.md, .typ) wins over the template picked.
  const ext = name.trim().slice(name.trim().lastIndexOf(".")).toLowerCase();
  // Any other extension (.py, .txt, .json) is a plain file: the Typst or Markdown template would
  // put a heading in a script.
  const kind: Template =
    ext === ".md"
      ? "markdown"
      : ext === ".typ"
        ? "typst"
        : /^\.[a-z0-9]+$/.test(ext)
          ? "text"
          : template;
  const body =
    kind === "typst"
      ? t("files.template.typst.body")
      : kind === "markdown"
        ? t("files.template.markdown.body")
        : "";
  const out = runtime.index.createFile(newFileParent, name.trim(), body);
  if (!out.ok) return explain(out.error);
  const id = out.ids[0] as string;
  if (newFileParent !== null) toggle(newFileParent, true);
  select([id], id);
  open(id);
  return undefined;
}

async function importFiles(files: ReturnType<typeof fromList>, parent: string | null) {
  if (!canWrite || files.length === 0) return;
  const res = await runtime.importFiles(files, parent);
  if (!res.outcome.ok) {
    refused(res.outcome.error);
    return;
  }
  if (parent !== null) toggle(parent, true);
  const lines: string[] = [];
  if (res.plan.files > 0) lines.push(t("files.import.done", { count: res.plan.files }));
  if (res.plan.renamed.length > 0) {
    lines.push(t("files.import.renamed", { count: res.plan.renamed.length }));
  }
  for (const s of res.plan.skipped) {
    lines.push(
      s.reason === "too_large"
        ? t("files.import.tooLarge", { name: s.path, limit: formatSize(s.limit ?? 0) })
        : t("files.import.failed", { name: s.path }),
    );
  }
  say(lines.join(" · "), res.plan.skipped.length > 0 ? "alert" : "status");
}

async function addFromDisk(path: string) {
  try {
    result(await runtime.addFromDisk(path));
  } catch (err) {
    say(t("files.error.store", { reason: err instanceof Error ? err.message : "" }), "alert");
  }
}

async function updateFromDisk(id: string) {
  try {
    const out = await runtime.updateFromDisk(id);
    if (!out.ok && out.error.code === "unchanged") say(t("files.error.unchanged"));
    else result(out);
  } catch (err) {
    say(t("files.error.store", { reason: err instanceof Error ? err.message : "" }), "alert");
  }
}

// — Drag and drop —

function dragStart(e: DragEvent, row: Row) {
  if (!canWrite || renaming) {
    e.preventDefault();
    return;
  }
  const ids = targets(row.entry.id);
  e.dataTransfer?.setData(INTERNAL_DRAG, JSON.stringify(ids));
  e.dataTransfer?.setData("text/plain", row.entry.path);
  if (e.dataTransfer) e.dataTransfer.effectAllowed = "move";
}

/** The folder a drop on this row goes into: the row itself if it is a folder, else its folder. */
const dropTarget = (e: Entry): string | null => (e.type === "dir" ? e.id : e.parentId);

function overRow(e: DragEvent, row: Row) {
  if (!canWrite) return;
  const internal = bringsProjectFiles(e.dataTransfer);
  if (!internal && !bringsFiles(e.dataTransfer)) return;
  e.preventDefault();
  e.stopPropagation();
  if (e.dataTransfer) e.dataTransfer.dropEffect = internal ? "move" : "copy";
  dropOn = dropTarget(row.entry) ?? "root";
}

async function dropRow(e: DragEvent, row: Row | null) {
  e.preventDefault();
  e.stopPropagation();
  const parent = row ? dropTarget(row.entry) : null;
  dropOn = null;
  external = false;
  dragCount = 0;
  const dt = e.dataTransfer;
  if (!dt || !canWrite) return;
  const raw = dt.getData(INTERNAL_DRAG);
  if (raw) {
    try {
      const ids = (JSON.parse(raw) as unknown[]).filter((x): x is string => typeof x === "string");
      moveTo(ids, parent);
    } catch {
      // not ours
    }
    return;
  }
  const files = await collectDropped(dt);
  await importFiles(files, parent);
}

function overPanel(e: DragEvent) {
  if (!canWrite || !bringsFiles(e.dataTransfer)) return;
  e.preventDefault();
  external = true;
  if (dropOn === null) dropOn = "root";
}

function enterPanel(e: DragEvent) {
  if (bringsFiles(e.dataTransfer)) dragCount++;
}

function leavePanel(e: DragEvent) {
  if (!bringsFiles(e.dataTransfer)) return;
  dragCount = Math.max(0, dragCount - 1);
  if (dragCount === 0) {
    external = false;
    dropOn = null;
  }
}

// — Menu —

const menuItems = $derived.by((): MenuItem[] => {
  if (!menu) return [];
  const ids = menu.ids;
  const one = ids.length === 1 ? byId.get(ids[0] as string) : undefined;
  const items: MenuItem[] = [];
  if (one && one.type !== "dir") {
    items.push({
      id: "open",
      label: t("files.menu.open"),
      keys: "↵",
      onchoose: () => open(one.id),
    });
  }
  if (one) {
    items.push({
      id: "path",
      label: t("files.menu.copyPath"),
      onchoose: () => void copyPath(one.id),
    });
  }
  if (canWrite) {
    if (one) {
      items.push({
        id: "rename",
        label: t("files.menu.rename"),
        keys: "F2",
        onchoose: () => startRename(one.id),
      });
    }
    items.push({
      id: "dup",
      label: t("files.menu.duplicate"),
      onchoose: () => result(runtime.index.duplicate(ids)),
    });
    const parent = one ? (one.type === "dir" ? one.id : one.parentId) : null;
    items.push({ separator: true });
    items.push({ id: "nf", label: t("files.menu.newHere"), onchoose: () => askNewFile(parent) });
    items.push({
      id: "nd",
      label: t("files.menu.newFolderHere"),
      onchoose: () => newFolder(parent),
    });
    if (one?.type === "blob") {
      items.push({
        id: "upd",
        label: t("files.disk.update"),
        onchoose: () => void updateFromDisk(one.id),
      });
    }
    if (isAdmin && one?.type === "text" && !one.main) {
      items.push({
        id: "main",
        label: t("files.menu.setMain"),
        onchoose: () => result(runtime.index.setMain(one.id)),
      });
    }
    items.push({ separator: true });
    if (isAdmin && ids.every((id) => byId.get(id)?.deletion)) {
      items.push({
        id: "accept",
        label: t("files.menu.accept"),
        onchoose: () => result(runtime.index.accept(ids)),
      });
      items.push({
        id: "refuse",
        label: t("files.menu.refuse"),
        onchoose: () => result(runtime.index.refuse(ids)),
      });
    }
    items.push({
      id: "delete",
      label: t(direct ? "files.menu.delete" : "files.menu.deleteAsk"),
      keys: t("files.menu.keys.delete"),
      danger: true,
      disabled: ids.some((id) => id === tree.mainId),
      onchoose: () => remove(ids),
    });
  }
  return items;
});

function openMenu(e: MouseEvent, id: string) {
  e.preventDefault();
  if (!selected.has(id)) select([id], id);
  focusId = id;
  menu = { x: e.clientX, y: e.clientY, ids: targets(id), where: id };
}

function closeMenu() {
  const back = menu?.where;
  menu = null;
  if (back && byId.has(back)) void focusRow(back);
}

// — Trash —

const trashCount = $derived(tree.trash.length);
const pending = $derived(tree.pending);

$effect(() => {
  if (trashView && trashCount === 0) trashView = false;
});

function restore(ids: string[]) {
  result(runtime.index.restore(ids));
}

function purgeAll() {
  result(runtime.index.purge(tree.trash.map((x) => x.id)));
}
</script>

<div
  class="manager"
  class:external
  role="presentation"
  ondragenter={enterPanel}
  ondragover={overPanel}
  ondragleave={leavePanel}
  ondrop={(e) => void dropRow(e, null)}
>
  <div class="bar">
    {#if canWrite}
      <MenuButton bind:open={newOpen} label={t("files.add")} name={t("files.add")} width={260} align="start">
        {#snippet trigger()}
          <Icon name="plus" size={14} /><Icon name="down" size={10} />
        {/snippet}
        {#snippet children({ close })}
          <button
            type="button"
            class="mi"
            role="menuitem"
            onclick={() => {
              close(false);
              askNewFile(null);
            }}
          >
            <Icon name="fileplus" /><span>{t("files.new")}</span>
          </button>
          <button
            type="button"
            class="mi"
            role="menuitem"
            onclick={() => {
              close(false);
              newFolder(null);
            }}
          >
            <Icon name="folderplus" /><span>{t("files.newFolder")}</span>
          </button>
          <div class="sep"></div>
          <button
            type="button"
            class="mi"
            role="menuitem"
            onclick={() => {
              close(false);
              filePicker?.click();
            }}
          >
            <Icon name="upload" /><span>{t("files.import")}</span>
          </button>
          <button
            type="button"
            class="mi"
            role="menuitem"
            onclick={() => {
              close(false);
              folderPicker?.click();
            }}
          >
            <Icon name="upload" /><span>{t("files.importFolder")}</span>
          </button>
        {/snippet}
      </MenuButton>
    {/if}
    <span class="grow"></span>
    {#if trashCount > 0 || trashView}
      <button
        type="button"
        class="fb"
        class:on={trashView}
        aria-pressed={trashView}
        aria-label={t(trashView ? "files.trash.hide" : "files.trash.show", { count: trashCount })}
        title={t(trashView ? "files.trash.hide" : "files.trash.show", { count: trashCount })}
        onclick={() => (trashView = !trashView)}
      >
        <Icon name="trash" size={14} />
        <span class="count">{trashCount}</span>
      </button>
    {/if}
    {#if isAdmin}
      <MenuButton bind:open={moreOpen} label={t("files.more")} name={t("files.more")} width={300} align="end">
        {#snippet trigger()}
          <Icon name="more" size={14} />
        {/snippet}
        {#snippet children({ close })}
          <div class="group" role="presentation">{t("files.menu.rule")}</div>
          {#each ["approval", "free"] as const as mode (mode)}
            <button
              type="button"
              class="mi"
              role="menuitemradio"
              aria-checked={runtime.index.mode === mode}
              onclick={() => {
                close(false);
                result(runtime.index.setMode(mode));
              }}
            >
              <span>{t(mode === "approval" ? "files.rule.approval" : "files.rule.free")}</span>
              {#if runtime.index.mode === mode}<Icon name="check" />{/if}
            </button>
          {/each}
        {/snippet}
      </MenuButton>
    {/if}
  </div>

  {#if pending.length > 0}
    <ul class="requests" aria-label={t("files.deletion.requests", { count: pending.length })}>
      {#each pending as e (e.id)}
        <li>
          <span class="ask">{t("files.deletion.request", { name: e.name })}</span>
          {#if isAdmin}
            <span class="btns">
              <button type="button" class="btn" onclick={() => result(runtime.index.accept([e.id]))}>
                {t("files.deletion.accept")}
              </button>
              <button type="button" class="btn" onclick={() => result(runtime.index.refuse([e.id]))}>
                {t("files.deletion.refuse")}
              </button>
            </span>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}

  {#if trashView}
    <div class="trash" aria-label={t("files.trash")} role="region">
      {#if tree.trash.length === 0}
        <p class="dim">{t("files.trash.empty")}</p>
      {:else}
        <ul>
          {#each tree.trash as item (item.id)}
            <li>
              <FileIcon mark={item.type === "dir" ? "folder" : resolve({ name: item.name, isText: item.type === "text" }).icon} />
              <span class="what">
                <span class="name">{item.path}</span>
                <span class="dim">{t("files.trash.by", { who: who(item.by) })}</span>
              </span>
              <button
                type="button"
                class="fb"
                aria-label="{t('files.trash.restore')} {item.name}"
                title={t("files.trash.restore")}
                onclick={() => restore([item.id])}
              >
                <Icon name="restore" size={14} />
              </button>
            </li>
          {/each}
        </ul>
        {#if isAdmin}
          <button type="button" class="btn purge" onclick={() => (purging = true)}>
            {t("files.trash.purge")}
          </button>
        {/if}
      {/if}
    </div>
  {:else}
    <nav
      bind:this={scroller}
      bind:clientHeight={viewH}
      class="scroll"
      aria-label={t("sidebar.files")}
      onscroll={(e) => (scrollTop = e.currentTarget.scrollTop)}
    >
      {#if rows.length === 0}
        <p class="dim empty">{t("files.empty")}</p>
      {/if}
      <!-- svelte-ignore a11y_interactive_supports_focus -->
      <div
        class="tree"
        class:drop-root={dropOn === "root"}
        role="tree"
        aria-label={t("files.tree")}
        aria-multiselectable="true"
        onkeydown={onKey}
        ondragover={(e) => {
          if (canWrite && (bringsProjectFiles(e.dataTransfer) || bringsFiles(e.dataTransfer))) {
            e.preventDefault();
            dropOn = "root";
          }
        }}
        ondrop={(e) => void dropRow(e, null)}
      >
        <div class="spacer" aria-hidden="true" style:height="{first * ROW}px"></div>
        {#each shown as row (row.entry.id)}
          {@const e = row.entry}
          {@const sel = selected.has(e.id)}
          <!-- The keys are handled once, by the tree that holds the rows (roving tabindex). -->
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <div
            class="row"
            class:sel
            class:cur={e.id === current}
            class:cut={cut.includes(e.id)}
            class:drop={dropOn === e.id}
            class:asked={e.deletion !== undefined}
            role="treeitem"
            tabindex={e.id === tabbable ? 0 : -1}
            aria-level={row.depth + 1}
            aria-posinset={row.pos}
            aria-setsize={row.size}
            aria-selected={sel}
            aria-expanded={e.type === "dir" ? row.expanded : undefined}
            aria-current={e.id === current ? "page" : undefined}
            data-id={e.id}
            data-path={e.path}
            style:--depth={row.depth}
            draggable={canWrite && renaming?.id !== e.id}
            onclick={(ev) => onRowClick(ev, row)}
            onfocus={() => (focusId = e.id)}
            oncontextmenu={(ev) => openMenu(ev, e.id)}
            ondragstart={(ev) => dragStart(ev, row)}
            ondragover={(ev) => overRow(ev, row)}
            ondragleave={() => {
              if (dropOn === e.id) dropOn = null;
            }}
            ondrop={(ev) => void dropRow(ev, row)}
            ondragend={() => (dropOn = null)}
          >
            {#if e.type === "dir"}
              <button
                type="button"
                class="chev"
                class:open={row.expanded}
                tabindex="-1"
                aria-hidden="true"
                onclick={(ev) => {
                  ev.stopPropagation();
                  toggle(e.id);
                }}
              >
                <Icon name="right" size={12} />
              </button>
            {:else}
              <span class="chev spacer-chev"></span>
            {/if}
            <FileIcon mark={e.type === "dir" ? "folder" : typeOfEntry(e).icon} />
            {#if renaming?.id === e.id}
              <input
                class="rename"
                aria-label={t("files.rename.label", { name: e.name })}
                aria-invalid={renaming.error ? "true" : undefined}
                bind:value={renameValue}
                spellcheck="false"
                autocomplete="off"
                onclick={(ev) => ev.stopPropagation()}
                onkeydown={(ev) => {
                  ev.stopPropagation();
                  if (ev.key === "Enter") {
                    ev.preventDefault();
                    commitRename();
                  } else if (ev.key === "Escape") {
                    ev.preventDefault();
                    cancelRename();
                  }
                }}
                onblur={() => {
                  if (renaming && !commitRename()) cancelRename();
                }}
              />
            {:else}
              <span class="name" title={e.renamedFrom ? e.name : undefined}>{e.name}</span>
              {#if e.main}<span class="tag">{t("sidebar.main")}</span>{/if}
              {#if e.deletion}<span class="tag warn">{t("files.deletion.pending")}</span>{/if}
              {#if e.refused}
                <span class="tag">{t("files.deletion.refused", { who: who(e.refused.by) })}</span>
              {/if}
            {/if}
          </div>
          {#if renaming?.id === e.id && renaming.error}
            <p class="rename-error" role="alert" style:--depth={row.depth}>{renaming.error}</p>
          {/if}
        {/each}
        <div class="spacer" aria-hidden="true" style:height="{(rows.length - last) * ROW}px"></div>
      </div>

      {#if runtime.untracked.length > 0 && canWrite}
        <section class="disk" aria-label={t("files.disk.title")}>
          <h3>{t("files.disk.title")}</h3>
          <ul>
            {#each runtime.untracked as f (f.path)}
              <li>
                <FileIcon mark={byFileName(f.path)?.icon ?? "other"} />
                <span class="name" title={f.path}>{f.path}</span>
                <button type="button" class="btn" onclick={() => void addFromDisk(f.path)}>
                  {t("files.disk.add")}
                </button>
              </li>
            {/each}
          </ul>
        </section>
      {/if}
    </nav>
  {/if}

  <LimitNotice excess={tree.warnings.find((w) => w.kind === "capped")?.count ?? 0} />
  <ReceptionProgress {runtime} />

  {#if external}
    <div class="overlay" aria-hidden="true"><span>{t("files.drop")}</span></div>
  {/if}
</div>

<input
  bind:this={filePicker}
  class="picker"
  type="file"
  multiple
  tabindex="-1"
  aria-hidden="true"
  data-testid="file-picker"
  onchange={(e) => {
    void importFiles(fromList(e.currentTarget.files), null);
    e.currentTarget.value = "";
  }}
/>
<input
  bind:this={folderPicker}
  class="picker"
  type="file"
  multiple
  webkitdirectory
  tabindex="-1"
  aria-hidden="true"
  data-testid="folder-picker"
  onchange={(e) => {
    void importFiles(fromList(e.currentTarget.files), null);
    e.currentTarget.value = "";
  }}
/>

{#if menu}
  <ContextMenu
    x={menu.x}
    y={menu.y}
    label={t("files.menu.label", {
      name: menu.ids.length === 1 ? (byId.get(menu.ids[0] as string)?.name ?? "") : t("files.selected", { count: menu.ids.length }),
    })}
    items={menuItems}
    onclose={closeMenu}
  />
{/if}

<NewFileDialog
  bind:open={newFile}
  where={newFileParent ? (byId.get(newFileParent)?.path ?? "") : ""}
  oncreate={createFile}
/>
<PurgeDialog bind:open={purging} items={tree.trash} onconfirm={purgeAll} />

{#if notice}
  <Toast role={notice.tone} onclose={() => (notice = null)}>{notice.text}</Toast>
{/if}

<style>
  .manager {
    position: relative;
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    flex: none;
    padding: 0 6px 6px;
  }
  .grow {
    flex: 1;
  }
  .count {
    margin-inline-start: 4px;
    font-size: 12px;
  }
  .scroll {
    flex: 1;
    min-height: 0;
    padding: 0 6px 6px;
    overflow-y: auto;
  }
  .tree {
    min-height: 100%;
    border-radius: var(--radius-small);
  }
  .tree.drop-root {
    box-shadow: inset 0 0 0 2px color-mix(in srgb, var(--accent-bg) 55%, transparent);
  }
  .row {
    box-sizing: border-box;
    display: flex;
    align-items: center;
    gap: 8px;
    height: 34px;
    padding: 0 10px 0 calc(6px + var(--depth, 0) * 18px);
    border-radius: var(--radius-small);
    cursor: default;
    user-select: none;
  }
  .row:hover {
    background: var(--hover-bg);
  }
  .row.sel {
    background: var(--selected-bg);
  }
  .row.cur {
    font-weight: 600;
  }
  .row:focus-visible {
    outline: var(--focus-ring);
    outline-offset: -2px;
  }
  .row.cut {
    opacity: 0.5;
  }
  .row.drop {
    background: color-mix(in srgb, var(--accent-bg) 22%, transparent);
    box-shadow: inset 0 0 0 2px color-mix(in srgb, var(--accent-bg) 65%, transparent);
  }
  .row.asked .name {
    text-decoration: line-through;
    text-decoration-thickness: 1.5px;
  }
  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .tag {
    flex: none;
    color: var(--fg-dim);
    font-size: 12px;
    font-weight: 400;
  }
  .tag.warn {
    color: var(--warning);
  }
  .chev {
    display: inline-grid;
    place-items: center;
    flex: none;
    width: 16px;
    height: 16px;
    padding: 0;
    border: 0;
    background: transparent;
    color: var(--fg-dim);
    transition: transform var(--duration);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  .rename {
    flex: 1;
    min-width: 0;
    height: 26px;
    padding: 0 6px;
    border: 0;
    border-radius: var(--radius-small);
    background: var(--view-bg);
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent-bg) 75%, transparent);
    color: inherit;
    font: inherit;
  }
  .rename[aria-invalid="true"] {
    box-shadow: 0 0 0 2px var(--warning);
  }
  .rename-error {
    margin: 0 4px 4px;
    padding: 4px 10px 4px calc(30px + var(--depth, 0) * 18px);
    color: var(--warning);
    font-size: 13px;
  }
  .dim {
    color: var(--fg-dim);
    font-size: 13px;
  }
  .empty {
    padding: 8px 10px;
  }
  .requests {
    flex: none;
    margin: 0 6px 6px;
    padding: 6px 8px;
    list-style: none;
    border-radius: var(--radius-small);
    background: var(--warning-bg);
    font-size: 13px;
  }
  .requests li {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 4px 0;
  }
  .btns {
    display: flex;
    gap: 6px;
  }
  .btns :global(.btn) {
    min-height: 30px;
    padding: 0 12px;
  }
  .trash {
    flex: 1;
    min-height: 0;
    padding: 0 6px 6px;
    overflow-y: auto;
  }
  .trash ul,
  .disk ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .trash li,
  .disk li {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 40px;
    padding: 2px 8px;
    border-radius: var(--radius-small);
  }
  .trash li:hover {
    background: var(--hover-bg);
  }
  .what {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-width: 0;
  }
  .what .name {
    flex: none;
  }
  .purge {
    width: 100%;
    margin-top: 8px;
    min-height: 36px;
  }
  .disk {
    margin-top: 12px;
    padding-top: 8px;
    border-top: 1px solid var(--border);
  }
  .disk h3 {
    margin: 0 8px 4px;
    color: var(--fg-dim);
    font: 600 12px var(--font-ui);
  }
  .disk .btn {
    flex: none;
    min-height: 28px;
    padding: 0 10px;
    font-size: 13px;
  }
  .group {
    padding: 4px 10px;
    color: var(--fg-dim);
    font-size: 12px;
  }
  .sep {
    height: 1px;
    margin: 4px 6px;
    background: var(--border);
  }
  .overlay {
    position: absolute;
    inset: 0;
    z-index: 5;
    display: grid;
    place-items: center;
    border-radius: var(--radius-small);
    background: color-mix(in srgb, var(--accent-bg) 18%, var(--sidebar-bg));
    box-shadow: inset 0 0 0 2px color-mix(in srgb, var(--accent-bg) 65%, transparent);
    font-weight: 600;
    pointer-events: none;
  }
  .picker {
    position: fixed;
    width: 0;
    height: 0;
    opacity: 0;
    pointer-events: none;
  }
  @media (prefers-reduced-motion: reduce) {
    .chev {
      transition: none;
    }
  }
</style>
