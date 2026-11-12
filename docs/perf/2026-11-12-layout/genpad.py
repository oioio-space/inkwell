"""Write a layout pad file: k tiny reachable functions (one function-alignment slot each)."""
import sys
pkg, k, out = sys.argv[1], int(sys.argv[2]), sys.argv[3]
lines = [f'package {pkg}', 'import "os"']
for i in range(k):
    lines.append(f'//go:noinline\nfunc layoutPad{i}() {{}}')
lines.append('var layoutPads = [...]func(){' + ''.join(f'layoutPad{i},' for i in range(k)) + '}')
lines.append('func init() { if os.Getenv("INKWELL_LAYOUT_PAD_NEVER") == "\\x00" { for _, f := range layoutPads { f() } } }')
open(out, 'w').write('\n'.join(lines) + '\n')
