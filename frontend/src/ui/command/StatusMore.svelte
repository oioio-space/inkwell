<script lang="ts">
import { run, textOf } from "../../extensions/commands";
import type { StatusItem } from "../../extensions/registry";
import { t } from "../../i18n";
import Icon from "../icons/Icon.svelte";
import MenuButton from "../popover/MenuButton.svelte";

/**
 * The status items left over for lack of room (the extensions' group past 3, all of it in a narrow bar),
 * by their names; choosing one runs its command (§4.31 row 5).
 */
let { items }: { items: readonly StatusItem[] } = $props();

let open = $state(false);
</script>

<MenuButton bind:open up align="end" width={260} label={t("status.more")} name={t("status.more")}>
  {#snippet trigger()}<Icon name="more" />{/snippet}
  {#snippet children({ close })}
    {#each items as x (x.id)}
      <button
        type="button"
        class="mi"
        role="menuitem"
        disabled={x.command === undefined}
        onclick={() => {
          close(false);
          if (x.command) run(x.command);
        }}>{textOf(x, x.label)}</button
      >
    {/each}
  {/snippet}
</MenuButton>
