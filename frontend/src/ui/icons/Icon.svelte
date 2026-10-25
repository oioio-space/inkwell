<script lang="ts">
import { ICONS, type IconName } from "./paths";

let { name, size = 16 }: { name: IconName; size?: number } = $props();
</script>

<svg class="ico" viewBox="0 0 16 16" width={size} height={size} aria-hidden="true" focusable="false">
  {@html ICONS[name]}
</svg>

<style>
  .ico {
    flex: none;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
</style>
