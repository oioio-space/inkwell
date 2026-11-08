import type { SocketLike } from "../net/WailsNetworkAdapter";
import { afterPaint } from "../ui/afterPaint";
import { incrementalChange, type Position } from "./positions";

type Document = { text: string; version: number; revision?: string };
type Pending = {
  resolve: (value: unknown) => void;
  file: string;
  version: number;
  generation: number;
  method: string;
  workspaceRevision: number;
};
export type LanguageDiagnostic = {
  message: string;
  severity?: number;
  range: { start: Position; end: Position };
};
const uriOf = (file: string) => `file:///${file.split("/").map(encodeURIComponent).join("/")}`;
/**
 * How long to wait before reconnecting after `failures` connections in a row closed before the
 * service answered `initialize`: exponential from 1 s, capped at 30 s, with jitter in the upper
 * half (never a hot loop, and windows that lost the service together do not retry in step).
 */
export function reconnectDelay(failures: number, random: () => number = Math.random): number {
  const ceiling = Math.min(30_000, 1000 * 2 ** Math.min(failures, 15));
  return ceiling * (0.5 + random() / 2);
}

/** The request's document version and server generation are frozen until its answer. */
export class LanguageClient {
  #socket: SocketLike | undefined;
  #generation = 0;
  #workspaceRevision = 0;
  #id = 0;
  #ready = false;
  #capabilities: Record<string, unknown> = {};
  #syncKind = 0;
  #openClose = false;
  #stopped = true;
  #reconnect: ReturnType<typeof setTimeout> | undefined;
  #failures = 0;
  #documents = new Map<string, Document>();
  #pending = new Map<number, Pending>();
  constructor(
    readonly connect: () => SocketLike,
    readonly diagnostics: (file: string, version: number, items: LanguageDiagnostic[]) => void,
  ) {}
  start() {
    this.#stopped = false;
    const socket = this.connect();
    this.#socket = socket;
    const generation = ++this.#generation;
    socket.binaryType = "arraybuffer";
    let initializeId = 0;
    socket.onopen = () => {
      if (this.#socket !== socket || generation !== this.#generation) return;
      initializeId = ++this.#id;
      this.#send({
        id: initializeId,
        method: "initialize",
        params: {
          capabilities: {
            general: { positionEncodings: ["utf-16"] },
            textDocument: { publishDiagnostics: { versionSupport: true } },
          },
          initializationOptions: {},
        },
      });
    };
    const receive = (e: MessageEvent) => {
      if (this.#socket !== socket || generation !== this.#generation) return;
      try {
        const m = JSON.parse(
          typeof e.data === "string" ? e.data : new TextDecoder().decode(e.data),
        );
        if (!this.#ready && m.id === initializeId && m.result?.capabilities) {
          const capabilities = m.result.capabilities;
          if (
            typeof capabilities !== "object" ||
            Array.isArray(capabilities) ||
            (capabilities.positionEncoding !== undefined &&
              capabilities.positionEncoding !== "utf-16")
          ) {
            socket.close();
            return;
          }
          this.#capabilities = capabilities;
          const sync = capabilities.textDocumentSync;
          const kind = typeof sync === "number" ? sync : sync?.change;
          this.#syncKind = kind === 1 || kind === 2 ? kind : 0;
          this.#openClose = typeof sync === "number" ? sync !== 0 : sync?.openClose === true;
          this.#ready = true;
          this.#failures = 0;
          this.#send({ method: "initialized", params: {} });
          for (const [file, doc] of this.#documents) this.#didOpen(file, doc);
          return;
        }
        if (m.method === "inkwell/unavailable") {
          this.#clear();
          socket.close();
          return;
        }
        const pending = this.#pending.get(m.id);
        if (pending) {
          this.#pending.delete(m.id);
          pending.resolve(
            !m.error &&
              pending.generation === this.#generation &&
              pending.workspaceRevision === this.#workspaceRevision &&
              pending.version === this.#documents.get(pending.file)?.version
              ? m.result
              : undefined,
          );
        } else if (m.method === "textDocument/publishDiagnostics") {
          const p = m.params;
          if (typeof p?.uri !== "string" || !p.uri.startsWith("file:///")) return;
          const file = decodeURIComponent(p.uri.slice(8));
          const doc = this.#documents.get(file);
          // Versionless diagnostics cannot prove which source revision was checked.
          if (!doc || p.version !== doc.version) return;
          if (Array.isArray(p.diagnostics))
            this.diagnostics(
              file,
              p.version ?? 0,
              p.diagnostics.slice(0, 500).filter(validDiagnostic),
            );
        }
      } catch {
        /* Malformed or disconnected peer data never reaches the UI. */
      }
    };
    // After the next paint, in order: responses never straddle a refresh while someone types.
    socket.onmessage = (e) => afterPaint(() => receive(e));
    socket.onclose = () => {
      if (this.#socket !== socket) return;
      this.#clear();
      this.#socket = undefined;
      if (!this.#stopped)
        this.#reconnect = setTimeout(() => this.start(), reconnectDelay(this.#failures++));
    };
  }
  reset() {
    this.stop();
    this.#documents.clear();
    this.#workspaceRevision++;
    this.start();
  }
  stop() {
    this.#stopped = true;
    clearTimeout(this.#reconnect);
    this.#generation++;
    this.#clear();
    const socket = this.#socket;
    this.#socket = undefined;
    socket?.close();
  }
  #cancelPending(matches: (pending: Pending) => boolean = () => true) {
    for (const [id, pending] of this.#pending) {
      if (!matches(pending)) continue;
      this.#send({ method: "$/cancelRequest", params: { id } });
      this.#pending.delete(id);
      pending.resolve(undefined);
    }
  }
  #clear() {
    this.#ready = false;
    this.#capabilities = {};
    this.#syncKind = 0;
    this.#openClose = false;
    this.#cancelPending();
  }
  #send(message: unknown) {
    try {
      this.#socket?.send(
        new TextEncoder().encode(JSON.stringify({ jsonrpc: "2.0", ...(message as object) })),
      );
    } catch {
      /* Closing streams drop the request; the generation rejects any late reply. */
    }
  }

