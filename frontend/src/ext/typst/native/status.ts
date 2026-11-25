/**
 * Typst's status items (ext/typst, EX0c1): the problems of the document and the page the
 * preview shows.
 */
import Page from "./status/Page.svelte";
import Problems from "./status/Problems.svelte";

export const status = {
  "typst.problems": Problems,
  "typst.page": Page,
};
