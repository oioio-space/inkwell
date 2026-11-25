<script lang="ts">
import type { StatusItemProps } from "../../../../ui/command/status.svelte";
import EncodingMenu from "../../../../ui/popover/EncodingMenu.svelte";

/** How the file is stored, shown only when it is unusual (its menu is a value picker). */
let { bar }: StatusItemProps = $props();
</script>

<EncodingMenu info={bar.info} />
