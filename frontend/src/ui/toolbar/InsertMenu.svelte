<script lang="ts">
import type { EditorState, StateCommand } from "@codemirror/state";
import {
  type InsertArgs,
  type InsertItem,
  insert,
  insertPrecondition,
  type Lang,
  relativePath,
  supports,
} from "../../editor/format";
import { type MessageKey, t } from "../../i18n";
import { FileKind, type ProjectFile } from "../../store/files";
import FileIcon from "../icons/FileIcon.svelte";
import Icon from "../icons/Icon.svelte";
import type { IconName } from "../icons/paths";
import MenuButton from "../popover/MenuButton.svelte";
import { bibKeys, parseCsv, typstLabels } from "./csv";

let {
  lang,
  editorState,
  run,
  files,
  currentFile,
  readFile,
}: {
  lang: Exclude<Lang, "plain">;
  editorState: EditorState;
  run: (command: StateCommand) => boolean;
  files: readonly ProjectFile[];
  currentFile: string;
  /** The text of a project file (for the CSV copy and the citation keys). */
  readFile: (path: string) => Promise<string | undefined>;
} = $props();

type Group = "files" | "refs" | "layout";
interface Entry {
  item: InsertItem;
  group: Group;
  icon: IconName;
  syntax: string;
  /** What the item asks for before it writes anything. */
  needs: "nothing" | "file" | "key";
  kinds?: readonly FileKind[];
  /** Markdown's substitute for an item it lacks (a link to the file). */
  substitute?: true;
}

const ANY = [
  FileKind.typst,
  FileKind.markdown,
  FileKind.text,
  FileKind.csv,
  FileKind.bib,
  FileKind.image,
  FileKind.pdf,
  FileKind.data,
  FileKind.other,
] as const;

const ENTRIES: readonly Entry[] = [
  {
    item: "image",
    group: "files",
    icon: "image",
    syntax: "#image()",
    needs: "file",
    kinds: [FileKind.image, FileKind.pdf],
  },
  {
    item: "figure",
    group: "files",
    icon: "figure",
    syntax: "#figure()",
    needs: "file",
    kinds: [FileKind.image, FileKind.pdf],
  },
  {
    item: "include",
    group: "files",
    icon: "include",
    syntax: "#include",
    needs: "file",
    kinds: [FileKind.typst],
  },
  {
    item: "attach",
    group: "files",
    icon: "paperclip",
    syntax: "#pdf.attach()",
    needs: "file",
    kinds: ANY,
  },
  {
    item: "csvTable",
    group: "files",
    icon: "csvtable",
    syntax: "csv()",
    needs: "file",
    kinds: [FileKind.csv],
  },
  {
    item: "data",
    group: "files",
    icon: "data",
    syntax: "json()",
    needs: "file",
    kinds: [FileKind.data],
  },
  {
    item: "bibliography",
    group: "files",
    icon: "book",
    syntax: "#bibliography()",
    needs: "file",
    kinds: [FileKind.bib],
  },
  {
    item: "link",
    group: "files",
    icon: "link",
    syntax: "[](…)",
    needs: "file",
    kinds: ANY,
    substitute: true,
  },
  { item: "cite", group: "refs", icon: "cite", syntax: "@clé", needs: "key" },
  { item: "ref", group: "refs", icon: "at", syntax: "@étiquette", needs: "key" },
  { item: "footnote", group: "refs", icon: "footnote", syntax: "#footnote[]", needs: "nothing" },
  { item: "outline", group: "layout", icon: "outline", syntax: "#outline()", needs: "nothing" },
  {
    item: "pagebreak",
    group: "layout",
    icon: "pagebreak",
    syntax: "#pagebreak()",
    needs: "nothing",
  },
];

/** Markdown writes some items differently: its own syntax and wording. */
const MD_SYNTAX: Partial<Record<InsertItem, string>> = {
  image: "![]()",
  csvTable: "| a | b |",
  footnote: "[^1]",
  link: "[](…)",
};

