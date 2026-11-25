<script lang="ts">
import { closeDialog, dialogs } from "./dialogs.svelte";

/** Where the palette or the shortcuts window shows, once loaded (ui/command/dialogs.svelte.ts). */
</script>

{#if dialogs.open && dialogs.component}
  {@const Dialog = dialogs.component}
  {#key dialogs.open}
    <Dialog target={dialogs.target} again={dialogs.again} start={dialogs.start} onclose={closeDialog} />
  {/key}
{/if}
