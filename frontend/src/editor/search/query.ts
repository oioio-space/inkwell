import { foldAccents } from "./textmap";

/**
 * How the query text is read:
 * - normal: literal text;
 * - extended: literal text with the escapes `\n \t \r \0 \\ \xHH \uHHHH`;
 * - wildcard: `*` = any run within a line, `?` = one character, `[abc]` = one of; all else literal;
 * - regex: a JavaScript regular expression (flag `u`).
 */
export type SearchMode = "normal" | "extended" | "wildcard" | "regex";

export interface QuerySpec {
  text: string;
  mode: SearchMode;
  caseSensitive: boolean;
  /** Ignored in regex mode (write \b yourself). */
  wholeWord: boolean;
  ignoreAccents: boolean;
  /** Regex mode only: `.` also matches a line break. */
  dotAll: boolean;
  /** Search the text as it reads: `**word**` is `word`. Plain data, so optional. */
  visibleText?: boolean;
}

/**
 * A compiled query. Plain data (structured-clonable), so it can be posted to the search worker.
 */
export interface Matcher {
  readonly spec: QuerySpec;
  /** The JavaScript regular expression source. */
  readonly source: string;
  readonly flags: string;
  /** Accents are removed from the searched text. */
  readonly fold: boolean;
  /** The searched text hides emphasis marks. */
  readonly visible: boolean;
  /** Could be slow on adversarial text: runs in the worker with a time limit. */
  readonly guarded: boolean;
  /** An empty query matches nothing. */
  readonly empty: boolean;
}

export type CompileResult = { ok: true; matcher: Matcher } | { ok: false; error: string };

const SYNTAX = /[\\^$.*+?()[\]{}|/]/g;

/** Escapes text so a RegExp (flag u) reads it literally. */
export function escapeRegex(s: string): string {
  return s.replace(SYNTAX, "\\$&");
}

const WORD = "[\\p{L}\\p{N}\\p{M}_]";

/** Reads the escapes of extended mode: `\n \t \r \0 \\ \xHH \uHHHH`. */
export function unescapeExtended(s: string): string {
  return s.replace(/\\(n|t|r|0|\\|x[0-9a-fA-F]{2}|u[0-9a-fA-F]{4})/g, (_, e: string) => {
    switch (e[0]) {
      case "n":
        return "\n";
      case "t":
        return "\t";
      case "r":
        return "\r";
      case "0":
        return "\0";
      case "\\":
        return "\\";
      default:
        return String.fromCharCode(Number.parseInt(e.slice(1), 16));
    }
  });
}

/** Translates a wildcard pattern to a regular expression source. */
function wildcardToSource(p: string): { source: string; stars: number } {
  let out = "";
  let stars = 0;
  let i = 0;
  while (i < p.length) {
    const ch = p.charAt(i);
    if (ch === "*") {
      while (p.charAt(i + 1) === "*") i++;
      out += "[^\\n]*?";
      stars++;
    } else if (ch === "?") {
      out += "[^\\n]";
    } else if (ch === "[") {
      const close = p.indexOf("]", i + 2);
      if (close === -1) {
        out += "\\[";
      } else {
        let body = p.slice(i + 1, close);
        const negate = body.startsWith("!") || body.startsWith("^");
        if (negate) body = body.slice(1);
        body = body.replace(/[\\[^]/g, "\\$&");
        out += negate ? `[^${body}\\n]` : `[${body}]`;
        i = close;
      }
    } else {
      out += escapeRegex(ch);
    }
    i++;
  }
  return { source: out, stars };
}

/** RegExp engine messages and how to say them in plain French. */
const REGEX_ERRORS: ReadonlyArray<[RegExp, string]> = [
  [/Unterminated group/i, "Parenthèse non fermée."],
  [/Unmatched '\)'/i, "Parenthèse fermante sans parenthèse ouvrante."],
  [/Nothing to repeat/i, "Un signe de répétition (*, + ou ?) n'a rien à répéter."],
  [/Unterminated character class/i, "Crochet « [ » non fermé."],
  [/at end of pattern/i, "L'expression ne peut pas finir par une barre oblique inverse."],
  [/Range out of order/i, "Intervalle à l'envers entre crochets (par exemple z-a)."],
  [/Invalid capture group name|Invalid group/i, "Groupe mal écrit."],
  [/Invalid named|named capture/i, "Référence à un groupe qui n'existe pas."],
  [/quantifier|numbers out of order/i, "Répétition {…} mal écrite."],
  [/Lone quantifier brackets/i, "Accolade seule : faites-la précéder d'une barre oblique inverse."],
  [/Invalid property name/i, "Nom de propriété Unicode inconnu."],
  [/Invalid (unicode )?escape/i, "Séquence d'échappement inconnue."],
];

/** A regular-expression error, said in plain French. */
function frenchError(message: string): string {
  for (const [re, fr] of REGEX_ERRORS) if (re.test(message)) return fr;
  return "Expression incorrecte.";
}

/**
 * Compiles a query. Never throws: a query the user is still typing is an ordinary state, and
 * the error is meant to be shown under the field.
 */
export function compileQuery(spec: QuerySpec): CompileResult {
  const fold = spec.ignoreAccents;
  const visible = spec.visibleText === true;
  const raw = fold ? foldAccents(spec.text) : spec.text;
  const base: Omit<Matcher, "source" | "flags" | "guarded" | "empty"> = { spec, fold, visible };
  if (raw === "") {
    return {
      ok: true,
      matcher: { ...base, source: "(?:)", flags: "gu", guarded: false, empty: true },
    };
  }

  let source: string;
  let guarded = false;
  switch (spec.mode) {
    case "regex":
      source = raw;
      guarded = true;
      break;
    case "wildcard": {
      const w = wildcardToSource(raw);
      source = w.source;
      guarded = w.stars >= 2;
      break;
    }
    case "extended":
      source = escapeRegex(unescapeExtended(raw));
      break;
    default:
      source = escapeRegex(raw);
  }
  if (spec.wholeWord && spec.mode !== "regex") {
    source = `(?<!${WORD})(?:${source})(?!${WORD})`;
  }
  let flags = "gu";
  if (!spec.caseSensitive) flags += "i";
  if (spec.mode === "regex") flags += spec.dotAll ? "ms" : "m";

  try {
    new RegExp(source, flags);
  } catch (e) {
    return { ok: false, error: frenchError(e instanceof Error ? e.message : String(e)) };
  }
  return { ok: true, matcher: { ...base, source, flags, guarded, empty: false } };
}
