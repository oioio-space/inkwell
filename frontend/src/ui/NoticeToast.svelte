<script lang="ts">
import { notice } from "./notice.svelte";
import Toast from "./Toast.svelte";
</script>

{#if notice.text}
  <Toast role={notice.role} onclose={() => notice.hide()}>{notice.text}</Toast>
{/if}
