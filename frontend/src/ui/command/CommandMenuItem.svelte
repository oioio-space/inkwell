<script lang="ts">
import { type Snippet, tick } from "svelte";
import { availability, cmdAttrs, isOn, run, shortcutOf } from "../../extensions/commands";
import Icon from "../icons/Icon.svelte";

/**
 * A menu item bound to a command: the registry's label and chord, `run()` on activation. A toggle
 * is a `menuitemcheckbox`; an unavailable command is greyed (`aria-disabled`), and activating it
 * says why and runs nothing.
 */
let {
  id,
  close,
  target,
  disabled = false,
  refocus,
  detail,
  checked,
  role,
  lead,
  onrun,
  class: cls = "",
  ...rest
}: {
  id: string;
  /** Closes the menu it is in; `refocus` gives the editor its focus back. */
  close: (refocus?: boolean) => void;
  target?: Element | undefined;
  /** Greyed for a reason the menu knows (the zoom at its bounds). */
  disabled?: boolean;
  /** Default: a toggle gives the editor its focus back, an action does not. */
  refocus?: boolean | undefined;
  /** Text after the label (the syntax a heading writes). */
  detail?: string | undefined;
  /** Checked by what the surface shows (bold at the caret, the heading level), for a command that is no toggle. */
  checked?: boolean | undefined;
  /** `menuitemradio` for a choice among levels; otherwise from the command. */
  role?: "menuitemradio" | undefined;
  /** An icon or glyph before the label. */
  lead?: Snippet;
  /** After the menu closed and the command ran (the grid's bar gives the grid its focus back). */
  onrun?: (() => void) | undefined;
  class?: string;
  [key: `data-${string}`]: string | undefined;
} = $props();

const attrs = $derived(cmdAttrs(id));
const state = $derived(availability(id, target));
const on = $derived(checked ?? isOn(id, target));
const chord = $derived(shortcutOf(id));
const label = $derived(attrs["aria-label"] ?? id);
</script>

<button
  type="button"
  class="mi {cls}"
  role={role ?? (on === undefined ? "menuitem" : "menuitemcheckbox")}
  aria-checked={on}
  {...rest}
  {...attrs}
  aria-label={undefined}
  aria-disabled={state.ok && !disabled ? undefined : "true"}
  disabled={disabled || undefined}
  onclick={() => {
    if (!state.ok) {
      run(id, target);
      return;
    }
    close(refocus ?? on !== undefined);
    // A radio item already chosen stays chosen: the same level again would make a paragraph.
    if (role !== "menuitemradio" || !on) run(id, target);
    // Once the menu is gone from the page: what takes the focus then keeps it.
    if (onrun) void tick().then(onrun);
  }}
>
  {@render lead?.()}
  <span>{label}</span>
  {#if role && on}<Icon name="check" />{/if}
  {#if detail || chord}<span class="k" class:mono={role !== undefined}>{[detail, chord].filter(Boolean).join(" · ")}</span>{/if}
  {#if on !== undefined && role === undefined && checked === undefined}<span class="tick" class:off={!on}><Icon name="check" size={14} /></span>{/if}
</button>

<style>
  .tick {
    margin-inline-start: auto;
    display: inline-flex;
  }
  .tick.off {
    visibility: hidden;
  }
  .k + .tick {
    margin-inline-start: 8px;
  }
</style>
