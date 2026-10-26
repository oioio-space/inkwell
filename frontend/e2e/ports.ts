import { createHash } from "node:crypto";
import { join, resolve } from "node:path";

// Each checkout gets its own block of eight ports, so e2e runs in several worktrees (parallel
// agents, or a second clone) never collide. INKWELL_E2E_PORT pins the base explicitly.
const checkout = resolve(import.meta.dirname, "..");
const derived = 18_000 + (createHash("sha256").update(checkout).digest().readUInt16BE(0) % 200) * 8;
const base = Number(process.env.INKWELL_E2E_PORT) || derived;

/** The first instance (Alice), the second (Bob), and a spare one started by a spec. */
export const ALICE_PORT = base;
export const BOB_PORT = base + 1;
export const SPARE_PORT = base + 2;

/** The relay of offline.spec.ts (UDP and TCP), and the two instances that spec starts itself. */
export const RELAY_PORT = base + 3;
export const OFFLINE_ALICE_PORT = base + 4;
export const OFFLINE_BOB_PORT = base + 5;

export const ALICE_URL = `http://127.0.0.1:${ALICE_PORT}/`;
export const BOB_URL = `http://127.0.0.1:${BOB_PORT}/`;
export const SPARE_URL = `http://127.0.0.1:${SPARE_PORT}/`;

/** Where an instance saves "Exporter en PDF" (INKWELL_EXPORT_DIR, set by playwright.config.ts). */
export const exportDir = (port: number) =>
  join(process.env.INKWELL_E2E_DOCS ?? "", `export-${port}`);
