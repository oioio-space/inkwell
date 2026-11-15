import {
  StartupID,
  TakeOver,
} from "../../bindings/github.com/oioio-space/inkwell/internal/app/session";
import "../theme/tokens.css";
import "./restart.css";
import { takeLeaveMark } from "./mark";
import { startRestart } from "./restart";

// The entry of restart.html: no repo, no adapter, no editor, no store (rule N-15).
const target = document.getElementById("restart");
if (!target) throw new Error("missing #restart mount point");
void startRestart(target, {
  confirmed: takeLeaveMark,
  takeOver: () => TakeOver(),
  startupId: () => StartupID(),
  load: (url) => location.replace(url),
  wait: (ms) => new Promise((resolve) => setTimeout(resolve, ms)),
});
