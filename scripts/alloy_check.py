#!/usr/bin/env python3
"""Run the Alloy models (docs/protocol/alloy) and their mutants. Development tool, never shipped.

Every `check` of a sound model must find no counterexample and every `run` must find an instance.
Every mutant (a rule removed by a text substitution) must make its named assertion fail: proof that
the assertion constrains the rule. The Alloy jar is pinned by SHA-256 and cached.
"""
import hashlib
import json
import pathlib
import shutil
import subprocess
import sys
import tempfile
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parent.parent
MODELS = ROOT / "docs/protocol/alloy"
JAR_URL = "https://github.com/AlloyTools/org.alloytools.alloy/releases/download/v6.2.0/org.alloytools.alloy.dist.jar"
JAR_SHA256 = "6b8c1cb5bc93bedfc7c61435c4e1ab6e688a242dc702a394628d9a9801edb78d"
CACHE = pathlib.Path.home() / ".cache/inkwell/tools/alloy-6.2.0.jar"

# (model, assertion that must now fail, original text, mutated text)
MUTANTS = [
    ("Decisions.als", "AntedatedStaysDead",
     "c.author != d.author and concurrent[c, d]               -- (b)",
     "c.author != d.author and concurrent[c, d] and some none -- (b) removed"),
    ("Decisions.als", "RevocationEffective",
     "no c: ms | c.kind = Cut and c.target = k",
     "some ms or no ms -- cut ignored"),
    ("Decisions.als", "NoEscalation",
     "adminAt[d.author, knownMembers[d]]                          -- (a)",
     "some d.author                                                -- (a) removed"),
]


def jar():
    if not CACHE.exists() or hashlib.sha256(CACHE.read_bytes()).hexdigest() != JAR_SHA256:
        CACHE.parent.mkdir(parents=True, exist_ok=True)
        tmp = CACHE.with_suffix(".part")
        with urllib.request.urlopen(JAR_URL, timeout=120) as r, open(tmp, "wb") as f:
            shutil.copyfileobj(r, f)
        digest = hashlib.sha256(tmp.read_bytes()).hexdigest()
        if digest != JAR_SHA256:
            tmp.unlink()
            sys.exit(f"alloy: jar SHA-256 {digest} differs from the pinned {JAR_SHA256}")
        tmp.replace(CACHE)
    return CACHE


def run(model: pathlib.Path):
    """Return {command: (type, has_solution)} for one model file."""
    with tempfile.TemporaryDirectory() as out:
        proc = subprocess.run(
            ["java", "-Xmx2g", "--enable-native-access=ALL-UNNAMED", "-jar", str(jar()),
             "exec", "-f", "-o", out, str(model)],
            capture_output=True, text=True, timeout=900)
        receipt = pathlib.Path(out) / "receipt.json"
        if proc.returncode != 0 or not receipt.exists():
            sys.exit(f"alloy: {model.name} failed:\n{proc.stdout[-2000:]}{proc.stderr[-2000:]}")
        commands = json.loads(receipt.read_text())["commands"]
        return {name: (c["type"], "solution" in c) for name, c in commands.items()}


def main():
    failures = []
    for model in sorted(MODELS.glob("*.als")):
        for name, (kind, solved) in run(model).items():
            ok = (not solved) if kind == "check" else solved
            print(f"{model.name} {kind} {name}: {'ok' if ok else 'FAIL'}")
            if not ok:
                failures.append(f"{model.name}:{name}")
    for model, assertion, before, after in MUTANTS:
        text = (MODELS / model).read_text()
        if before not in text:
            failures.append(f"mutant {model}:{assertion}: anchor text not found")
            continue
        with tempfile.TemporaryDirectory() as d:
            mutant = pathlib.Path(d) / model
            mutant.write_text(text.replace(before, after, 1))
            kind, solved = run(mutant)[assertion]
            caught = kind == "check" and solved
            print(f"mutant {model} -> {assertion}: {'caught' if caught else 'NOT CAUGHT'}")
            if not caught:
                failures.append(f"mutant {model}:{assertion}")
    if failures:
        sys.exit("alloy: " + ", ".join(failures))


if __name__ == "__main__":
    main()
