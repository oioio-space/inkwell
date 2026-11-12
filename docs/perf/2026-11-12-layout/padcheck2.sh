#!/bin/bash
# Minimal pads: offsets realized, inlining/escape decisions, normalized assembly and data of mux, per pad size.
S=$1; R=/home/mathieu/GolandProjects/inkwell; cd $R; d=$R/internal/transport/mux
for k in none 0 1 37 63; do
  if [ $k = none ]; then ov=""; else python3 $S/genpad2.py mux $k $S/p2-$k.go $S/p2-$k-keep.go; echo "{\"Replace\":{\"$d/0inkwellpad.go\":\"$S/p2-$k.go\",\"$d/zz_inkwellpad_test.go\":\"$S/p2-$k-keep.go\"}}" > $S/p2-$k.json; ov="-overlay=$S/p2-$k.json"; fi
  go test -c $ov -gcflags='github.com/oioio-space/inkwell/internal/transport/mux=-m=2' -ldflags=-funcalign=64 -o $S/p2-$k.test ./internal/transport/mux 2>&1 | grep -v -i inkwellLayoutPad | grep -v '^#' | grep -v '_test.go' | sed -E 's/^([^:]+):[0-9]+:[0-9]+:/\1:/' | sort > $S/p2-m-$k.txt
  go tool objdump -s 'transport/mux\.' $S/p2-$k.test | awk '/^TEXT/{skip = ($2 ~ /inkwellLayoutPad|Test|Benchmark|_test/); if (!skip) print "TEXT", $2; next} !skip && NF>=4 {$1=""; $2=""; print}' | sed -E 's/0x[0-9a-f]{5,}/ADDR/g' > $S/p2-asm-$k.txt
  go tool nm -size -sort address $S/p2-$k.test | awk '$3 ~ /^[DdBbRr]$/ && $4 ~ /transport\/mux\./ {print $4, $2}' | sort > $S/p2-data-$k.txt
  send=$(go tool nm -n $S/p2-$k.test | awk '/mux\.\(\*sub\)\.Send$/{print $1}')
  echo "k=$k Send=0x$send m-lines=$(wc -l < $S/p2-m-$k.txt) asm-lines=$(wc -l < $S/p2-asm-$k.txt) data-syms=$(wc -l < $S/p2-data-$k.txt)"
done
for k in 0 1 37 63; do
  echo "== k=$k vs none: -m diff $(diff $S/p2-m-none.txt $S/p2-m-$k.txt | grep -c '^[<>]'), asm diff $(diff $S/p2-asm-none.txt $S/p2-asm-$k.txt | grep -c '^[<>]'), data diff $(diff $S/p2-data-none.txt $S/p2-data-$k.txt | grep -c '^[<>]')"
  diff $S/p2-asm-none.txt $S/p2-asm-$k.txt | head -4; diff $S/p2-data-none.txt $S/p2-data-$k.txt | head -4
done