  #didOpen(file: string, doc: Document) {
    if (!this.#openClose) return;
    this.syntaxOnly([...this.#documents.values()].some((d) => d.text.length > 2 * 1024 * 1024));
    this.#send({
      method: "textDocument/didOpen",
      params: {
        textDocument: {
          uri: uriOf(file),
          languageId: "typst",
          version: doc.version,
          text: doc.text,
        },
      },
    });
  }
  open(file: string, text: string, revision?: string) {
    if (this.#documents.has(file)) {
      this.change(file, text, revision);
      return;
    }
    this.#workspaceRevision++;
    this.#cancelPending();
    const doc: Document = { text, version: 1, ...(revision !== undefined ? { revision } : {}) };
    this.#documents.set(file, doc);
    if (this.#ready) this.#didOpen(file, doc);
  }
  get ready(): boolean {
    return this.#ready;
  }
  version(file: string) {
    return this.#documents.get(file)?.version ?? 0;
  }
  change(file: string, text: string, revision?: string) {
    const doc = this.#documents.get(file);
    if (!doc) {
      this.open(file, text, revision);
      return;
    }
    if (doc.text === text && (revision === undefined || doc.revision === revision)) return;
    this.#workspaceRevision++;
    this.#cancelPending();
    if (revision !== undefined) doc.revision = revision;
    const old = doc.text;
    doc.text = text;
    doc.version++;
    if (this.#ready) {
      if (old.length > 2 * 1024 * 1024 !== text.length > 2 * 1024 * 1024)
        this.syntaxOnly([...this.#documents.values()].some((d) => d.text.length > 2 * 1024 * 1024));
      if (this.#syncKind !== 0)
        this.#send({
          method: "textDocument/didChange",
          params: {
            textDocument: { uri: uriOf(file), version: doc.version },
            contentChanges: [this.#syncKind === 2 ? incrementalChange(old, text) : { text }],
          },
        });
    }
  }
  syntaxOnly(on: boolean) {
    if (this.#ready)
      this.#send({
        method: "workspace/didChangeConfiguration",
        params: { settings: { syntaxOnly: on ? "enable" : "auto" } },
      });
  }
  request(method: string, file: string, position: Position): Promise<unknown> {
    const doc = this.#documents.get(file);
    const provider = {
      "textDocument/completion": "completionProvider",
      "textDocument/signatureHelp": "signatureHelpProvider",
      "textDocument/hover": "hoverProvider",
    }[method];
    if (!this.#ready || !doc || !provider || !this.#capabilities[provider])
      return Promise.resolve(undefined);
    this.#cancelPending((pending) => pending.method === method && pending.file === file);
    const id = ++this.#id;
    return new Promise((resolve) => {
      this.#pending.set(id, {
        resolve,
        file,
        version: doc.version,
        generation: this.#generation,
        method,
        workspaceRevision: this.#workspaceRevision,
      });
      this.#send({ id, method, params: { textDocument: { uri: uriOf(file) }, position } });
    });
  }
}
function validDiagnostic(d: unknown): d is LanguageDiagnostic {
  if (!d || typeof d !== "object") return false;
  const p = d as LanguageDiagnostic;
  return (
    typeof p.message === "string" &&
    p.message.length <= 16384 &&
    [p.range?.start, p.range?.end].every(
      (p) =>
        p &&
        Number.isInteger(p.line) &&
        p.line >= 0 &&
        Number.isInteger(p.character) &&
        p.character >= 0,
    )
  );
}
