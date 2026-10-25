/**
 * Binary envelope between the webview and Go over the Wails stream `net`
 * (mirror of internal/bridge/envelope.go; both are checked against
 * internal/bridge/testdata/envelopes.json):
 *
 *   version u8 = 1 | kind u8 | len(peer) u8 | peer (ASCII) | payload
 *
 * Webview → Go only sends Message (peer = recipient); Go → webview sends
 * PeerUp, PeerDown and Message (peer = sender).
 */

export const Kind = { PeerUp: 1, PeerDown: 2, Message: 3 } as const;
export type Kind = (typeof Kind)[keyof typeof Kind];

export interface Envelope {
  kind: Kind;
  peer: string;
  payload: Uint8Array;
}

export class MalformedEnvelope extends Error {
  override name = "MalformedEnvelope";
}

const VERSION = 1;
const HEADER = 3;
const MAX_PEER = 64;

function validPeer(peer: string): boolean {
  if (peer.length === 0 || peer.length > MAX_PEER) return false;
  for (let i = 0; i < peer.length; i++) {
    const c = peer.charCodeAt(i);
    if (c <= 0x20 || c > 0x7e) return false;
  }
  return true;
}

function check(kind: number, peer: string, payloadLength: number): asserts kind is Kind {
  if (kind !== Kind.PeerUp && kind !== Kind.PeerDown && kind !== Kind.Message) {
    throw new MalformedEnvelope(`unknown kind ${kind}`);
  }
  if (!validPeer(peer)) throw new MalformedEnvelope("invalid peer id");
  if (kind !== Kind.Message && payloadLength > 0) {
    throw new MalformedEnvelope("peer events carry no payload");
  }
}

export function encodeEnvelope(e: Envelope): Uint8Array {
  check(e.kind, e.peer, e.payload.length);
  const out = new Uint8Array(HEADER + e.peer.length + e.payload.length);
  out[0] = VERSION;
  out[1] = e.kind;
  out[2] = e.peer.length;
  for (let i = 0; i < e.peer.length; i++) out[HEADER + i] = e.peer.charCodeAt(i);
  out.set(e.payload, HEADER + e.peer.length);
  return out;
}

/** Decodes a frame; the returned payload is a view on `b`, not a copy. */
export function decodeEnvelope(b: Uint8Array): Envelope {
  if (b.length < HEADER) throw new MalformedEnvelope("short frame");
  if (b[0] !== VERSION) throw new MalformedEnvelope(`unsupported version ${b[0]}`);
  const kind = b[1] as number;
  const peerLength = b[2] as number;
  if (b.length < HEADER + peerLength) throw new MalformedEnvelope("truncated peer");
  const peer = String.fromCharCode(...b.subarray(HEADER, HEADER + peerLength));
  const payload = b.subarray(HEADER + peerLength);
  check(kind, peer, payload.length);
  return { kind, peer, payload };
}
