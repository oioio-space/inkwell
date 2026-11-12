#!/bin/bash
# Interleaved A/B, one sample per process, order reversed every other round (as regression.py).
D=$1; tag=$2; : > $D/$tag-base.txt; : > $D/$tag-c5s.txt
for r in $(seq 0 19); do
  if (( r % 2 == 0 )); then order="base c5s"; else order="c5s base"; fi
  for s in $order; do $D/$s.test -test.run='^$' -test.bench='Send' -test.benchmem -test.count=1 >> $D/$tag-$s.txt; done
done
