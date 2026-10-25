<script lang="ts">
import { t } from "../../i18n";
import type { UiMode } from "./history";

let {
  value = $bindable(""),
  mode = $bindable<UiMode>("normal"),
  extended = $bindable(false),
  caseSensitive = $bindable(false),
  wholeWord = $bindable(false),
  ignoreAccents = $bindable(false),
  dotAll = $bindable(false),
  visibleText = $bindable(false),
  error = "",
  onsubmit,
  onhistory,
}: {
  value?: string;
  mode?: UiMode;
  extended?: boolean;
  caseSensitive?: boolean;
  wholeWord?: boolean;
  ignoreAccents?: boolean;
  dotAll?: boolean;
  visibleText?: boolean;
  /** Plain-words reason the query cannot run; shown under the field. */
  error?: string;
  /** Enter (backwards = Shift+Enter). */
  onsubmit?: (backwards: boolean) => void;
  /** ArrowDown on an empty field asks for the previous searches. */
  onhistory?: () => void;
} = $props();

let input: HTMLInputElement;
const errorId = `search-error-${Math.random().toString(36).slice(2, 8)}`;

export function focus(select = true) {
  input?.focus();
  if (select) input?.select();
}

const modes: Array<{ id: UiMode; label: () => string; hint: () => string }> = [
  { id: "normal", label: () => t("search.mode.normal"), hint: () => t("search.mode.normal.hint") },
  {
    id: "wildcard",
    label: () => t("search.mode.wildcard"),
    hint: () => t("search.mode.wildcard.hint"),
  },
  { id: "regex", label: () => t("search.mode.regex"), hint: () => t("search.mode.regex.hint") },
];

function keydown(e: KeyboardEvent) {
  if (e.key === "Enter") {
    e.preventDefault();
    onsubmit?.(e.shiftKey);
  } else if (e.key === "ArrowDown" && value === "") {
    e.preventDefault();
    onhistory?.();
  }
}
</script>

<div class="field">
  <div class="line">
    <input
      bind:this={input}
      bind:value
      type="text"
      class:invalid={error !== ""}
      role="searchbox"
      aria-label={t("search.field.label")}
      aria-invalid={error !== ""}
      aria-describedby={error !== "" ? errorId : undefined}
      placeholder={t("search.field.placeholder")}
      autocomplete="off"
      autocapitalize="off"
      spellcheck="false"
      onkeydown={keydown}
    />
    <div class="modes" role="group" aria-label={t("search.mode.group")}>
      {#each modes as m (m.id)}
        <button
          type="button"
          class:on={mode === m.id}
          aria-pressed={mode === m.id}
          title={m.hint()}
          onclick={() => (mode = m.id)}
        >{m.label()}</button>
      {/each}
    </div>
  </div>

  <div class="chips" role="group" aria-label={t("search.options")}>
    <button type="button" class="chip" class:on={caseSensitive} aria-pressed={caseSensitive} onclick={() => (caseSensitive = !caseSensitive)}>{t("search.opt.case")}</button>
    <button type="button" class="chip" class:on={wholeWord && mode !== "regex"} aria-pressed={wholeWord && mode !== "regex"} disabled={mode === "regex"} title={mode === "regex" ? t("search.opt.word.regex") : undefined} onclick={() => (wholeWord = !wholeWord)}>{t("search.opt.word")}</button>
    <button type="button" class="chip" class:on={ignoreAccents} aria-pressed={ignoreAccents} onclick={() => (ignoreAccents = !ignoreAccents)}>{t("search.opt.accents")}</button>
    {#if mode === "regex"}
      <button type="button" class="chip" class:on={dotAll} aria-pressed={dotAll} onclick={() => (dotAll = !dotAll)}>{t("search.opt.dotall")}</button>
    {:else if mode === "normal"}
      <button type="button" class="chip" class:on={extended} aria-pressed={extended} title={t("search.opt.extended.hint")} onclick={() => (extended = !extended)}>{t("search.opt.extended")}</button>
    {/if}
    <button type="button" class="chip" class:on={visibleText} aria-pressed={visibleText} title={t("search.opt.visible.hint")} onclick={() => (visibleText = !visibleText)}>{t("search.opt.visible")}</button>
  </div>

  {#if error !== ""}
    <p id={errorId} class="error" role="alert">{error}</p>
  {/if}
</div>

<style>
  .field {
    display: grid;
    gap: var(--space-2);
  }
  .line {
    display: flex;
    gap: var(--space-2);
    align-items: center;
    flex-wrap: wrap;
  }
  input {
    flex: 1 1 14em;
    min-height: 34px;
    box-sizing: border-box;
    padding: 0 var(--space-3);
    border: 0;
    border-radius: 9px;
    background: color-mix(in srgb, var(--fg) 8%, transparent);
    color: var(--fg);
    font: 15px/1.3 var(--font-ui);
    box-shadow: inset 0 0 0 1px transparent;
  }
  input:focus-visible {
    outline: var(--focus-ring);
    outline-offset: 0;
  }
  input.invalid {
    box-shadow: inset 0 0 0 2px var(--warning);
  }
  .modes {
    display: inline-flex;
    padding: 2px;
    gap: 2px;
    border-radius: 11px;
    background: color-mix(in srgb, var(--fg) 8%, transparent);
  }
  .modes button {
    min-height: 30px;
    padding: 0 var(--space-3);
    border: 0;
    border-radius: 9px;
    background: transparent;
    color: var(--fg);
    font: 600 13.5px/1 var(--font-ui);
    cursor: pointer;
  }
  .modes button.on {
    background: var(--card-bg);
    box-shadow: 0 0 0 1px var(--border), 0 1px 2px rgb(0 0 6 / 12%);
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }
  .chip {
    min-height: 30px;
    padding: 0 var(--space-3);
    border: 0;
    border-radius: 9px;
    background: transparent;
    color: var(--fg-dim);
    font: 13.5px/1 var(--font-ui);
    cursor: pointer;
  }
  .chip:hover:not(:disabled) {
    background: color-mix(in srgb, var(--fg) 7%, transparent);
  }
  .chip.on {
    background: color-mix(in srgb, var(--accent) 18%, transparent);
    color: var(--accent);
    font-weight: 600;
  }
  .chip:disabled {
    opacity: 0.45;
    cursor: default;
  }
  .error {
    margin: 0;
    color: color-mix(in srgb, var(--warning) 55%, var(--fg));
    font-size: 13.5px;
  }
</style>
