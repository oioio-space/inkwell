/** The markup language of a file, which decides what the formatting bar can do. */
export type Lang = "typst" | "markdown" | "plain";

/** The language of a file, from its extension (case-insensitive); anything else is plain text. */
export function langOf(path: string): Lang {
  const name = path.slice(Math.max(path.lastIndexOf("/"), path.lastIndexOf("\\")) + 1);
  const dot = name.lastIndexOf(".");
  if (dot <= 0) return "plain";
  switch (name.slice(dot + 1).toLowerCase()) {
    case "typ":
    case "typst":
      return "typst";
    case "md":
    case "markdown":
      return "markdown";
    default:
      return "plain";
  }
}
