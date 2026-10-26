import { BINARY, register } from "./registry";

const open = () => import("./editors/viewer");

register({
  id: "font",
  label: "kind.font",
  icon: "font",
  extensions: [".ttf", ".otf", ".woff", ".woff2", ".ttc"],
  mime: ["font/ttf", "font/otf", "font/woff", "font/woff2"],
  binary: true,
  sniff(b) {
    const four = String.fromCharCode(b[0] ?? 0, b[1] ?? 0, b[2] ?? 0, b[3] ?? 0);
    const sfnt = b[0] === 0 && b[1] === 1 && b[2] === 0 && b[3] === 0;
    return four === "OTTO" || four === "wOFF" || four === "wOF2" || sfnt ? 0.85 : 0;
  },
  open,
});

register({
  id: "archive",
  label: "kind.archive",
  icon: "archive",
  extensions: [".zip", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".7z", ".rar", ".jar"],
  mime: ["application/zip", "application/gzip", "application/x-tar"],
  binary: true,
  sniff: (b) =>
    (b[0] === 0x50 && b[1] === 0x4b && b[2] === 3) || (b[0] === 0x1f && b[1] === 0x8b) ? 0.9 : 0,
  open,
});

// What nothing else claims and is not text: shown, never garbled.
register({
  id: BINARY,
  label: "kind.binary",
  icon: "binary",
  extensions: [".bin", ".exe", ".dll", ".so", ".dylib", ".dat", ".db", ".sqlite", ".wasm", ".o"],
  mime: ["application/octet-stream"],
  binary: true,
  open,
});
