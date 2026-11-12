#!/bin/bash
# Build layout-variant mux test binaries for HEAD (base) and the working tree (C5S).
set -e
S=$1; R=/home/mathieu/GolandProjects/inkwell; B=$S/basetree
if [ ! -d $B ]; then git clone -q --bare --shared $R $S/repo.git; git --git-dir=$S/repo.git worktree add -q --detach $B HEAD; fi
mkdir -p $S/bin
build() { # tree tag flags k
  local d=$1/internal/transport/mux; python3 $S/genpad.py mux $4 $S/pad-$2.go
  echo "{\"Replace\":{\"$d/0layoutpad.go\":\"$S/pad-$2.go\"}}" > $S/ov-$2.json
  (cd $1 && go test -c $3 -overlay=$S/ov-$2.json -o $S/bin/$2.test ./internal/transport/mux)
  printf '%s Send%%64=%d\n' $2 $(( 0x$(go tool nm -n $S/bin/$2.test | awk '/mux\.\(\*sub\)\.Send$/{print $1}') % 64 ))
}
build $B base-def-k0 "" 0; build $R c5s-def-k0 "" 0; build $R c5s-def-k1 "" 1
build $B base-fa-k0 "-ldflags=-funcalign=64" 0; build $R c5s-fa-k0 "-ldflags=-funcalign=64" 0
for i in $(seq 0 11); do k=$(( (i*37+5) % 64 )); build $R c5s-fa-r$k "-ldflags=-funcalign=64" $k; done
for i in $(seq 0 11); do k=$(( (i*53+3) % 128 )); build $R c5s-def-r$k "" $k; done
for i in $(seq 0 11); do k=$(( (i*37+5) % 64 )); build $B base-fa-r$k "-ldflags=-funcalign=64" $k; done
