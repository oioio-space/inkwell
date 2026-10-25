<script lang="ts">
import { t } from "../../i18n";
import { Eol, type FileInfo } from "../../store/files";
import Icon from "../icons/Icon.svelte";
import { encodingLabel, eolLabel, needsIndicator } from "./encoding";
import MenuButton from "./MenuButton.svelte";

/**
 * How the file is stored on disk. The indicator only exists when it is not UTF-8 with LF line
 * ends; the menu shows both and offers to convert (wired by the saving work, so disabled).
 */
let { info }: { info: FileInfo | undefined } = $props();

let open = $state(false);
const label = $derived(info ? `${encodingLabel(info.encoding)} · ${eolLabel(info.eol)}` : "");

const EOLS = [
  [Eol.LF, "encoding.eol.lf.hint"],
  [Eol.CRLF, "encoding.eol.crlf.hint"],
  [Eol.CR, "encoding.eol.cr.hint"],
] as const;
</script>

{#if info && needsIndicator(info)}
  <MenuButton bind:open label={t("encoding.menu")} up align="end" width={340} triggerClass="encoding">
    {#snippet trigger()}<span class="warn">{label}</span>{/snippet}
    {#snippet children()}
      <div class="cap">{t("encoding.title")}</div>
      <p class="line" data-testid="encoding-current">{t("encoding.current", { encoding: encodingLabel(info.encoding) })}</p>
      <span class="tipwrap block">
        <button
          type="button"
          class="mi"
          role="menuitem"
          aria-disabled="true"
          aria-describedby="encoding-soon"
          onclick={(e) => e.preventDefault()}
        >
          <span>{t("encoding.convert")}</span>
        </button>
        <span class="tip" role="tooltip" id="encoding-soon">{t("encoding.soon")}</span>
      </span>
      <div class="sep"></div>
      <div class="cap">{t("encoding.eol.title")}</div>
      {#each EOLS as [eol, hint] (eol)}
        <div class="mi static" role="menuitemradio" aria-checked={info.eol === eol} tabindex="-1">
          <span>{eolLabel(eol)}</span>
          {#if info.eol === eol}<Icon name="check" />{/if}
          <span class="k">{t(hint)}</span>
        </div>
      {/each}
      {#if info.mixed}
        <p class="line sub">{t("encoding.mixed")}</p>
      {/if}
      <div class="sep"></div>
      <p class="line sub">{t("encoding.footnote")}</p>
    {/snippet}
  </MenuButton>
{/if}

<style>
  .warn {
    color: var(--warning);
    font-weight: 700;
  }
  .line {
    margin: 0;
    padding: 4px 10px 6px;
  }
  .block {
    display: flex;
  }
  .block :global(.tip) {
    top: auto;
    bottom: calc(100% + 4px);
  }
  .static {
    cursor: default;
  }
  .mi :global(.ico) {
    color: var(--accent);
  }
</style>
