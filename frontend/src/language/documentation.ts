export function documentation(value: unknown): string {
  if (typeof value === "string") return value;
  if (Array.isArray(value)) return value.map(documentation).join("\n");
  const obj =
    value !== null && typeof value === "object" ? (value as Record<string, unknown>) : undefined;
  return typeof obj?.value === "string" ? obj.value : "";
}

/**
 * The service answers in Markdown, but the card shows plain text (nothing shared is rendered as
 * markup): the fences, rules and link syntax go, the words and the code stay.
 */
export function readable(text: string): string {
  return text
    .split("\n")
    .filter((line) => !/^\s*(```.*|---+|\*\*\*+)\s*$/.test(line))
    .join("\n")
    .replace(/\[([^\]]+)\]\((?:https?:\/\/|\/)[^)\s]*\)/g, "$1")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}
