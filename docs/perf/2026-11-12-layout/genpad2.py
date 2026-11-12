"""Minimal pad: k empty noinline functions in a file that sorts first (no import, no init, no data table), kept
reachable by one internal test function in a second overlay file that `-test.run=^$` never runs."""
import sys
pkg, k, pad, keep = sys.argv[1], int(sys.argv[2]), sys.argv[3], sys.argv[4]
open(pad, 'w').write(f'package {pkg}\n' + ''.join(f'//go:noinline\nfunc inkwellLayoutPad{i}() {{}}\n' for i in range(k)))
open(keep, 'w').write(f'package {pkg}\nimport "testing"\nfunc TestInkwellLayoutPadKeep(t *testing.T) {{\n'
                      + ''.join(f'\tinkwellLayoutPad{i}()\n' for i in range(k)) + '\tt.Skip("layout pad")\n}\n')
