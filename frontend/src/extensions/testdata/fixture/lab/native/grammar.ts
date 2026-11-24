import { LanguageSupport, StreamLanguage } from "@codemirror/language";

/** A test grammar: every word is a keyword, so its nodes are easy to find in a tree. */
export function grammar(): LanguageSupport {
  return new LanguageSupport(
    StreamLanguage.define({
      name: "lab",
      token(stream) {
        if (stream.match(/^\w+/)) return "keyword";
        stream.next();
        return null;
      },
    }),
  );
}
