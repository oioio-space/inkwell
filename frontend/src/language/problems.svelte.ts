import { EditorView } from "@codemirror/view";
import { tick } from "svelte";
import { flushTyped } from "../editor/amSync";
import type { Diagnostic } from "../preview/protocol";
import { settings } from "../store/settings.svelte";
import { session } from "../ui/editorSession.svelte";
import { history } from "../ui/history/history.svelte";
import { sheetView } from "../ui/sheet/viewMode.svelte";
import { workspace } from "../workspace/workspace.svelte";
import type { LanguageDiagnostic } from "./client";
import {
  type AnchoredRange,
  anchorRange,
  type Position,
  resolveRange,
  toOffset,
} from "./positions";
export interface Problem {
  source: string;
  file: string;
  severity: "error" | "warning";
  message: string;
  version: number;
  from: number;
  to: number;
  anchor?: AnchoredRange;
  position?: { start: Position; end: Position };
  stale: boolean;
  verifiedHash?: string;
}
export class Problems {
  sources = $state.raw<readonly Problem[]>([]);
  entries = $derived(
    this.sources.filter(
      (d) =>
        d.source !== "compiler" ||
        !this.sources.some(
          (other) => other.source === "language" && other.file === d.file && !other.stale,
        ),
    ),
  );
  index = -1;
  #compilerTicket = 0;
  #text: string | undefined;
  errors = $derived(this.entries.filter((d) => d.severity === "error").length);
  warnings = $derived(this.entries.filter((d) => d.severity === "warning").length);
  publish(file: string, version: number, items: LanguageDiagnostic[]) {
    const doc = file === "main.typ" ? workspace.handle?.doc() : undefined;
    const text =
      doc?.text ?? (workspace.current === file ? workspace.opened?.text : undefined) ?? "";
    const next = items.map((d) => {
      const from = toOffset(text, d.range.start),
        to = toOffset(text, d.range.end);
      return {
        source: "language",
        file,
        version,
        from,
        to,
        position: d.range,
        message: d.message,
        severity: d.severity === 2 ? ("warning" as const) : ("error" as const),
        stale: false,
        ...(doc ? { anchor: anchorRange(doc, from, to) } : {}),
      };
    });
    this.sources = [
      ...this.sources.filter((d) => d.file !== file || d.source !== "language"),
      ...next,
    ].sort(order);
  }
  compiler(items: readonly Diagnostic[], version: number, stale = true) {
    const next: Problem[] = items.map((d) => {
      const file = d.file ?? "main.typ";
      const doc = file === "main.typ" ? workspace.handle?.doc() : undefined;
      const from = toOffset(doc?.text ?? "", {
        line: (d.line ?? 1) - 1,
        character: (d.column ?? 1) - 1,
      });
      return {
        source: "compiler",
        file,
        severity: d.severity,
        message: d.message,
        version,
        from,
        to: from + 1,
        position: {
          start: { line: (d.line ?? 1) - 1, character: (d.column ?? 1) - 1 },
          end: { line: (d.line ?? 1) - 1, character: d.column ?? 1 },
        },
        stale,
        ...(doc && !stale
          ? { anchor: anchorRange(doc, from, Math.min(from + 1, doc.text.length)) }
          : {}),
      };
    });
    this.sources = [...this.sources.filter((d) => d.source !== "compiler"), ...next].sort(order);
  }
  async compilerVerified(
    items: readonly Diagnostic[],
    version: number,
    hashes: Record<string, string>,
  ) {
    const ticket = ++this.#compilerTicket;
    this.compiler(items, version, true);
    const doc = workspace.handle?.doc();
    const texts = new Map<string, string>();
    for (const file of new Set(items.map((d) => d.file ?? "main.typ"))) {
      const text = file === "main.typ" ? doc?.text : await workspace.readFile(file);
      if (text === undefined || !hashes[file]) continue;
      const bytes = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(text));
      const hash = Array.from(new Uint8Array(bytes), (b) => b.toString(16).padStart(2, "0")).join(
        "",
      );
      if (hash === hashes[file]) texts.set(file, text);
    }
    if (ticket !== this.#compilerTicket || workspace.handle?.doc() !== doc) return;
    this.sources = this.sources.map((d) => {
      const text = texts.get(d.file);
      if (d.source !== "compiler" || text === undefined || !d.position) return d;
      const line = d.position.start.line;
      const lineStart = toOffset(text, { line, character: 0 });
      const lineText = text.slice(lineStart).split("\n")[0] ?? "";
      // Typst CLI columns count Unicode scalar values, unlike LSP's UTF-16.
      const character = Array.from(lineText).slice(0, d.position.start.character).join("").length;
      const from = lineStart + character;
      const name = /^unknown variable: (.+)$/.exec(d.message)?.[1];
      const length =
        name && text.slice(from, from + name.length) === name
          ? name.length
          : Math.min(1, text.length - from);
      const to = from + length;
      return {
        ...d,
        from,
        to,
        position: { start: { line, character }, end: { line, character: character + length } },
        stale: false,
        ...(hashes[d.file] ? { verifiedHash: hashes[d.file] } : {}),
        ...(d.file === "main.typ" && doc ? { anchor: anchorRange(doc, from, to) } : {}),
      };
    });
  }
  csv(file: string, text: string, items: readonly { line: number; message: string }[]) {
    const next: Problem[] = items.map((d) => ({
      source: "csv",
      file,
      severity: "warning",
      message: d.message,
      version: 0,
      from: toOffset(text, { line: d.line - 1, character: 0 }),
      to: toOffset(text, { line: d.line, character: 0 }),
      position: { start: { line: d.line - 1, character: 0 }, end: { line: d.line, character: 0 } },
      stale: false,
    }));
    this.sources = [
      ...this.sources.filter((d) => d.source !== "csv" || d.file !== file),
      ...next,
    ].sort(order);
  }
  reset() {
    this.#text = undefined;
    this.#compilerTicket++;
    this.sources = [];
    this.index = -1;
  }
  /** The document changed; the same text again (the echo of a save) leaves verified problems as they are. */
  edit(text: string) {
    if (text === this.#text) return;
    this.#text = text;
    this.#compilerTicket++;
    this.sources = this.sources.map((d) => (d.source === "compiler" ? { ...d, stale: true } : d));
  }
  show() {
    history.open = true;
    history.tab = "problems";
  }
  next(reverse = false) {
    const active = this.entries.filter((d) => !d.stale);
    if (!active.length) return;
    this.index =
      this.index < 0
        ? reverse
          ? active.length - 1
          : 0
        : (this.index + (reverse ? -1 : 1) + active.length) % active.length;
    const problem = active[this.index];
    if (problem) void this.jump(problem);
  }
  async jump(d: Problem) {
    if (d.stale) return;
    const ticket = this.#compilerTicket;
    if (d.source === "csv") sheetView.toText(d.from);
    if (!workspace.files.some((file) => file.path === d.file)) await workspace.refreshFiles();
    await workspace.openFile(d.file);
    await tick();
    const view = session.view;
    if (!view) return;
    if (d.source === "compiler" && d.verifiedHash) {
      const text = view.state.doc.toString();
      const bytes = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(text));
      const hash = Array.from(new Uint8Array(bytes), (b) => b.toString(16).padStart(2, "0")).join(
        "",
      );
      if (
        ticket !== this.#compilerTicket ||
        view !== session.view ||
        view.state.doc.toString() !== text
      )
        return;
      if (hash !== d.verifiedHash) {
        this.sources = this.sources.map((current) =>
          current === d ? { ...current, stale: true } : current,
        );
        return;
      }
    }

    // The anchor is a place in the document: what was typed since the paint goes in first.
    if (d.file === "main.typ" && workspace.handle) flushTyped(workspace.handle);
    const doc = d.file === "main.typ" ? workspace.handle?.doc() : undefined;
    const text = view.state.doc.toString();
    const range =
      doc && d.anchor
        ? resolveRange(doc, d.anchor)
        : d.position
          ? { from: toOffset(text, d.position.start), to: toOffset(text, d.position.end) }
          : { from: d.from, to: d.to };
    const from = Math.min(view.state.doc.length, range.from),
      to = Math.min(view.state.doc.length, range.to);
    revealEditor();
    view.dispatch({
      selection: { anchor: to, head: from },
      effects: EditorView.scrollIntoView(from, { y: "center" }),
    });
    view.focus();
  }
}
/** In a narrow window the preview covers the source: the caret would jump out of sight. */
function revealEditor() {
  const preview = document.querySelector(".side-preview");
  if (preview && getComputedStyle(preview).position === "absolute")
    settings.setPreviewVisible(workspace.project, false);
}
function order(a: Problem, b: Problem) {
  return a.file.localeCompare(b.file) || a.from - b.from;
}
export const problems = new Problems();
