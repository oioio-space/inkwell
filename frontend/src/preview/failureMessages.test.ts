// Block SB1b (gates/SB1b.toml, criterion "messages"; ADR 0042 A7; plan
// docs/plans/2026-11-11-sandbox-crash-reports.md § 7): what the person reads
// when a compilation fails. The wording is confirmed at the UX review (D3);
// these rules are frozen.
import { flushSync } from "svelte";
import { afterEach, describe, expect, it } from "vitest";
import en from "../i18n/en.json";
import fr from "../i18n/fr.json";
import { t } from "../i18n/index";
import { cleanup, render } from "../ui/testkit.svelte";
import Preview from "./Preview.svelte";
import { preview } from "./preview.svelte";
import { FAILURE_KINDS, parseMessage } from "./protocol";

const catalogs: Record<string, Record<string, string>> = { en, fr };
const failKeys = (catalog: Record<string, string>) =>
  Object.keys(catalog)
    .filter((k) => k.startsWith("preview.fail."))
    .sort();

// A message says what to do (UX rule 5) when it holds one of these.
const actions: Record<string, RegExp> = {
  en: /\b(shorten|split|only open|restart|fix|add|remove)\b/i,
  fr: /\b(raccourcissez|découpez|ne l'ouvrez que|relancez|corrigez|ajoutez|retirez)\b/i,
};

afterEach(() => {
  cleanup();
});

describe("preview failure messages", () => {
  it("SB1b every failure kind has an English and a French message", () => {
    expect(FAILURE_KINDS).toContain("blocked");
    for (const [lang, catalog] of Object.entries(catalogs)) {
      for (const kind of FAILURE_KINDS) {
        const text = catalog[`preview.fail.${kind}`];
        expect(text, `${lang} ${kind}`).toBeTruthy();
        expect(text, `${lang} ${kind}`).not.toContain("preview.fail");
      }
    }
  });

  it("SB1b memory, tooLarge and blocked say what to do", () => {
    for (const [lang, catalog] of Object.entries(catalogs)) {
      for (const kind of ["memory", "tooLarge", "blocked"]) {
        expect(catalog[`preview.fail.${kind}`], `${lang} ${kind}`).toMatch(actions[lang] as RegExp);
      }
    }
  });

  it("SB1b failure messages use no jargon", () => {
    for (const catalog of Object.values(catalogs)) {
      for (const key of failKeys(catalog)) {
        expect(catalog[key], key).not.toMatch(
          /\b(sandbox|signal|seccomp|crash|core|bac à sable)\b/i,
        );
      }
    }
  });

  it("SB1b both languages hold the same preview.fail keys", () => {
    expect(failKeys(en)).toEqual(failKeys(fr));
    expect(failKeys(en)).toEqual(FAILURE_KINDS.map((k) => `preview.fail.${k}`).sort());
  });

  it("SB1b a failure shows only the fixed message of its kind", () => {
    const canary = "CANARY-5b1b";
    preview.receive({ type: "compiling", seq: 0 });
    preview.pages = new Map();
    preview.pageCount = 0;
    preview.hasResult = false;
    preview.failure = undefined;
    preview.diagnostics = [];
    const host = render(Preview, {});
    const frame = JSON.stringify({
      type: "result",
      seq: 1_000_000,
      rev: 1,
      first: 1,
      pageCount: 0,
      pages: [],
      diagnostics: [],
      truncated: false,
      failure: { kind: "blocked", packages: [canary], detail: canary, message: canary },
    });
    const msg = parseMessage(frame);
    expect(msg).toBeDefined();
    preview.receive(msg as NonNullable<typeof msg>);
    flushSync();
    const empty = host.querySelector('[data-testid="preview-empty"]');
    expect(empty?.textContent?.trim()).toBe(t("preview.fail.blocked"));
    expect(host.textContent).not.toContain(canary);
  });
});
