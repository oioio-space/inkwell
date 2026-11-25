/**
 * The application's commands (ext/app): each one asks the shell to do what the header bar, the
 * main menu or the status bar did by hand before EX0b1. Commands acting on a view (undo, find,
 * the next difference) have no entry: the focused surface handles them.
 */
import type { CommandModule } from "../../../extensions/commands";

export const commands: CommandModule["commands"] = {
  "app.save": ({ shell }) => shell.save(),
  "app.toolbar": ({ shell }) => shell.toggleToolbar(),
  "app.preview": ({ shell }) => shell.togglePreview(),
  "app.sidebar": ({ shell }) => shell.toggleSidebar(),
  "app.review": ({ shell }) => shell.toggleReview(),
  "app.fullscreen": ({ shell }) => shell.fullscreen(),
  "app.share": ({ shell }) => shell.share(),
  "app.people": ({ shell }) => shell.people(),
  "app.return": ({ shell }) => shell.returnToPrevious(),
  "app.preferences": ({ shell }) => shell.preferences(),
  "app.about": ({ shell }) => shell.about(),
  "app.problem-next": ({ shell }) => shell.problem(false),
  "app.problem-previous": ({ shell }) => shell.problem(true),
  "app.zoom-in": ({ shell }) => shell.zoom("in"),
  "app.zoom-out": ({ shell }) => shell.zoom("out"),
  "app.zoom-reset": ({ shell }) => shell.zoom("reset"),
  "app.palette": ({ shell }) => shell.palette(),
  "app.shortcuts": ({ shell }) => shell.shortcuts(),
};
