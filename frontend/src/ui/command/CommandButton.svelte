<script lang="ts">
import type { Snippet } from "svelte";
import { availability, cmdAttrs, isOn, run } from "../../extensions/commands";

/**
 * A button bound to a command (ADR 0027): its name, title, chord and `data-cmd` come from the
 * registry, and a click goes through `run()`, never anywhere else. A toggle keeps one label and
 * says its state with `aria-pressed`; an unavailable command is greyed and says why when used.
 */
let {
  id,
  target,
  class: cls = "fb",
  testid,
  describedby,
  keepFocus = false,
  pressed,
  children,
  ...rest
}: {
  id: string;
  /** The surface the command acts on (the editor, for the formatting bar). */
  target?: Element | undefined;
  class?: string;
  testid?: string;
  /** A tooltip of its own that shows the label and the chord: then no `title`, one tooltip only. */
  describedby?: string;
  /** A mouse press leaves the focus where it is (the formatting bar keeps the caret). */
  keepFocus?: boolean;
  /** Pressed by what the surface shows (bold at the caret), for a command that is no toggle. */
  pressed?: boolean | undefined;
  children: Snippet;
  /** `data-*` attributes the surrounding view needs (`data-bar`). */
  [key: `data-${string}`]: string | undefined;
} = $props();

const attrs = $derived(cmdAttrs(id));
const state = $derived(availability(id, target));
const toggled = $derived(isOn(id));
const on = $derived(pressed ?? toggled);
</script>

<button
  type="button"
  class={cls}
  class:on={toggled}
  {...rest}
  {...attrs}
  title={state.reason ?? (describedby ? undefined : attrs.title)}
  aria-pressed={on}
  aria-disabled={state.ok ? undefined : "true"}
  aria-describedby={describedby}
  data-testid={testid}
  onmousedown={(e) => {
    if (keepFocus) e.preventDefault();
  }}
  onclick={() => run(id, target)}
>
  {@render children()}
</button>
