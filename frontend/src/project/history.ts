import { next as A } from "@automerge/automerge";
import { t } from "../i18n";
import type { DeletionMode } from "./model";
import type { MessageItem, OpKind, OpMessage } from "./ops";

/**
 * What happened to the files, read back from the index's own history (B5, for B2's panel).
 * Every operation of {@link ./ops} writes a message with its change; this module reads them.
 * They are declarations of the people who made the changes, so the text is cleaned before it is
 * shown, and the time is that of their device ("selon l'appareil de X", X-20).
 */

/** One operation on the files, in words. */
export interface FileOpEntry {
  /** The hash of the change: a stable key, and what "Revenir" would point at. */
  hash: string;
  /** Milliseconds, by the clock of the device that made the change. */
  time: number;
  op: OpKind;
  /** The device of the author, and the name they gave. */
  by: string;
  who: string;
  items: MessageItem[];
  /** How many files the operation touched (`items` may have been cut). */
  count: number;
  mode?: DeletionMode;
  /** The sentence for the panel, in the language of the interface. */
  text: string;
}

const OPS: ReadonlySet<string> = new Set<OpKind>([
  "create",
  "rename",
  "move",
  "trash",
  "request",
  "accept",
  "refuse",
  "restore",
  "purge",
  "main",
  "mode",
  "replace",
]);

/** Control characters and bidirectional overrides, which could make a name read as another. */
// biome-ignore lint/suspicious/noControlCharactersInRegex: that is the point
const UNSAFE = /[\u0000-\u001f\u007f‪-‮⁦-⁩]/g;

const clean = (v: unknown, max: number): string =>
  typeof v === "string" ? v.replace(UNSAFE, "").slice(0, max) : "";

function parseItem(v: unknown): MessageItem | undefined {
  if (v === null || typeof v !== "object") return undefined;
  const o = v as Record<string, unknown>;
  const id = clean(o.id, 40);
  if (id === "") return undefined;
  const item: MessageItem = { id, name: clean(o.name, 255) };
  if (o.type === "dir" || o.type === "text" || o.type === "blob") item.type = o.type;
  if (typeof o.from === "string") item.from = clean(o.from, 1024);
  if (typeof o.to === "string") item.to = clean(o.to, 1024);
  if (o.fromId === null || typeof o.fromId === "string") item.fromId = o.fromId as string | null;
  if (o.toId === null || typeof o.toId === "string") item.toId = o.toId as string | null;
  return item;
}

/** Reads the message of a change; undefined if it is not one of ours or is malformed. */
export function parseMessage(raw: string | null): OpMessage | undefined {
  if (!raw?.startsWith('{"k":"file"') || raw.length > 20_000) return undefined;
  let v: unknown;
  try {
    v = JSON.parse(raw);
  } catch {
    return undefined;
  }
  if (v === null || typeof v !== "object") return undefined;
  const o = v as Record<string, unknown>;
  if (o.k !== "file" || o.v !== 1 || typeof o.op !== "string" || !OPS.has(o.op)) return undefined;
  const items = Array.isArray(o.items)
    ? o.items.slice(0, 100).flatMap((i) => parseItem(i) ?? [])
    : undefined;
  const msg: OpMessage = {
    k: "file",
    v: 1,
    op: o.op as OpKind,
    by: clean(o.by, 64),
    who: clean(o.who, 80),
  };
  if (items) msg.items = items;
  if (typeof o.n === "number" && Number.isSafeInteger(o.n) && o.n > 0) msg.n = o.n;
  if (o.mode === "approval" || o.mode === "free") msg.mode = o.mode;
  return msg;
}

const where = (path: string | undefined) => (path ? path : t("history.root"));

/** The sentence for an entry, in the language of the interface. */
export function describe(m: OpMessage): string {
  const items = m.items ?? [];
  const count = m.n ?? items.length;
  const first = items[0];
  const name = first?.name ?? "";
  const subject = count > 1 ? t("history.many", { name, count: count - 1 }) : name;
  const who = m.who || t("history.someone");
  switch (m.op) {
    case "create":
      return t("history.create", { who, name: subject });
    case "rename":
      return t("history.rename", { who, from: first?.from ?? "", name });
    case "move":
      return t("history.move", { who, name: subject, to: where(first?.to) });
    case "trash":
      return t("history.trash", { who, name: subject });
    case "request":
      return t("history.request", { who, name: subject });
    case "accept":
      return t("history.accept", { who, name: subject });
    case "refuse":
      return t("history.refuse", { who, name: subject });
    case "restore":
      return t("history.restore", { who, name: subject });
    case "purge":
      return t("history.purge", { who, name: subject });
    case "main":
      return t("history.main", { who, name });
    case "mode":
      return t(m.mode === "free" ? "history.mode.free" : "history.mode.approval", { who });
    case "replace":
      return t("history.replace", { who, name });
  }
}

/**
 * The file operations of an index, oldest first, kept up to date by reading only the changes
 * that came since the last look. The order is the causal order of the changes, never the clocks.
 */
export class FileHistory {
  #entries: FileOpEntry[] = [];
  #heads: A.Heads = [];

  /** Reads the changes made since the last call. */
  update(doc: A.Doc<unknown>): void {
    const heads = A.getHeads(doc);
    if (heads.length === this.#heads.length && heads.every((h) => this.#heads.includes(h))) return;
    for (const meta of A.getChangesMetaSince(doc, this.#heads)) {
      const m = parseMessage(meta.message);
      if (!m) continue;
      this.#entries.push({
        hash: meta.hash,
        time: meta.time * 1000,
        op: m.op,
        by: m.by,
        who: m.who,
        items: m.items ?? [],
        count: m.n ?? m.items?.length ?? 0,
        ...(m.mode ? { mode: m.mode } : {}),
        text: describe(m),
      });
    }
    this.#heads = heads;
  }

  /** The operations made after `since` (milliseconds on the authors' clocks; 0 for all). */
  since(since = 0): FileOpEntry[] {
    return this.#entries.filter((e) => e.time > since);
  }
}
