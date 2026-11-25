/**
 * The application's drawer, « Relire » (ext/app, EX0c2): the problems, the history, the comments
 * and the activity. One chunk, fetched at idle after the first paint (ui/command/panels.svelte.ts).
 */
import Problems from "../../../language/Problems.svelte";
import Activity from "../../../ui/history/ActivityPanel.svelte";
import Comments from "../../../ui/history/CommentsPanel.svelte";
import History from "../../../ui/history/HistoryPanel.svelte";

export const panels = {
  "app.problems": Problems,
  "app.history": History,
  "app.comments": Comments,
  "app.activity": Activity,
};
