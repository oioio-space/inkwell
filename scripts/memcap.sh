#!/usr/bin/env bash
# Runs a command with its address space capped (INKWELL_AS_LIMIT bytes, default 6 GB), so that a
# runaway test binary dies alone with an allocation failure. Meant for `go test -exec`, which takes
# a program but no arguments (GOFLAGS has no quoting): a mutant that allocates without bound then
# fails its own test instead of exhausting the machine.
#
#   INKWELL_GOFLAGS=-exec=$PWD/scripts/memcap.sh mise run mutate:go -- ./internal/trust
set -euo pipefail
exec prlimit --as="${INKWELL_AS_LIMIT:-6000000000}" -- "$@"
