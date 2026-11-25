#!/usr/bin/env python3
"""EX0c3 test codemod: the formatting language becomes an object (`Lang`, plan §1), so the tests that passed
the names "typst", "markdown" and "plain" pass the testkit's TYPST, MARKDOWN and PLAIN instead.

    python3 -I scripts/codemods/ex0c3_type_literals.py apply       # rewrite FILES in place (step 2)
    python3 -I scripts/codemods/ex0c3_type_literals.py roundtrip   # reverse(forward(file)) == file, modulo formatting, for FILES

The edit is a closed table of exact token pairs, so it reverses exactly: every target token is absent from the
files at the merge-base (checked). The gate's `mechanical` criterion carries its own copy of the reverse, frozen with
the gate; this file only writes the edit. A literal that names a file type rather than a formatting language
(`fileType: "typst"`, `id:`, `grammar:`, `toBe("typst")`) is left alone by `SKIP`.
"""
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]
SRC = ROOT / "frontend/src"
FILES = [
    "editor/format/active.test.ts",
    "editor/format/commands.prop.test.ts",
    "editor/format/commands.test.ts",
    "editor/format/insert.test.ts",
    "editor/highlight.perf.test.ts",
    "editor/highlight.test.ts",
    "editor/stats/stats.test.ts",
    "extensions/commands.test.ts",
    "ui/ExternalNotice.test.ts",
    "ui/command/FormatBar.test.ts",
    "ui/command/command.test.ts",
    "ui/command/locations.test.ts",
    "ui/command/palette.test.ts",
    "ui/foreign.test.ts",
    "ui/projectChange.test.ts",
    "ui/shell/StatusBar.problems.test.ts",
    "ui/shell/StatusBar.test.ts",
]
# Exact pairs, applied in this order forward and in the reverse order backward. Unions first, then values.
TYPES = [
    ('"typst" | "markdown" | "plain"', "AnyLang"),
    ('"typst" | "markdown"', "TestLang"),
    ("lang: string", "lang: LangArg"),
]
VALUES = [('"typst"', "TYPST"), ('"markdown"', "MARKDOWN"), ('"plain"', "PLAIN")]
NAMES = ["TYPST", "MARKDOWN", "PLAIN", "AnyLang", "TestLang", "LangArg"]
TYPE_NAMES = {"AnyLang", "TestLang", "LangArg"}
# The text just before a literal that makes it a file type, an id or an expected value, not a formatting language.
SKIP = re.compile(r"""(?:fileType|languageId|grammar|\bid|type|kind)\s*(?::|==|===|!=)\s*$|fileType == $|toBe\(\s*$|toEqual\(\s*$""")
WORD = lambda n: re.compile(r"(?<![\w$-])%s(?![\w$-])" % re.escape(n))


def testkit_path(rel: str) -> str:
    here = pathlib.PurePosixPath(rel).parent
    if here == pathlib.PurePosixPath("editor/format"):
        return "./testkit"
    up = "../" * len(here.parts)
    return f"{up}editor/format/testkit"


def forward(text: str, rel: str) -> str:
    for a, b in TYPES:
        text = text.replace(a, b)
    out, used = [], set()
    for line in text.splitlines(keepends=True):
        for a, b in VALUES:
            pieces, i = [], 0
            for m in re.finditer(re.escape(a), line):
                before = line[: m.start()]
                # `expect(langOf(…)).toBe("typst")` expects a formatting language (EX0c3r), not a type id.
                expected = "langOf(" in before and before.rstrip().endswith("toBe(")
                if (SKIP.search(before[-24:]) and not expected) or line[m.end():].lstrip().startswith("as const"):
                    continue
                pieces += [line[i:m.start()], b]
                i = m.end()
                used.add(b)
            line = "".join(pieces) + line[i:]
        out.append(line)
    text = "".join(out)
    used |= {n for n in TYPE_NAMES if WORD(n).search(text)}
    if not used:
        return text
    spec = ", ".join(("type " if n in TYPE_NAMES else "") + n for n in sorted(used, key=str.lower))
    src = testkit_path(rel)
    imp = re.compile(r'^import \{([^}]*)\} from "%s";$' % re.escape(src), re.M)
    m = imp.search(text)
    if m:
        names = [s.strip() for s in m.group(1).split(",") if s.strip()] + spec.split(", ")
        names.sort(key=lambda s: s.removeprefix("type ").lower())
        return text[: m.start()] + 'import { %s } from "%s";' % (", ".join(names), src) + text[m.end():]
    lines = text.splitlines(keepends=True)
    last = max((i for i, l in enumerate(lines) if l.startswith("import ") or l.startswith("} from ")), default=-1)
    lines.insert(last + 1, f'import {{ {spec} }} from "{src}";\n')
    return "".join(lines)


def reverse(text: str) -> str:
    def strip(m: re.Match) -> str:
        names = [s.strip() for s in m.group(1).split(",") if s.strip()]
        kept = [s for s in names if s.removeprefix("type ").strip() not in NAMES]
        return "" if not kept else "import { %s } from %s;%s" % (", ".join(kept), m.group(2), m.group(3))
    text = re.sub(r'import \{([^}]*)\} from ("(?:\./|(?:\.\./)+editor/format/)testkit");(\n?)', strip, text)
    for a, b in reversed(VALUES):
        text = WORD(b).sub(a, text)
    for a, b in reversed(TYPES):
        text = WORD(b.split()[-1]).sub(a.split()[-1], text) if b.startswith("lang: ") else WORD(b).sub(a, text)
    return text


def norm(t):
    """Formatting-blind outside strings and comments: whitespace dropped, a trailing comma before a closer dropped."""
    out, i, n = [], 0, len(t)
    while i < n:
        c = t[i]
        if c in "\"'`":
            j = i + 1
            while j < n and t[j] != c:
                j += 2 if t[j] == "\\" else 1
            out.append(t[i:j + 1])
            i = j + 1
        elif t.startswith("//", i):
            j = t.find("\n", i)
            j = n if j < 0 else j
            out.append(re.sub(r"\s+", "", t[i:j]))
            i = j
        elif t.startswith("/*", i):
            j = t.find("*/", i + 2)
            j = n if j < 0 else j + 2
            out.append(re.sub(r"\s+", "", t[i:j]))
            i = j
        elif c.isspace():
            i += 1
        else:
            out.append(c)
            i += 1
    return re.sub(r",(?=[)\]}>])", "", "".join(out))


def main() -> int:
    mode = sys.argv[1] if len(sys.argv) > 1 else ""
    bad, edits = [], 0
    for rel in FILES:
        p = SRC / rel
        base = p.read_text()
        clash = [n for n in NAMES if WORD(n).search(base)]
        if clash:
            bad.append(f"{rel}: already holds {clash}")
            continue
        new = forward(base, rel)
        edits += sum(len(WORD(n).findall(new)) for n in NAMES) - sum(len(WORD(n).findall(base)) for n in NAMES)
        if norm(reverse(new)) != norm(base):
            bad.append(f"{rel}: the reverse does not give the file back")
        if mode == "apply":
            p.write_text(new)
    print(f"codemod {mode}: {len(FILES)} files, {edits} edits, problems {bad}")
    return 1 if bad or mode not in ("apply", "roundtrip") else 0


if __name__ == "__main__":
    sys.exit(main())
