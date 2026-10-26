<script lang="ts">
import { t } from "../../i18n";
import { ZOOM_MAX, ZOOM_MIN, zoom } from "../../store/zoom.svelte";
import { workspace } from "../../workspace/workspace.svelte";
import MenuButton from "./MenuButton.svelte";

/** The text zoom of this device (50 to 300 %): the figure in the status bar opens its menu. */
const KEYS = { out: "Ctrl+−", in: "Ctrl+=", reset: "Ctrl+0" };
let open = $state(false);
const level = $derived(zoom.level(workspace.project));
const project = $derived(workspace.project);
</script>

<MenuButton bind:open label={t("zoom.menu")} up align="end" width={260} triggerClass="zoom">
  {#snippet trigger()}<span data-testid="text-zoom">{level} %</span>{/snippet}
  {#snippet children()}
    <div class="cap">{t("zoom.title")}</div>
    <button type="button" class="mi" role="menuitem" disabled={level <= ZOOM_MIN} onclick={() => zoom.out(project)}>
      <span>{t("zoom.out")}</span><span class="k">{KEYS.out}</span>
    </button>
    <button type="button" class="mi" role="menuitem" disabled={level >= ZOOM_MAX} onclick={() => zoom.in(project)}>
      <span>{t("zoom.in")}</span><span class="k">{KEYS.in}</span>
    </button>
    <button type="button" class="mi" role="menuitem" disabled={level === 100} onclick={() => zoom.reset(project)}>
      <span>{t("zoom.reset")}</span><span class="k">{KEYS.reset}</span>
    </button>
    <p class="line sub">{t("zoom.note")}</p>
  {/snippet}
</MenuButton>
