#!/usr/bin/env python3
"""Keeps CLAUDE.md true to the repository (rule « Cohérence de CLAUDE.md »).

Fails when CLAUDE.md cites something that does not exist:
- a `mise run <task>` whose task is not defined;
- a backticked repository path that is missing (a path not created yet is allowed on a line
  that says when it comes: « (étape N) »);
- an agent (`agent X`, routing table) missing from .claude/agents/, or a project skill
  (`skill X`) missing from .claude/skills/ (plugin skills `a:b` and built-ins are skipped);
- a HOT_PATHS package that is missing;
and when a Claude hook registered in .claude/settings.json is not named in CLAUDE.md.

Usage: check_claude_md.py [repo-root]   (default: the parent of this script's directory)
"""
import json
import pathlib
import re
import subprocess
import sys

ROOTS = ("scripts/", "docs/", ".claude/", ".githooks/", "gates/", "internal/", "protocol/",
         "frontend/", "cmd/", "build/")
BUILTIN_SKILLS = {"simplify"}
PENDING = re.compile(r"\(étape \d+[^)]*\)")


def mise_tasks(root: pathlib.Path) -> set[str]:
    out = subprocess.run(["mise", "tasks", "--no-header"], cwd=root, capture_output=True,
                         text=True, check=True).stdout
    return {line.split()[0] for line in out.splitlines() if line.strip()}


def problems(root: pathlib.Path, text: str, tasks: set[str]) -> list[str]:
    found = []
    for n, line in enumerate(text.splitlines(), 1):
        for t in re.findall(r"mise run ([a-z][a-z0-9:_*-]*)", line):
            ok = any(x.startswith(t[:-1]) for x in tasks) if t.endswith("*") else t in tasks
            if not ok:
                found.append(f"{n}: mise task `{t}` is not defined")
        for p in re.findall(r"`([^`\s]+)`", line):
            p = p.rstrip("/")
            if (p.startswith(ROOTS) or p in (".mcp.json", ".env.reference")) \
                    and not re.search(r"[<>*$]", p) and not PENDING.search(line) \
                    and not (root / p).exists():
                found.append(f"{n}: path `{p}` does not exist")
        for a in re.findall(r"agent `([a-z][a-z-]+)`", line) + \
                re.findall(r"^\|[^|]+\| `([a-z][a-z-]+)`", line):
            if not (root / ".claude/agents" / f"{a}.md").exists():
                found.append(f"{n}: agent `{a}` is missing from .claude/agents/")
        for group in re.findall(r"skills? ((?:`[^`]+`(?:, | et | ou )?)+)", line):
            for s in re.findall(r"`([^`]+)`", group):
                if ":" not in s and s not in BUILTIN_SKILLS \
                        and not (root / ".claude/skills" / s).is_dir():
                    found.append(f"{n}: skill `{s}` is missing from .claude/skills/")
        if line.startswith("HOT_PATHS:"):
            for pkg in line.split()[1:]:
                if not (root / pkg).is_dir():
                    found.append(f"{n}: HOT_PATHS package `{pkg}` does not exist")
    settings = json.loads((root / ".claude/settings.json").read_text())
    for groups in settings.get("hooks", {}).values():
        for group in groups:
            for hook in group.get("hooks", []):
                argv = hook.get("command", "").split()
                name = pathlib.PurePath(argv[0]).name if argv else ""
                msg = f"Claude hook `{name}` (.claude/settings.json) is not named"
                if name and name not in text and msg not in found:
                    found.append(msg)
    return found


def docs_map_problems(root: pathlib.Path) -> list[str]:
    """Findings (already prefixed with their file) on docs/README.md, the map of docs/."""
    readme = root / "docs/README.md"
    if not readme.is_file():
        return ["docs/README.md: file is missing"]
    first, found = set(), []
    for n, line in enumerate(readme.read_text().splitlines(), 1):
        for target in re.findall(r"\[[^\]]*\]\(([^)\s]+)\)", line):
            path = target.split("#")[0]
            if not path or re.match(r"[a-z][a-z0-9+.-]*:", path):
                continue
            if not (readme.parent / path).exists():
                found.append(f"docs/README.md:{n}: link `{target}` does not resolve")
            parts = pathlib.PurePosixPath(path).parts
            if parts and parts[0] != "..":
                first.add(parts[1] if parts[0] == "." and len(parts) > 1 else parts[0])
    for entry in sorted(e.name for e in readme.parent.iterdir()):
        if not entry.startswith(".") and entry != "README.md" and entry not in first:
            found.append(f"docs/README.md: top-level entry `docs/{entry}` is not linked")
    return found


def main() -> int:
    root = pathlib.Path(sys.argv[1] if len(sys.argv) > 1
                        else pathlib.Path(__file__).resolve().parent.parent)
    found = [f"CLAUDE.md:{f}" for f in
             problems(root, (root / "CLAUDE.md").read_text(), mise_tasks(root))]
    found += docs_map_problems(root)
    for f in found:
        print(f, file=sys.stderr)
    return 1 if found else 0


if __name__ == "__main__":
    sys.exit(main())
