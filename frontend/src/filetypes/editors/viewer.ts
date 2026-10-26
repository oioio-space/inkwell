import { mount, unmount } from "svelte";
import BlobView from "../../ui/files/BlobView.svelte";
import type { EditorModule } from "../types";

/** The viewer of a file that is not text: what it is, its size, and the way out to the system. */
const viewer: EditorModule = {
  mount(host, ctx) {
    const app = mount(BlobView, {
      target: host,
      props: {
        name: ctx.name,
        size: ctx.size,
        type: ctx.type,
        ...(ctx.openWithSystem ? { openWithSystem: ctx.openWithSystem } : {}),
      },
    });
    return {
      destroy: () => void unmount(app),
      focus: () => host.querySelector<HTMLElement>("button")?.focus(),
      selection: () => undefined,
    };
  },
};

export default viewer;
