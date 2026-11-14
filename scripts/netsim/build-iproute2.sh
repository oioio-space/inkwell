#!/usr/bin/env bash
# Build the pinned iproute2 release into a prefix, for the netsim job: `tc qdisc ... netem ... seed N`
# (topology.sh seeds every qdisc so that the draws are replayable) needs iproute2 6.6.0 (upstream change
# a67cf9a2523c of 2023-08-28, first released in v6.6.0; v6.5.0 does not know "seed") and Ubuntu 24.04
# ships 6.1.0. The tarball is fetched through scripts/fetch_pinned.py, which refuses any other SHA-256
# (the one kernel.org publishes in sha256sums.asc next to the tarball).
#
# Usage: scripts/netsim/build-iproute2.sh PREFIX   # leaves PREFIX/sbin/tc; no-op when it is already there
# Needs: gcc, make, bison, flex, pkg-config (Debian/Ubuntu: sudo apt-get install bison flex pkg-config).
set -euo pipefail

version=6.12.0
sha256=bbd141ef7b5d0127cc2152843ba61f274dc32814fa3e0f13e7d07a080bef53d9
url="https://mirrors.edge.kernel.org/pub/linux/utils/net/iproute2/iproute2-$version.tar.xz"

prefix=${1:?usage: build-iproute2.sh PREFIX}
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

if [[ -x $prefix/sbin/tc ]] && "$prefix/sbin/tc" -V 2>&1 | grep -q "iproute2-$version"; then
  echo "iproute2 $version already built in $prefix"
  exit 0
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
python3 -I "$here/../fetch_pinned.py" "$url" "$sha256" "$work/iproute2.tar.xz"
tar -xJf "$work/iproute2.tar.xz" -C "$work"
cd "$work/iproute2-$version"
./configure >/dev/null
make -j"$(nproc)" >/dev/null
make DESTDIR="$work/stage" PREFIX=/usr SBINDIR=/sbin install >/dev/null
mkdir -p "$prefix/sbin"
install -m 0755 "$work/stage/sbin/tc" "$prefix/sbin/tc"
"$prefix/sbin/tc" -V
