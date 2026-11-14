#!/usr/bin/env bash
# Runs a command with its address space capped (INKWELL_AS_LIMIT bytes, default 6 GB), so that a
# runaway test binary dies alone with an allocation failure. Meant for `go test -exec`, which takes
# a program but no arguments (GOFLAGS has no quoting): a mutant that allocates without bound then
# fails its own test instead of exhausting the machine.
#
#   INKWELL_GOFLAGS=-exec=$PWD/scripts/memcap.sh mise run mutate:go -- ./internal/trust
#
# The cap counts address space, not memory used. A cgo binary (any test binary importing net on
# Linux) gets a glibc malloc arena of 64 MB of address space per thread that allocates, up to
# 8 per core: about ten threads exhaust 1.8 GB and pthread_create fails (measured on
# internal/launch, 2026-11-12). Two arenas keep the cap a measure of what the program allocates.
set -euo pipefail
export MALLOC_ARENA_MAX="${MALLOC_ARENA_MAX:-2}"
exec prlimit --as="${INKWELL_AS_LIMIT:-6000000000}" -- "$@"
