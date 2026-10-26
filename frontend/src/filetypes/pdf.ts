import { register } from "./registry";

register({
  id: "pdf",
  label: "kind.pdf",
  icon: "pdf",
  extensions: [".pdf"],
  mime: ["application/pdf"],
  binary: true,
  sniff: (b) => (b[0] === 0x25 && b[1] === 0x50 && b[2] === 0x44 && b[3] === 0x46 ? 0.95 : 0),
  open: () => import("./editors/viewer"),
});
