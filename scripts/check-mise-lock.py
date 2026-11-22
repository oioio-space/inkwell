#!/usr/bin/env python3
"""Check the project lock's tool versions and Linux entries without invoking mise."""

import sys
import tomllib
from pathlib import Path

PLATFORMS = ("linux-x64", "linux-arm64")


def problems(config, lock):
    for tool, requests in config.items():
        entries = lock.get(tool, [])
        if isinstance(requests, (str, dict)):
            requests = [requests]
        source_build = tool.startswith("go:")
        for request in requests:
            version = request["version"] if isinstance(request, dict) else request
            if version == "latest":
                if not entries:
                    continue
                matches = [
                    entry for entry in entries
                    if "latest" in entry.get("specifiers", ["latest"])
                    and entry["version"] not in ("", "latest")
                ]
            elif source_build:
                # mise can strip the Go module tag's leading v in the lock.
                matches = [
                    entry for entry in entries
                    if entry["version"].removeprefix("v") == version.removeprefix("v")
                ]
            else:
                matches = [entry for entry in entries if entry["version"] == version]

            if not matches:
                recorded = ", ".join(entry["version"] for entry in entries) or "none"
                yield f"{tool}@{version} is missing (locked: {recorded})"
                continue

            for entry in matches:
                # The Go backend builds these tools from a module version on both targets.
                if source_build and not any(key.startswith("platforms.") for key in entry):
                    continue
                missing = [name for name in PLATFORMS if not entry.get(f"platforms.{name}")]
                if missing:
                    yield f"{tool}@{entry['version']} is missing {', '.join(missing)}"


def main():
    try:
        with Path("mise.toml").open("rb") as file:
            config = tomllib.load(file)["tools"]
        with Path("mise.lock").open("rb") as file:
            lock = tomllib.load(file).get("tools", {})
        errors = list(problems(config, lock))
    except (OSError, tomllib.TOMLDecodeError, KeyError, TypeError) as error:
        print(f"lint:lock: {error}", file=sys.stderr)
        return 1

    if errors:
        for error in errors:
            print(f"mise.lock: {error}", file=sys.stderr)
        print("Refresh mise.lock with mise lock (see the DEP1 plan).", file=sys.stderr)
        return 1
    print("mise.lock matches mise.toml for linux-x64 and linux-arm64")
    return 0


if __name__ == "__main__":
    sys.exit(main())