const GROUPS: Group[] = ["files", "refs", "layout"];

const entries = $derived(
  ENTRIES.filter((e) =>
    e.substitute ? !supports(lang, "attach") && supports(lang, "link") : supports(lang, e.item),
  ),
);
const grouped = $derived(
  GROUPS.map((g) => ({ group: g, entries: entries.filter((e) => e.group === g) })).filter(
    (g) => g.entries.length > 0,
  ),
);

/** Markdown words some items differently ("Copie figée…"); the others read the same. */
const MD_WORDING = new Set<string>(["csvTable"]);

function wording(e: Entry, part: "" | ".desc"): string {
  const own = MD_WORDING.has(e.item) && lang === "markdown" ? ".md" : "";
  return t(`insert.${e.item}${own}${part}` as MessageKey);
}

type Step = { kind: "files"; entry: Entry } | { kind: "key"; entry: Entry } | { kind: "nobib" };

let open = $state(false);
let step = $state<Step | undefined>();
let keys = $state<{ key: string; hint: string }[]>([]);
let typed = $state("");
let busy = $state(false);

const doc = $derived(editorState.doc.toString());
const selected = $derived(
  editorState.sliceDoc(editorState.selection.main.from, editorState.selection.main.to),
);
const hasBibliography = $derived(/#bibliography\(/.test(doc));

function reset() {
  step = undefined;
  keys = [];
  typed = "";
}

function go(entry: Entry, close: (refocus?: boolean) => void) {
  if (entry.needs === "nothing") {
    run(insert(lang, entry.item));
    close();
  } else if (entry.needs === "file") {
    step = { kind: "files", entry };
  } else if (insertPrecondition(lang, entry.item, { hasBibliography })) {
    step = { kind: "nobib" };
  } else {
    typed = "";
    step = { kind: "key", entry };
    void loadKeys(entry);
  }
}

async function loadKeys(entry: Entry) {
  if (entry.item === "ref") {
    keys = typstLabels(doc).map((key) => ({ key, hint: "" }));
    return;
  }
  const found: { key: string; hint: string }[] = [];
  for (const f of files.filter((f) => f.kind === FileKind.bib)) {
    const text = await readFile(f.path);
    for (const key of text ? bibKeys(text) : []) found.push({ key, hint: f.path });
  }
  keys = found;
}

async function pick(entry: Entry, file: ProjectFile, close: (refocus?: boolean) => void) {
  busy = true;
  try {
    const args: InsertArgs = { path: file.path, currentFile };
    if (entry.item === "link") {
      args.url = relativePath(currentFile, file.path);
      args.text = selected || file.path.slice(file.path.lastIndexOf("/") + 1);
    } else if (entry.item === "csvTable" && lang === "markdown") {
      const text = await readFile(file.path);
      args.rows = text ? parseCsv(text) : [];
    }
    run(insert(lang, entry.item, args));
    close();
  } finally {
    busy = false;
  }
}

function useKey(entry: Entry, key: string, close: (refocus?: boolean) => void) {
  if (!key.trim()) return;
  run(insert(lang, entry.item, { key: key.trim() }));
  close();
}

const candidates = (entry: Entry) =>
  files.filter((f) => entry.kinds?.includes(f.kind)).sort((a, b) => a.path.localeCompare(b.path));

/** What to say when the project has no file of the kind an item needs. */
function kindHint(entry: Entry): MessageKey {
  switch (entry.item) {
    case "include":
      return "insert.empty.typst";
    case "bibliography":
      return "insert.empty.bib";
    case "csvTable":
      return "insert.empty.csv";
    case "data":
      return "insert.empty.data";
    case "image":
    case "figure":
      return "insert.empty.image";
    default:
      return "insert.empty.any";
  }
}
</script>

<MenuButton bind:open label={t("insert.menu")} width={520} align="start" onclose={reset}>
  {#snippet trigger()}
    <Icon name="plus" />
    <span>{t("insert.button")}</span>
    <Icon name="down" size={12} />
  {/snippet}
  {#snippet children({ close })}
    {#if !step}
      {#each grouped as g, gi (g.group)}
        {#if gi > 0}<div class="sep"></div>{/if}
        <div class="cap">{t(`insert.group.${g.group}`)}</div>
        {#each g.entries as e (e.item)}
          {@const desc = wording(e, ".desc")}
          <button type="button" class="mi two" role="menuitem" data-insert={e.item} onclick={() => go(e, close)}>
            <Icon name={e.icon} />
            <span class="text">
              <span>{wording(e, "")}</span>
              {#if desc}<span class="sub">{desc}</span>{/if}
            </span>
            <span class="k mono">{lang === "markdown" ? (MD_SYNTAX[e.item] ?? e.syntax) : e.syntax}</span>
          </button>
        {/each}
      {/each}
      {#if lang === "markdown"}
        <div class="sep"></div>
        <p class="note sub">{t("insert.markdown.note")}</p>
      {/if}
    {:else}
      <button type="button" class="mi back" role="menuitem" onclick={reset}>
        <Icon name="back" />
        <span>{t("insert.back")}</span>
      </button>
      <div class="sep"></div>
      {#if step.kind === "nobib"}
        <p class="note">{t("insert.nobib")}</p>
        <button
          type="button"
          class="mi"
          role="menuitem"
          onclick={() => {
            const bib = ENTRIES.find((e) => e.item === "bibliography");
            if (bib) step = { kind: "files", entry: bib };
          }}
        >
          <Icon name="book" />
          <span>{t("insert.addBibliography")}</span>
        </button>
      {:else if step.kind === "files"}
        {@const entry = step.entry}
        <div class="cap">{t("insert.choose")}</div>
        {#each candidates(entry) as f (f.path)}
          <button type="button" class="mi" role="menuitem" disabled={busy} onclick={() => pick(entry, f, close)}>
            <FileIcon kind={f.kind} />
            <span class="text">{f.path}</span>
          </button>
        {:else}
          <p class="note sub">{t(kindHint(entry))}</p>
        {/each}
      {:else}
        {@const entry = step.entry}
        <form
          class="keyform"
          onsubmit={(e) => {
            e.preventDefault();
            useKey(entry, typed, close);
          }}
        >
          <label>
            <span class="cap">{t(entry.item === "cite" ? "insert.key.cite" : "insert.key.ref")}</span>
            <input bind:value={typed} spellcheck="false" autocomplete="off" />
          </label>
          <button type="submit" class="btn" disabled={!typed.trim()}>{t("insert.key.insert")}</button>
        </form>
        {#each keys as k (k.key)}
          <button type="button" class="mi" role="menuitem" onclick={() => useKey(entry, k.key, close)}>
            <Icon name={entry.icon} />
            <span class="text mono">{k.key}</span>
            {#if k.hint}<span class="k">{k.hint}</span>{/if}
          </button>
        {/each}
      {/if}
    {/if}
  {/snippet}
</MenuButton>

<style>
  .note {
    margin: 0;
    padding: 6px 10px 8px;
    font-size: 12.5px;
    line-height: 1.45;
  }
  .keyform {
    display: grid;
    gap: 6px;
    padding: 0 10px 8px;
  }
  .keyform label {
    display: grid;
  }
  .keyform .cap {
    padding: 0 0 4px;
  }
  input {
    min-height: var(--target-bar);
    padding: 0 10px;
    border: 0;
    border-radius: var(--radius-button);
    background: var(--btn-bg);
    color: inherit;
    font: inherit;
    font-family: var(--font-mono);
  }
  .mi :global(.ico) {
    color: var(--fg);
    opacity: 0.85;
  }
</style>
