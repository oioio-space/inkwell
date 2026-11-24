#!/usr/bin/env python3
"""Run the Alloy models (docs/protocol/alloy) and their mutants. Development tool, never shipped.

Every `check` of a sound model must find no counterexample and every `run` must find an instance.
Every mutant (a rule removed by a text substitution) must make its named assertion fail: proof that
the assertion constrains the rule. The Alloy jar is pinned by SHA-256 and cached.
"""
import ast
import functools
import hashlib
import json
import os
import pathlib
import signal
import subprocess
import sys
import tempfile

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import fetch_pinned  # noqa: E402
import formal_cache  # noqa: E402

ROOT = pathlib.Path(__file__).resolve().parent.parent
MODELS = ROOT / "docs/protocol/alloy"
JAR_URL = "https://github.com/AlloyTools/org.alloytools.alloy/releases/download/v6.2.0/org.alloytools.alloy.dist.jar"
JAR_SHA256 = "6b8c1cb5bc93bedfc7c61435c4e1ab6e688a242dc702a394628d9a9801edb78d"
CACHE = pathlib.Path(os.environ.get("INKWELL_ALLOY_CACHE", pathlib.Path.home() / ".cache/inkwell/tools")) / "alloy-6.2.0.jar"

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
    ("Registry.als", "RefusalWins",
     "fun rank[v: Value]: Int { v = Kept => 2 else (v = Trashed => 1 else 0) }",
     "fun rank[v: Value]: Int { v = Purged => 2 else (v = Trashed => 1 else 0) }"),
    ("Tree.als", "Acyclic",
     "(no n.parent or n in breakers) => p = Root else p = n.parent",
     "(no n.parent) => p = Root else p = n.parent"),
    ("Tree.als", "OneParent",
     "(no n.parent or n in breakers) => p = Root else p = n.parent",
     "(n in breakers) => p = Root else p = n.parent"),
    # HP1 (plan 2026-11-16-hp1-direct-upgrade.md §3.4, H-3): who initiates the DCUtR exchange.
    ("DirectUpgradeRoles.als", "SomeEligibleAccepted",   # lower PeerID only: stalls on the lower side's outbound
     "pred eligible[s: Side] { holdsInbound[s] }",
     "pred eligible[s: Side] { holdsInbound[s] and s in Lower }"),
    ("DirectUpgradeRoles.als", "AcceptedIsEligible",     # lower PeerID only forbids the side that would succeed
     "pred eligible[s: Side] { holdsInbound[s] }",
     "pred eligible[s: Side] { holdsInbound[s] and s in Lower }"),
    ("DirectUpgradeRoles.als", "PunchNeedsInbound",      # the outbound side punches too
     "pred eligible[s: Side] { holdsInbound[s] }",
     "pred eligible[s: Side] { some Circuit }"),
    ("DirectUpgradeRoles.als", "SomeEligibleAccepted",   # NewStream picks the oldest: both ends pick their outbound
     "localTime[a, s] > localTime[b, s])",
     "localTime[a, s] < localTime[b, s])"),
    ("DirectUpgradeRoles.als", "SomeEligibleAccepted",   # no handshake delay: a full tie lets both pick their outbound
     "all c: Circuit | c.delay >= 1   --",
     "all c: Circuit | c.delay >= 0   --"),
]


def java_ok():
    out = subprocess.run(["java", "-version"], capture_output=True, text=True)
    if out.returncode != 0:
        sys.exit("alloy: Java >= 17 is required (dev tool only): install a JDK or use the CI runner's")


def jar():
    return fetch_pinned.fetch(JAR_URL, JAR_SHA256, CACHE)


def run(model: pathlib.Path, scratch: pathlib.Path):
    """Return {command: (type, has_solution)} for one model file.

    java.io.tmpdir is scratch: Java ignores TMPDIR, and Alloy's SAT back end leaves an empty
    alloy-plingeling<n>.cnf in it for every command it solves (1 077 of them in /tmp by 2026-11-23).
    """
    with tempfile.TemporaryDirectory() as out:
        proc = subprocess.run(
            ["java", "-Xmx2g", f"-Djava.io.tmpdir={scratch}", "--enable-native-access=ALL-UNNAMED",
             "-jar", str(jar()), "exec", "-f", "-o", out, str(model)],
            capture_output=True, text=True, timeout=900)
        receipt = pathlib.Path(out) / "receipt.json"
        if proc.returncode != 0 or not receipt.exists():
            sys.exit(f"alloy: {model.name} failed:\n{proc.stdout[-2000:]}{proc.stderr[-2000:]}")
        commands = json.loads(receipt.read_text())["commands"]
        return {name: (c["type"], "solution" in c) for name, c in commands.items()}


