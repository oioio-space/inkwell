<script lang="ts">
import type { LanguageSupport } from "@codemirror/language";
import { registry } from "../../../extensions";
import type { NativeViewerProps } from "../../../extensions/context";
import Editor from "../../../ui/Editor.svelte";
import ReadOnlyText from "../../../ui/ReadOnlyText.svelte";

/** The text editor: the main document's, or a read-only text (a projection, a text too big). */
let { opening, handle, big, me, lang, startOffset, readOnlyReason }: NativeViewerProps = $props();

const grammar = $derived(opening.type.grammar);
const loaded = (g: string | undefined) => (g ? registry.loadedSupport(g) : undefined);
let support = $state.raw<LanguageSupport | undefined>();
// The grammar's colours come once its entry has loaded (at once for Typst, loaded at start).
$effect.pre(() => {
  const g = grammar;
  support = loaded(g);
  if (!g || support) return;
  let live = true;
  void registry.languageSupport(g).then((s) => {
    if (live) support = s;
  });
  return () => {
    live = false;
  };
});
</script>

{#if handle}
  <Editor {handle} {me} {lang} fileType={opening.type.id} {support} at={startOffset} />
{:else if big}
  <ReadOnlyText text={big.text} message={readOnlyReason} {support} at={startOffset} />
{/if}
