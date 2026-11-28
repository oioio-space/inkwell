<script lang="ts">
import { type MessageKey, t } from "../../../../i18n";
import type { StatusItemProps } from "../../../../ui/command/status.svelte";

/** The file's language: its grammar's name (`label`), plain text without a format entry. */
let { bar }: StatusItemProps = $props();
</script>

<span class="fb lang" data-testid="language">{t((bar.lang?.label ?? "lang.plain") as MessageKey)}</span>
