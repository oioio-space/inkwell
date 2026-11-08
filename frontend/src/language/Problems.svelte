<script lang="ts">
import { t } from "../i18n";
import { explain } from "./messages";
import { problems } from "./problems.svelte";
</script>
<section aria-label={t("problems.tab")} class="problem-list">
 {#if problems.entries.length===0}<p>{t("problems.none")}</p>{/if}
 {#each [...new Set(problems.entries.map(d=>d.file))] as file}
  <h3>{file}</h3>
  {#each problems.entries.filter(d=>d.file===file) as d}
   {@const help=explain(d.message)}
   <article class:stale={d.stale}>
    <button type="button" class="mi two" disabled={d.stale} onclick={()=>void problems.jump(d)}>
      <span>{d.severity==="error"?t("problems.error"):t("problems.warning")} : {help.summary}</span>
      <span class="sub">{t("problems.where",{file:d.file,line:(d.position?.start.line??0)+1})}</span>
    </button>
    <p>{help.action}</p>
    {#if d.stale}<p>{t("problems.stale")}</p>{/if}
    <details><summary>{t("problems.technical")}</summary><pre>{d.message}</pre></details>
   </article>
  {/each}
 {/each}
</section>
<style>
.problem-list {padding:12px;overflow:auto;} h3{font-size:var(--text-small);color:var(--fg-dim);} article{padding:8px 0;border-bottom:1px solid var(--shade);} p{font-size:var(--text-small);margin:4px 8px;} pre{white-space:pre-wrap;overflow-wrap:anywhere;} .stale{opacity:var(--disabled-opacity);} button{white-space:normal;text-align:start;}
</style>
