<script lang="ts">
import { run } from "../../../../extensions/commands";
import { t, tn } from "../../../../i18n";
import { problems } from "../../../../language/problems.svelte";
import { preview } from "../../../../preview/preview.svelte";
import type { StatusItemProps } from "../../../../ui/command/status.svelte";
import Icon from "../../../../ui/icons/Icon.svelte";
import MenuButton from "../../../../ui/popover/MenuButton.svelte";

/**
 * The problems of the document, for a Typst file (`fileType == typst`). In the main window the
 * counter steps through them (its command), Shift+click or a right click opens their list;
 * elsewhere it opens the compilation's list.
 */
let { item, bar }: StatusItemProps = $props();

let open = $state(false);
const errors = $derived(bar.problemsNav ? problems.errors : preview.errors);
const warnings = $derived(bar.problemsNav ? problems.warnings : preview.warnings);
const name = $derived(
  errors + warnings === 0
    ? t("problems.none")
    : [
        errors > 0 ? tn("problems.errors", errors) : "",
        warnings > 0 ? tn("problems.warnings", warnings) : "",
      ]
        .filter(Boolean)
        .join(", "),
);
const next = () => {
  if (item.command) run(item.command);
};
</script>

{#if bar.problemsNav}
  <!-- The problems are the compilation's: a Typst file takes part in it, a note or a picture does not (UX-L2-1). -->
  {#if preview.hasResult || problems.entries.length > 0}
    <button type="button" class="fb" title={t("problems.hint")} aria-label={`${t("problems.label")} : ${t("problems.next")}, ${name}`} data-cmd={item.command} onclick={(event) => event.shiftKey ? problems.show() : next()} oncontextmenu={(event) => {event.preventDefault();problems.show();}}>
      <span class="problems" data-testid="problems" data-errors={problems.errors} data-warnings={problems.warnings}>
        {#if problems.errors + problems.warnings === 0}
          <Icon name="check" size={13} /><span class="ptext">{t("problems.none")}</span>
        {:else}
          {#if problems.errors > 0}<span class="n err"><Icon name="fail" size={13} />{problems.errors}</span>{/if}
          {#if problems.warnings > 0}<span class="n warn"><Icon name="warn" size={13} />{problems.warnings}</span>{/if}
        {/if}
      </span>
    </button>
  {/if}
{:else if bar.ofMain && preview.hasResult}
  <MenuButton bind:open up align="end" width={460} label={t("problems.label")} name={`${t("problems.label")} : ${name}`}>
    {#snippet trigger()}
      <span class="problems" data-testid="problems" data-errors={preview.errors} data-warnings={preview.warnings}>
        {#if preview.errors + preview.warnings === 0}
          <Icon name="check" size={13} /><span class="ptext">{t("problems.none")}</span>
        {:else}
          {#if preview.errors > 0}<span class="n err"><Icon name="fail" size={13} />{preview.errors}</span>{/if}
          {#if preview.warnings > 0}<span class="n warn"><Icon name="warn" size={13} />{preview.warnings}</span>{/if}
        {/if}
      </span>
    {/snippet}
    {#snippet children({ close })}
      {#if preview.diagnostics.length === 0}
        <p class="none">{t("problems.empty")}</p>
      {/if}
      {#each preview.diagnostics as d, i (i)}
        <button
          type="button"
          class="mi two problem"
          role="menuitem"
          data-severity={d.severity}
          disabled={d.line === undefined}
          onclick={() => {
            close(false);
            bar.onProblem(d);
          }}
        >
          <Icon name={d.severity === "error" ? "fail" : "warn"} size={14} />
          <span class="text">
            {d.message.split("\n")[0]}
            {#if d.line !== undefined}
              <span class="sub">{t("problems.where", { file: d.file ?? "", line: d.line })}</span>
            {/if}
          </span>
        </button>
      {/each}
      {#if preview.truncated}<p class="none">{t("problems.more")}</p>{/if}
    {/snippet}
  </MenuButton>
{/if}

<style>
  .problems {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    font-size: 12.5px;
  }
  .problems .n {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-variant-numeric: tabular-nums;
  }
  .problems .err {
    color: var(--error);
  }
  .problems .warn {
    color: var(--warning);
  }
</style>
