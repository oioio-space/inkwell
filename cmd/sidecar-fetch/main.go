// Command sidecar-fetch downloads the pinned typst and tinymist releases and
// prepares the files the executable embeds (internal/sidecar/bin). It runs at
// build time (mise run sidecar:fetch), never inside the application:
//
//	sidecar-fetch                      # this machine's platform
//	sidecar-fetch -target linux_arm64  # another platform
//	sidecar-fetch -target all          # every pinned platform
//
// Archives are checked against the SHA-256 in internal/sidecar/manifest.json
// and so are the binaries inside them; a second run needs no network. It needs
// tar (with xz) to read the release archives.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"

	"github.com/oioio-space/inkwell/internal/sidecar"
	"github.com/oioio-space/inkwell/internal/sidecar/fetch"
)

func main() {
	os.Exit(exitCode(os.Args[1:], os.Stdout, os.Stderr))
}

// exitCode runs the command until it finishes or is interrupted and returns
// the process exit status, printing a failure on stderr.
func exitCode(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, args, stdout, stderr); err != nil {
		_, _ = fmt.Fprintln(stderr, "sidecar-fetch:", err)
		return 1
	}
	return 0
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("sidecar-fetch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	targets := flags.String("target", sidecar.HostTarget(), `platform "<goos>_<goarch>", a comma-separated list, or "all"`)
	out := flags.String("out", filepath.Join("internal", "sidecar", "bin"), "directory that receives <target>/<name>.gz")
	cache := flags.String("cache", "", "download cache (default: the user cache directory, inkwell/sidecar-dl)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	manifest, err := sidecar.LoadManifest()
	if err != nil {
		return err
	}
	list, err := resolveTargets(manifest, *targets)
	if err != nil {
		return err
	}
	if *cache == "" {
		dir, err := sidecar.DefaultDir()
		if err != nil {
			return err
		}
		*cache = filepath.Join(dir, "sidecar-dl")
	}
	return fetch.Fetch(ctx, fetch.Config{
		Manifest: manifest, Targets: list, OutDir: *out, CacheDir: *cache, Log: stdout,
	})
}

// resolveTargets expands "all" and checks every name against the manifest.
func resolveTargets(m *sidecar.Manifest, spec string) ([]string, error) {
	known := map[string]bool{}
	for _, sc := range m.Sidecars {
		for t := range sc.Targets {
			known[t] = true
		}
	}
	if spec == "all" {
		list := make([]string, 0, len(known))
		for t := range known {
			list = append(list, t)
		}
		slices.Sort(list)
		return list, nil
	}
	list := strings.Split(spec, ",")
	for _, t := range list {
		if !known[t] {
			return nil, errors.New("no pinned build for " + t + ` (try -target all to see them in manifest.json)`)
		}
	}
	return list, nil
}
