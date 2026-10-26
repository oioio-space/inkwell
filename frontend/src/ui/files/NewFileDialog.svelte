<script lang="ts" module>
export type Template = "typst" | "markdown" | "text";
</script>

<script lang="ts">
import { tick } from "svelte";
import { t } from "../../i18n";
import Icon from "../icons/Icon.svelte";

/** A new file: a name and a starting point (a Typst, a Markdown or a plain text document). */
let {
  open = $bindable(false),
  where,
  oncreate,
}: {
  open: boolean;
  /** The folder it goes in, for the title ("" is the root). */
  where: string;
  /** Makes the file; returns why it could not, or nothing when it is made. */
  oncreate: (name: string, template: Template) => string | undefined;
} = $props();

const TEMPLATES: Template[] = ["typst", "markdown", "text"];
const defaultName = (tpl: Template) => t(`files.name.${tpl}`);

let dialog: HTMLDialogElement | undefined = $state();
let input: HTMLInputElement | undefined = $state();
let template = $state<Template>("typst");
let name = $state("");
let typed = false;
let error = $state("");

$effect(() => {
  if (open && !dialog?.open) {
    template = "typst";
    name = defaultName("typst");
    typed = false;
    error = "";
    dialog?.showModal();
    void tick().then(() => {
      input?.focus();
      // Select the name without its extension: typing replaces the stem.
      const dot = name.lastIndexOf(".");
      input?.setSelectionRange(0, dot > 0 ? dot : name.length);
    });
  }
  if (!open && dialog?.open) dialog.close();
});

function pick(tpl: Template) {
  template = tpl;
  if (!typed) name = defaultName(tpl);
}

function submit(e: SubmitEvent) {
  e.preventDefault();
  const why = oncreate(name, template);
  if (why) {
    error = why;
    input?.focus();
  } else open = false;
}
</script>

<dialog bind:this={dialog} onclose={() => (open = false)} aria-labelledby="newfile-title">
  <form onsubmit={submit}>
    <header>
      <h2 id="newfile-title">{t("files.new.title")}</h2>
      {#if where}<p class="where">{where}</p>{/if}
      <button type="button" class="close" aria-label={t("files.cancel")} onclick={() => (open = false)}>
        <span class="disc"><Icon name="close" size={12} /></span>
      </button>
    </header>

    <fieldset class="card kinds">
      <legend>{t("files.new.kind")}</legend>
      {#each TEMPLATES as tpl (tpl)}
        <label class="kind">
          <input type="radio" name="template" checked={template === tpl} onchange={() => pick(tpl)} />
          <span>{t(`files.template.${tpl}`)}</span>
        </label>
      {/each}
    </fieldset>

    <label class="field card">
      <span>{t("files.new.name")}</span>
      <input
        bind:this={input}
        bind:value={name}
        oninput={() => (typed = true)}
        spellcheck="false"
        autocomplete="off"
        required
        aria-invalid={error ? "true" : undefined}
        aria-describedby={error ? "newfile-error" : undefined}
      />
    </label>
    {#if error}<p id="newfile-error" class="error" role="alert">{error}</p>{/if}

    <div class="actions">
      <button type="button" class="btn" onclick={() => (open = false)}>{t("files.cancel")}</button>
      <button type="submit" class="sug">{t("files.new.create")}</button>
    </div>
  </form>
</dialog>

<style>
  dialog {
    box-sizing: border-box;
    width: min(420px, calc(100vw - 32px));
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
  .where {
    margin: 2px 0 0;
    color: var(--fg-dim);
    font-size: 13px;
  }
  .close {
    position: absolute;
    inset-inline-end: -10px;
    inset-block-start: -6px;
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
  .card {
    display: block;
    margin: var(--space-3) 0;
    padding: var(--space-2) var(--space-3);
    border: 0;
  }
  legend {
    padding: 0;
    color: var(--fg-dim);
    font-size: 13px;
  }
  .kinds {
    display: flex;
    flex-direction: column;
  }
  .kind {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-height: var(--target-min);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }
  .field input {
    box-sizing: border-box;
    min-height: var(--target-min);
    padding: 0 var(--space-2);
    border: 0;
    border-radius: var(--radius-button);
    background: var(--btn-bg);
    color: inherit;
    font: 14px var(--font-ui);
  }
  .error {
    margin: 0 var(--space-3);
    color: var(--warning);
    font-size: 14px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-2);
    margin-top: var(--space-3);
  }
  .actions :global(.btn),
  .actions :global(.sug) {
    min-height: var(--target-min);
  }
</style>
