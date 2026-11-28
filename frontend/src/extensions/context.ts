/**
 * What a viewer receives (ADR 0027, « Never exposed »). A viewer of any origin gets the facade: the
 * file's name, path and size, why it is read-only, its bytes read on demand, and a way to propose a
 * change. Only a built-in, essential extension that declares `ui.native` gets the native props,
 * which add the editor's own handles (X-3, the typing path). The types are written now so that a
 * built-in cannot widen the facade unnoticed.
 */
import type { DocHandle } from "@automerge/automerge-repo";
import type { Component } from "svelte";
import type { Lang } from "../editor/format";
import { NATIVE } from "./manifest";
import type { Opening, Viewer } from "./registry";

/** The context of a viewer that is not native: data and two verbs, no handle on anything. */
export interface ViewerContext {
  readonly name: string;
  readonly path: string;
  readonly size: number;
  /** Why the file cannot be changed here, if it cannot. */
  readonly readOnlyReason?: string | undefined;
  /** `document.read`: the bytes of a range, read when asked. */
  read(range?: { from: number; to: number }): Promise<Uint8Array>;
  /** `document.propose`: a change the person then accepts or not. */
  propose(change: { from: number; to: number; insert: string }): Promise<boolean>;
}

/** The props of a native viewer of a built-in. */
export interface NativeViewerProps {
  readonly opening: Opening;
  readonly path: string;
  /** The formatting language of the file, for the editor's keys. */
  readonly lang: Lang | undefined;
  readonly me: { id: string; name: string };
  /** The document holding the text, for the main document. */
  readonly handle?: DocHandle<{ text: string }> | undefined;
  /** A text read from the disk, in no document (a projection, or a text too big for one). */
  readonly big?: { text: string } | undefined;
  readonly readOnly?: boolean | undefined;
  readonly readOnlyReason?: string | undefined;
  /** The file could not be read. */
  readonly failed?: boolean | undefined;
  /** The file may be text but is over what a document holds: it was not read. */
  readonly large?: boolean | undefined;
  /** Where the caret goes when the view opens (switching from another view of the same file). */
  readonly startOffset?: number | undefined;
  /** Asks for the text view of the file, the caret at this offset. */
  readonly onshowtext: (offset: number) => void;
  /** An image of the project, as a data URL; undefined when it cannot be read. */
  readonly readImage: (path: string) => Promise<string | undefined>;
}

/** What a viewer entry exports. */
export interface ViewerModule {
  readonly component: Component<NativeViewerProps>;
}

/** What a viewer that is not native is mounted with: the facade alone. */
export interface FacadeProps {
  readonly ctx: ViewerContext;
}

/**
 * The props a viewer is mounted with, decided at this one place from its declared capabilities:
 * a viewer declaring `ui.native` (accepted only from an essential built-in) gets the native props;
 * any other gets only a {@link ViewerContext}, whatever the caller holds.
 */
export function viewerProps(
  viewer: Viewer,
  props: NativeViewerProps,
): NativeViewerProps | FacadeProps {
  if (viewer.capabilities.includes(NATIVE)) return props;
  const bytes = () => new TextEncoder().encode(props.big?.text ?? props.handle?.doc()?.text ?? "");
  const ctx: ViewerContext = Object.freeze({
    name: props.path.slice(props.path.lastIndexOf("/") + 1),
    path: props.path,
    get size() {
      return bytes().length;
    },
    readOnlyReason: props.readOnlyReason,
    read: async (range?: { from: number; to: number }) =>
      range ? bytes().slice(range.from, range.to) : bytes(),
    // Proposals need the review flow of the plugin host (EXT1): none is taken yet.
    propose: async () => false,
  });
  return Object.freeze({ ctx });
}
