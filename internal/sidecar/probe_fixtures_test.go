package sidecar

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// L1 builds these fixtures before starting the suite with PATH empty. Regular
// go test runs build only the fixtures their selected tests use.
var preparedProbeFixtures = flag.String("sidecar-probe-fixtures", "", "directory of prebuilt sidecar probe fixtures")

func probeFixture(t *testing.T, ctx context.Context, name string) string {
	t.Helper()
	dir := *preparedProbeFixtures
	if dir == "" {
		dir = t.TempDir()
		if err := buildProbeFixture(ctx, dir, name); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, name)
}

func buildProbeFixtures(dir string) int {
	for _, name := range []string{"spin", "version", "runpath", "glibc-version"} {
		if err := buildProbeFixture(context.Background(), dir, name); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return 0
}

func buildProbeFixture(ctx context.Context, dir, name string) error {
	target := filepath.Join(dir, name)
	switch name {
	case "spin":
		// Hang without making a system call that the filter might reject.
		return compileProbeFixture(ctx, target, "int main(void) { for (;;) {} }\n")
	case "version":
		// Check the argument and an ioctl allowed by the Typst descriptor.
		return compileProbeFixture(ctx, target, `#include <errno.h>
#include <string.h>
#include <sys/ioctl.h>
int main(int argc, char **argv) {
	struct winsize ws;
	if (argc != 2 || strcmp(argv[1], "--version") != 0) return 3;
	if (ioctl(0, TIOCGWINSZ, &ws) != 0 && errno == EPERM) return 4;
	return 0;
}
`)
	case "runpath":
		// The only copy of the library is in DT_RUNPATH, as for the
		// WebKitGTK dependency that caused UX-SB2a1-1.
		lib := filepath.Join(dir, "private")
		if err := os.Mkdir(lib, 0o700); err != nil {
			return err
		}
		if err := compileProbeFixture(ctx, filepath.Join(lib, "libinkwellprobedep.so"),
			"int inkwell_probe_dep(void) { return 0; }\n", "-shared", "-fPIC"); err != nil {
			return err
		}
		return compileProbeFixture(ctx, target,
			"int inkwell_probe_dep(void);\nint main(void) { return inkwell_probe_dep(); }\n",
			"-L"+lib, "-linkwellprobedep", "-Wl,--enable-new-dtags,-rpath,"+lib)
	case "glibc-version":
		out, err := exec.CommandContext(ctx, "getconf", "GNU_LIBC_VERSION").Output()
		if err != nil {
			return fmt.Errorf("S-6 clone3: glibc version: %w", err)
		}
		return os.WriteFile(target, out, 0o600)
	default:
		return fmt.Errorf("unknown probe fixture %q", name)
	}
}

func compileProbeFixture(ctx context.Context, target, source string, linkArgs ...string) error {
	args := append([]string{"-o", target, "-x", "c", "-"}, linkArgs...)
	cmd := exec.CommandContext(ctx, "cc", args...) // #nosec G204 -- builds fixed test sources.
	cmd.Stdin = strings.NewReader(source)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("build probe fixture %s: %w\n%s", filepath.Base(target), err, out)
	}
	return nil
}
