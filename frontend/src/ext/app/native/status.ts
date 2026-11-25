/**
 * The application's status items (ext/app, EX0c1): who is here, the unsaved text, the caret,
 * the words, the text zoom, the encoding and the language. The bar draws them by their groups.
 */
import Encoding from "./status/Encoding.svelte";
import Language from "./status/Language.svelte";
import Position from "./status/Position.svelte";
import Presence from "./status/Presence.svelte";
import Unsaved from "./status/Unsaved.svelte";
import Words from "./status/Words.svelte";
import Zoom from "./status/Zoom.svelte";

export const status = {
  "app.presence": Presence,
  "app.unsaved": Unsaved,
  "app.position": Position,
  "app.words": Words,
  "app.zoom": Zoom,
  "app.encoding": Encoding,
  "app.language": Language,
};
