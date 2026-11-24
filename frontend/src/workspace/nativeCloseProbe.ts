import { Stream } from "@wailsio/runtime";
import { tick } from "svelte";
import { t } from "../i18n";
import type { SocketLike } from "../net/WailsNetworkAdapter";
import { session } from "../ui/editorSession.svelte";

import { viewChoice } from "./viewChoice.svelte";
import { workspace } from "./workspace.svelte";

interface ProbeCommand {
  command: string;
  value?: string;
}

const decoder = new TextDecoder();
const encoder = new TextEncoder();

// Only an explicitly instrumented test page connects to the native test supervisor.
export function nativeCloseProbe(mode: string, ready: Promise<unknown>) {
  const socket = Stream("wc1-proof") as unknown as SocketLike;
  socket.binaryType = "arraybuffer";
  const send = (event: string, value: unknown = null) =>
    socket.send(encoder.encode(JSON.stringify({ event, value })));
  const editing = () => session.view?.state.doc.toString();
  const waitFor = async (predicate: () => boolean, bound = 10_000) => {
    const end = performance.now() + bound;
    while (!predicate()) {
      if (performance.now() >= end) throw new Error("native page condition did not arrive");
      await new Promise<void>((resolve) => setTimeout(resolve, 10));
    }
  };
  const inspect = () => ({
    text: editing() ?? workspace.active?.doc()?.text ?? "",
    unsaved: document.querySelector("[data-testid=unsaved]")?.textContent?.trim() ?? "",
    remote: document.querySelectorAll(".cell.rem").length,
    peers: workspace.peers.length,
  });
  let release: (() => void) | undefined;
  socket.onopen = () => {
    void ready
      .then(async () => {
        await waitFor(() => editing() !== undefined);
        send("page-ready", inspect());
      })
      .catch((error) => send("page-error", String(error)));
  };
  socket.onmessage = (event) => {
    const { command, value } = JSON.parse(
      decoder.decode(event.data as ArrayBuffer),
    ) as ProbeCommand;
    void (async () => {
      switch (command) {
        case "state":
          send(command, inspect());
          break;
        case "type": {
          const view = session.view;
          if (!view) throw new Error("editor is missing");
          const baseline = view.state.doc.toString();
          view.dispatch({ selection: { anchor: view.state.doc.length } });
          view.focus();
          send("type-ready", baseline);
          await waitFor(() => editing() !== baseline);
          send("input-accepted", editing());
          break;
        }
        case "durable":
          try {
            await workspace.saveForClose(AbortSignal.timeout(5000));
          } finally {
            workspace.resumeAfterClose();
          }
          send(command, inspect());
          break;
        case "hang":
          send("hang-armed");
          setTimeout(() => {
            // The fixture holds the WebKit renderer, while GTK and Go can still respond.
            for (;;) Math.sqrt(Date.now());
          }, 0);
          break;
        case "write": {
          const handle = workspace.active;
          if (!handle || value === undefined) throw new Error("document is missing");
          handle.change((doc) => {
            doc.text = value;
          });
          send(command, inspect());
          break;
        }
        case "invite":
          await workspace.host();
          if (!workspace.invitation) throw new Error(workspace.error || "no invitation");
          send(command, workspace.invitation);
          break;
        case "join":
          await workspace.join(value ?? "");
          if (workspace.error) throw new Error(workspace.error);
          // Into another document Go holds the Welcome and the page asks before Inkwell restarts
          // (PC1b, rules J-1 and J-2).
          if (workspace.joinOffer) send("join-held", workspace.joinOffer);
          else send(command, inspect());
          break;
        case "confirm-join": {
          // « Redémarrer et ouvrir », as the person presses it: the page leaves through the
          // desktop close approval and this process restarts in place (PC1b, rule J-2).
          await tick();
          const yes = [
            ...document.querySelectorAll<HTMLButtonElement>("[data-testid=join-confirm] button"),
          ].find((b) => b.textContent?.trim() === t("join.confirm.yes"));
          if (!yes) throw new Error("the join confirmation is missing");
          yes.click();
          send(command);
          break;
        }
        case "sheet":
          viewChoice.openAsTable(workspace.project, "main.typ");
          await tick();
          await waitFor(() => document.querySelector("#cell-2-2") !== null);
          send(command, inspect());
          break;
        case "select": {
          const first = document.querySelector("#cell-1-1");
          const last = document.querySelector("#cell-2-2");
          if (!first || !last) throw new Error("sheet cells are missing");
          first.dispatchEvent(new PointerEvent("pointerdown", { bubbles: true, button: 0 }));
          last.dispatchEvent(
            new PointerEvent("pointerdown", { bubbles: true, button: 0, shiftKey: true }),
          );
          send(command, inspect());
          break;
        }
        case "release":
          release?.();
          break;
        default:
          throw new Error(`unrecognized native fixture command ${command}`);
      }
    })().catch((error) => send("page-error", String(error)));
  };
  return {
    waitForSelectionClear(signal: AbortSignal): Promise<void> {
      if (mode !== "selection-hold") return Promise.resolve();
      return new Promise<void>((resolve, reject) => {
        release = resolve;
        signal.addEventListener("abort", () => reject(signal.reason), { once: true });
      });
    },
    stop: () => socket.close(),
  };
}
