/**
 * Rule V of the local statistics and debug state (plan docs/plans/2026-11-24-obs1-observability.md
 * §2): a value is a finite number ≥ 0, a boolean, an enum value its contribution declares, or a
 * code of the diagnostics taxonomy (docs/plans/2026-11-09-diagnostics.md §3.3). No other string:
 * never text, a name, a path, a keystroke, a peer id, an address or a secret.
 */
import type { FieldType } from "./manifest";

/** The diagnostics taxonomy's event names (§3.3), a closed list: a code is one of them. */
export const CODES: ReadonlySet<string> = new Set([
  "app.start",
  "app.stop",
  "app.start_failed",
  "app.panic",
  "net.listen.ok",
  "net.listen.fallback",
  "net.listen.failed",
  "net.relay.reserve",
  "net.relay.lost",
  "net.path.change",
  "net.nat.status",
  "net.conn.open",
  "net.conn.close",
  "net.conn.fail",
  "net.send.cut",
  "net.frame.refused",
  "sync.resync.start",
  "sync.resync.done",
  "sync.diverged",
  "sync.apply.fail",
  "sync.state",
  "typst.compile.fail",
  "typst.sidecar.exit",
  "typst.sandbox.unavailable",
  "store.write.fail",
  "store.read.fail",
  "store.corrupt",
  "store.disk.low",
  "store.backup",
  "ui.error",
  "ui.freeze",
  "ui.render.fail",
  "ui.webview.crashed",
  "action.run",
  "sec.refused",
  "diag.log_unwritable",
]);

export type DebugValue = number | boolean | string;

/** A debug read once rule V has passed over it: what was kept, and how many values were not. */
export interface DebugRead {
  readonly values: Readonly<Record<string, DebugValue>>;
  readonly dropped: number;
}

/** The most values one read keeps; the rest is dropped. */
const MAX_VALUES = 1024;
/**
 * What may come before a field's name in a key (`3fa9c2e1d4b5a607.kind`, `2.kind`): a pseudonym
 * of 8 bytes in hex or an index, never a word a provider chose.
 */
const PREFIX = /^([0-9a-f]{16}|\d{1,4})$/;

/** The declared field a key names, whole or after a {@link PREFIX}; undefined for none. */
function fieldOf(key: string, fields: Readonly<Record<string, FieldType>>): FieldType | undefined {
  if (Object.hasOwn(fields, key)) return fields[key];
  const dot = key.indexOf(".");
  const name = key.slice(dot + 1);
  return dot > 0 && PREFIX.test(key.slice(0, dot)) && Object.hasOwn(fields, name)
    ? fields[name]
    : undefined;
}

/** Whether v passes rule V as a value of type `type`. */
export function passes(v: unknown, type: FieldType): boolean {
  if (type === "number") return typeof v === "number" && Number.isFinite(v) && v >= 0;
  if (type === "boolean") return typeof v === "boolean";
  if (type === "code") return typeof v === "string" && CODES.has(v);
  return typeof v === "string" && type.includes(v);
}

/**
 * Applies rule V to what a provider's `read()` returned: a flat record whose every key names a
 * declared field (whole, or after a pseudonym or an index) and whose every value passes that
 * field's type.
 * Anything else is dropped and counted, never kept: a provider that breaks the rule loses the
 * value, not the read.
 */
export function checkRead(raw: unknown, fields: Readonly<Record<string, FieldType>>): DebugRead {
  const values: Record<string, DebugValue> = Object.create(null);
  if (raw === null || typeof raw !== "object" || Array.isArray(raw))
    return Object.freeze({ values: Object.freeze(values), dropped: 1 });
  let dropped = 0;
  let kept = 0;
  for (const [key, v] of Object.entries(raw)) {
    const type = fieldOf(key, fields);
    if (kept < MAX_VALUES && type !== undefined && passes(v, type)) {
      values[key] = v as DebugValue;
      kept++;
    } else dropped++;
  }
  return Object.freeze({ values: Object.freeze(values), dropped });
}
