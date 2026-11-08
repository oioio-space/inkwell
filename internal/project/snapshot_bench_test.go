package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkProjectionSnapshotLarge represents a project with many source files
// and 32 MiB of unchanged binary dependencies; each compile changes one source.
func BenchmarkProjectionSnapshotLarge(b *testing.B) {
	base := newBase(b)
	p, err := Create(base, "Snapshot benchmark", "automerge:abc")
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	asset := make([]byte, 2<<20)
	for i := range asset {
		asset[i] = byte(i % 256)
	}
	for i := range 16 {
		if err := os.WriteFile(filepath.Join(p.Path(), fmt.Sprintf("asset-%02d.bin", i)), asset, 0o600); err != nil {
			b.Fatal(err)
		}
	}
	for i := range 100 {
		if err := os.WriteFile(filepath.Join(p.Path(), fmt.Sprintf("chapter-%03d.typ", i)), []byte("#let chapter = 1\n"), 0o600); err != nil {
			b.Fatal(err)
		}
	}
	targets := b.TempDir()
	b.ReportAllocs()
	b.SetBytes(32 << 20)
	b.ResetTimer()
	i := 0
	for b.Loop() {
		i++
		if err := os.WriteFile(filepath.Join(p.Path(), "main.typ"), fmt.Appendf(nil, "#let changed = %d", i), 0o600); err != nil {
			b.Fatal(err)
		}
		target, err := os.MkdirTemp(targets, "compile-")
		if err != nil {
			b.Fatal(err)
		}
		snap, err := p.SnapshotTo(context.Background(), target)
		if err != nil {
			b.Fatal(err)
		}
		if !p.MatchesSnapshot(context.Background(), snap) {
			b.Fatal("current snapshot lost identity")
		}
		if err := os.RemoveAll(target); err != nil {
			b.Fatal(err)
		}
	}
}
