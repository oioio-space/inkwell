/**
 * The network's debug provider (rule V-3 of plan docs/plans/2026-11-24-obs1-observability.md):
 * Go's snapshot, flattened. Each link is keyed by its pseudonym for the session, never its peer
 * id; the registry passes the record through rule V. It runs only when it is read (Z-4).
 */
import { StatsService } from "../../../../bindings/github.com/oioio-space/inkwell/internal/app/index";

export async function read(): Promise<Record<string, number | string>> {
  const links = (await StatsService.Network()).links ?? [];
  const out: Record<string, number | string> = { links: links.length };
  for (const { peer, ...fields } of links)
    for (const [name, value] of Object.entries(fields)) out[`${peer}.${name}`] = value;
  return out;
}
