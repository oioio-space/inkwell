package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/oioio-space/inkwell/internal/project"

	"github.com/oioio-space/inkwell/internal/typst"
)

type diagnosticCompiler struct {
	Compiler
	compiled bool
	read     func(typst.Project)
}

func (c *diagnosticCompiler) Compile(_ context.Context, p typst.Project, _ string, _ ...typst.Option) (*typst.Result, error) {
	c.compiled = true
	if c.read != nil {
		c.read(p)
	}
	return &typst.Result{Diagnostics: []typst.Diagnostic{{File: "main.typ", Line: 1, Column: 2, Message: "unknown variable: x", Severity: typst.Error}}}, nil
}

func TestCompilerDiagnosticSourceIdentity(t *testing.T) {
	for _, changed := range []bool{false, true} {
		compiler := &diagnosticCompiler{}
		p := NewPreview(PreviewOptions{Root: t.TempDir(), Engine: compiler, Snapshot: func(context.Context) (SourceSnapshot, error) {
			return SourceSnapshot{Root: t.TempDir(), Hashes: map[string]string{"main.typ": "original"}, Current: func(context.Context) bool { return !changed }, Cleanup: func() {}}, nil
		}})
		p.compile(t.Context(), compilation{})
		var msg PreviewMessage
		if err := json.Unmarshal(p.last, &msg); err != nil {
			t.Fatal(err)
		}
		p.Close()
		if changed && len(msg.SourceHashes) != 0 {
			t.Fatal("a changed projection was labelled fresh")
		}
		if !changed && msg.SourceHashes["main.typ"] != "original" {
			t.Fatal("stable source identity missing")
		}
	}
}

func TestCompilerReadsImmutableSourcesAcrossABA(t *testing.T) {
	base, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = base.Close() }()
	live, err := project.Create(base, "Snapshot", "automerge:abc")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = live.Close() }()
	path := filepath.Join(live.Path(), "main.typ")
	a := []byte("#knownA")
	if err := os.WriteFile(path, a, 0o600); err != nil {
		t.Fatal(err)
	}
	compiler := &diagnosticCompiler{read: func(input typst.Project) {
		if err := os.WriteFile(path, []byte("#unknownB"), 0o600); err != nil {
			t.Fatal(err)
		}
		compiled, err := os.ReadFile(filepath.Join(input.Root, "main.typ"))
		if err != nil {
			t.Fatal(err)
		}
		if string(compiled) != string(a) {
			t.Fatalf("compiler observed live B instead of immutable A: %s", compiled)
		}
		if err := os.WriteFile(path, a, 0o600); err != nil {
			t.Fatal(err)
		}
	}}
	p := NewPreview(PreviewOptions{Root: live.Path(), Engine: compiler, Snapshot: func(ctx context.Context) (SourceSnapshot, error) { return snapshotProject(ctx, live) }})
	defer p.Close()
	p.compile(t.Context(), compilation{})
	var msg PreviewMessage
	if err := json.Unmarshal(p.last, &msg); err != nil {
		t.Fatal(err)
	}
	if len(msg.SourceHashes["main.typ"]) != 64 {
		t.Fatal("actual immutable source identity missing")
	}
}

func TestSnapshotFailureKeepsPreviewWithoutDiagnosticIdentity(t *testing.T) {
	root := t.TempDir()
	compiler := &diagnosticCompiler{read: func(input typst.Project) {
		if input.Root != root {
			t.Fatal("fallback compiled wrong project")
		}
	}}
	p := NewPreview(PreviewOptions{Root: root, Engine: compiler, Snapshot: func(context.Context) (SourceSnapshot, error) { return SourceSnapshot{}, errors.New("copy unavailable") }})
	defer p.Close()
	p.compile(t.Context(), compilation{})
	var msg PreviewMessage
	if err := json.Unmarshal(p.last, &msg); err != nil {
		t.Fatal(err)
	}
	if !compiler.compiled || len(msg.Diagnostics) != 1 || len(msg.SourceHashes) != 0 {
		t.Fatal("fallback lost preview or labelled live sources as immutable")
	}
}