def _exit_on_signal(signum, _frame):
    """SIGTERM and SIGHUP end the run through SystemExit, so that every `with` below removes its
    folder (subprocess.run kills the running java first); Python's default would leave them."""
    sys.exit(128 + signum)


def main():
    java_ok()
    for sig in (signal.SIGTERM, signal.SIGHUP):
        signal.signal(sig, _exit_on_signal)
    # One scratch folder for the whole run, removed on every exit but SIGKILL.
    with tempfile.TemporaryDirectory(prefix="inkwell-alloy-") as scratch:
        check(pathlib.Path(scratch))


def verdict_key(text: str, args: list[str]):
    """The verdict cache key of one Alloy execution (scripts/formal_cache.py), or None when bypassed or when an
    opened module cannot be named exactly. The runner part is this whole file minus the MUTANTS list (runner_sha)."""
    if formal_cache.disabled():
        return None
    try:
        files = formal_cache.alloy_closure(MODELS, text)
    except formal_cache.Unresolved as exc:
        print(f"formal-cache: not cacheable: {exc}", file=sys.stderr)
        return None
    files["model"] = formal_cache.sha256(text.encode())
    runner = runner_sha(pathlib.Path(__file__).read_text())
    return formal_cache.key("alloy", files, JAR_SHA256, java_version(), runner, args)


def runner_sha(source: str) -> str:
    """SHA-256 of this runner's whole source minus the MUTANTS list, which is data: every function, constant and
    import that runs or judges a check is in the key, and adding a mutant changes no other key (its own key holds its
    mutated text and assertion)."""
    lines = source.splitlines(keepends=True)
    for node in ast.parse(source).body:
        if isinstance(node, ast.Assign) and any(getattr(t, "id", None) == "MUTANTS" for t in node.targets):
            del lines[node.lineno - 1:node.end_lineno]
            break
    else:
        raise ValueError("alloy_check.py: no MUTANTS list")
    return formal_cache.sha256("".join(lines).encode())


@functools.cache
def java_version():
    return subprocess.run(["java", "-version"], capture_output=True, text=True).stderr


def cached(key, label):
    """Prints a reused verdict's lines and returns True on a hit."""
    lines = formal_cache.get(key) if key else None
    if lines is None:
        return False
    print("\n".join(lines))
    print(formal_cache.reused_line(key, label))
    return True


def check(scratch: pathlib.Path):
    failures = []
    for model in sorted(MODELS.glob("*.als")):
        text = model.read_text()
        key = verdict_key(text, [f"model={model.name}"])
        if cached(key, model.name):
            continue
        lines, green = [], True
        for name, (kind, solved) in run(model, scratch).items():
            ok = (not solved) if kind == "check" else solved
            lines.append(f"{model.name} {kind} {name}: {'ok' if ok else 'FAIL'}")
            print(lines[-1])
            if not ok:
                green = False
                failures.append(f"{model.name}:{name}")
        if key and green and lines:
            formal_cache.put(key, model.name, lines)
    for model, assertion, before, after in MUTANTS:
        text = (MODELS / model).read_text()
        if before not in text:
            failures.append(f"mutant {model}:{assertion}: anchor text not found")
            continue
        mutated = text.replace(before, after, 1)
        key = verdict_key(mutated, [f"mutant={model}", f"assertion={assertion}"])
        if cached(key, f"mutant {model} -> {assertion}"):
            continue
        with tempfile.TemporaryDirectory() as d:
            mutant = pathlib.Path(d) / model
            mutant.write_text(mutated)
            kind, solved = run(mutant, scratch)[assertion]
            caught = kind == "check" and solved
            line = f"mutant {model} -> {assertion}: {'caught' if caught else 'NOT CAUGHT'}"
            print(line)
            if not caught:
                failures.append(f"mutant {model}:{assertion}")
            elif key:
                formal_cache.put(key, f"mutant {model} -> {assertion}", [line])
    if failures:
        sys.exit("alloy: " + ", ".join(failures))


if __name__ == "__main__":
    main()
