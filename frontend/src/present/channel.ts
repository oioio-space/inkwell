import type { DocHandle } from "@automerge/automerge-repo";
import { MARKER, parseWire, type WireMessage } from "./wire";

/** Where a presentation talks to the other people of the project. */
export interface Channel {
  /** Says `m` to everyone connected; nothing is stored, a person who is not there misses it. */
  broadcast(m: WireMessage): void;
  /** Calls `cb` with the peer the transport vouches for and what it said; returns how to stop. */
  subscribe(cb: (from: string, m: WireMessage) => void): () => void;
}

/**
 * The channel over the ephemeral messages of a document handle (the same wire as the cursors,
 * under another key: the presence code ignores it and this one ignores the presence). What does
 * not read as a presentation message is dropped here.
 */
export function handleChannel(handle: DocHandle<unknown>): Channel {
  return {
    broadcast: (m) => handle.broadcast({ [MARKER]: m }),
    subscribe(cb) {
      const on = ({ senderId, message }: { senderId: string; message: unknown }) => {
        if (typeof message !== "object" || message === null || !(MARKER in message)) return;
        const m = parseWire((message as Record<string, unknown>)[MARKER]);
        if (m) cb(senderId, m);
      };
      handle.on("ephemeral-message", on);
      return () => handle.off("ephemeral-message", on);
    },
  };
}
