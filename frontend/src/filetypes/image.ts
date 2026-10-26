import { register } from "./registry";

const starts = (b: Uint8Array, ...sig: number[]) => sig.every((v, i) => b[i] === v);

register({
  id: "image",
  label: "kind.image",
  icon: "image",
  extensions: [
    ".png",
    ".jpg",
    ".jpeg",
    ".gif",
    ".webp",
    ".bmp",
    ".svg",
    ".ico",
    ".avif",
    ".tif",
    ".tiff",
  ],
  mime: ["image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp", "image/svg+xml"],
  binary: true,
  sniff(b) {
    if (
      starts(b, 0x89, 0x50, 0x4e, 0x47) ||
      starts(b, 0xff, 0xd8, 0xff) ||
      starts(b, 0x47, 0x49, 0x46, 0x38)
    ) {
      return 0.95;
    }
    // RIFF....WEBP
    return starts(b, 0x52, 0x49, 0x46, 0x46) && b[8] === 0x57 && b[9] === 0x45 ? 0.9 : 0;
  },
  open: () => import("./editors/viewer"),
});
