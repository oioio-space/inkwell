<script lang="ts">
import { t } from "../../../../i18n";
import type { StatusItemProps } from "../../../../ui/command/status.svelte";

/** The file's language. */
let { bar }: StatusItemProps = $props();
</script>

<span class="fb lang" data-testid="language">{t(`lang.${bar.lang}`)}</span>
