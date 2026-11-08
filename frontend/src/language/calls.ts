/** Find the nearest unfinished call, ignoring parentheses inside quoted strings. */
export function calleeOffset(text: string, position: number): number | undefined {
  const prefix = text.slice(0, position);
  const opens: number[] = [];
  let quoted = false,
    escaped = false;
  for (let i = 0; i < prefix.length; i++) {
    const ch = prefix[i];
    if (escaped) {
      escaped = false;
      continue;
    }
    if (quoted && ch === "\\") {
      escaped = true;
      continue;
    }
    if (ch === '"') {
      quoted = !quoted;
      continue;
    }
    if (quoted) continue;
    if (ch === "(") opens.push(i);
    else if (ch === ")") opens.pop();
  }
  const open = opens.at(-1);
  if (open === undefined) return undefined;
  const name = /[\p{L}_][\p{L}\p{N}_.-]*\s*$/u.exec(prefix.slice(0, open));
  return name?.index;
}
