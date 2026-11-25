<script lang="ts">
import ApplyDialog from "./ApplyDialog.svelte";
import DiffView from "./DiffView.svelte";
import { history } from "./history.svelte";

/** The history's two windows, mounted with the shell: a dialog never waits on the drawer's code. */
</script>

{#if history.diff}<DiffView req={history.diff} />{/if}
{#if history.apply}<ApplyDialog req={history.apply} />{/if}
