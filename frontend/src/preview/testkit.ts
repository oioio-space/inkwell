import type { SocketLike } from "../net/WailsNetworkAdapter";
import type { Diagnostic, ResultMessage } from "./protocol";

/** A page of Typst's output in miniature: an A4 SVG whose root carries the size in points. */
export const svgPage = (width = 595.28, height = 841.89): string =>
  `<svg viewBox="0 0 ${width} ${height}" width="${width}pt" height="${height}pt" xmlns="http://www.w3.org/2000/svg"><path fill="#fff" d="M 0 0v ${height}h ${width}v -${height}Z"/></svg>`;

/** A result message of Go, with sensible defaults. */
export function result(over: Partial<ResultMessage> = {}): ResultMessage {
  return {
    type: "result",
    seq: 1,
    rev: 1,
    pageCount: 3,
    first: 1,
    pages: [svgPage(), svgPage(), svgPage()],
    diagnostics: [],
    truncated: false,
    ...over,
  };
}

export const problem = (over: Partial<Diagnostic> = {}): Diagnostic => ({
  severity: "error",
  message: "unknown variable: foo",
  file: "main.typ",
  line: 3,
  column: 2,
  ...over,
});

/** The webview's end of a stream, played by the test: `sent` holds the JSON the client wrote. */
export class FakeSocket implements SocketLike {
  binaryType = "blob";
  onopen: ((ev: Event) => unknown) | null = null;
  onclose: ((ev: Event) => unknown) | null = null;
  onmessage: ((ev: MessageEvent) => unknown) | null = null;
  sent: Record<string, unknown>[] = [];
  closed = false;

  send(data: Uint8Array): void {
    this.sent.push(JSON.parse(new TextDecoder().decode(data)));
  }

  close(): void {
    this.closed = true;
  }

  /** Go accepted the stream. */
  open(): void {
    this.onopen?.(new Event("open"));
  }

  /** Go dropped the stream. */
  drop(): void {
    this.onclose?.(new Event("close"));
  }

  /** Go sends a message. */
  receive(message: unknown): void {
    const bytes = new TextEncoder().encode(JSON.stringify(message));
    this.onmessage?.({ data: bytes.buffer } as MessageEvent);
  }
}
