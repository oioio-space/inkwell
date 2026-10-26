import { DELIMITERS, type Delimiter } from "../../editor/csv/records";
import { safeGet, safeSet } from "../../store/safeStorage";

const key = (project: string, path: string) => `inkwell.sheet.delimiter.${project}.${path}`;

/** The separator the person chose for this file, if they did. */
export function rememberedDelimiter(project: string, path: string): Delimiter | undefined {
  const saved = safeGet(key(project, path));
  return DELIMITERS.find((d) => d === saved);
}

/** Keeps the person's choice of separator for this file. */
export function rememberDelimiter(project: string, path: string, delimiter: Delimiter): void {
  safeSet(key(project, path), delimiter);
}
