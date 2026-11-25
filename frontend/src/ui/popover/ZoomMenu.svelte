<script lang="ts">
import { t } from "../../i18n";
import { ZOOM_MAX, ZOOM_MIN, zoom } from "../../store/zoom.svelte";
import { workspace } from "../../workspace/workspace.svelte";
import CommandMenuItem from "../command/CommandMenuItem.svelte";
import MenuButton from "./MenuButton.svelte";

/**
 * The text zoom of this device (50 to 300 %): the figure in the status bar opens its menu, whose
 * items are the `app.zoom-*` commands. The menu stays open so a step can be repeated.
 */
let open = $state(false);
const level = $derived(zoom.level(workspace.project));
const stay = () => {};
</script>

<MenuButton bind:open label={t("zoom.menu")} up align="end" width={260} triggerClass="zoom">
  {#snippet trigger()}<span data-testid="text-zoom">{level} %</span>{/snippet}
  {#snippet children()}
    <div class="cap">{t("zoom.title")}</div>
    <CommandMenuItem id="app.zoom-out" close={stay} disabled={level <= ZOOM_MIN} />
    <CommandMenuItem id="app.zoom-in" close={stay} disabled={level >= ZOOM_MAX} />
    <CommandMenuItem id="app.zoom-reset" close={stay} disabled={level === 100} />
    <p class="line sub">{t("zoom.note")}</p>
  {/snippet}
</MenuButton>
