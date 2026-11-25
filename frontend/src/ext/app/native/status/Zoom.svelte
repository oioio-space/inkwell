<script lang="ts">
import { menuOf } from "../../../../extensions/commands";
import { t } from "../../../../i18n";
import { ZOOM_MAX, ZOOM_MIN, zoom } from "../../../../store/zoom.svelte";
import CommandMenuItem from "../../../../ui/command/CommandMenuItem.svelte";
import MenuButton from "../../../../ui/popover/MenuButton.svelte";
import { workspace } from "../../../../workspace/workspace.svelte";

/**
 * The text zoom of this device (50 to 300 %): the figure opens its menu, the `status/zoom`
 * location (`app.zoom-*`). The menu stays open so a step can be repeated.
 */

let open = $state(false);
const level = $derived(zoom.level(workspace.project));
const items = $derived(open ? menuOf("status/zoom").flatMap((g) => g.items) : []);
/** A step that would leave the range, or the reset at 100 %, is greyed. */
const atBound = (id: string) =>
  id === "app.zoom-out"
    ? level <= ZOOM_MIN
    : id === "app.zoom-in"
      ? level >= ZOOM_MAX
      : level === 100;
const stay = () => {};
</script>

<MenuButton bind:open label={t("zoom.menu")} up align="end" width={260} triggerClass="zoom">
  {#snippet trigger()}<span data-testid="text-zoom">{level} %</span>{/snippet}
  {#snippet children()}
    <div class="cap">{t("zoom.title")}</div>
    {#each items as c (c.id)}
      <CommandMenuItem id={c.id} close={stay} disabled={atBound(c.id)} />
    {/each}
    <p class="line sub">{t("zoom.note")}</p>
  {/snippet}
</MenuButton>
