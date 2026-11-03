#!/usr/bin/env bash
# Native Linux CI trace collection; no downloads or unverified executables.
set -euo pipefail
scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
export SIDECAR_TRACE_SCRATCH="$scratch"
python3 - <<'PY'
import gzip
import hashlib
import json
import os
import platform
from pathlib import Path
arch = {'x86_64': 'amd64', 'aarch64': 'arm64'}[platform.machine()]
pins = json.loads(Path('internal/sidecar/manifest.json').read_text())['sidecars']
for name in ('typst', 'tinymist'):
    data = gzip.decompress(Path(f'internal/sidecar/bin/linux_{arch}/{name}.gz').read_bytes())
    if hashlib.sha256(data).hexdigest() != pins[name]['targets'][f'linux_{arch}']['sha256']:
        raise SystemExit('sidecar pin mismatch')
    target = Path(os.environ['SIDECAR_TRACE_SCRATCH']) / name
    target.write_bytes(data)
    target.chmod(0o700)
corpus = Path('testdata/perf/generated/text/main.typ')
data = (b'Performance reference text.\n' * 90000)[:2*1024*1024]
pin = json.loads(Path('testdata/perf/corpus.json').read_text())['files']['text/main.typ']['sha256']
if hashlib.sha256(data).hexdigest() != pin:
    raise SystemExit('PM-1 pin mismatch')
corpus.parent.mkdir(parents=True, exist_ok=True)
corpus.write_bytes(data)
PY
export TYPST="$scratch/typst" TINYMIST="$scratch/tinymist"
python3 scripts/sidecar-trace.py "${1:-gates/L1/ci-seccomp-traces}"
python3 scripts/sidecar-allowlist.py --check "${1:-gates/L1/ci-seccomp-traces}"
