package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/app"
	"github.com/oioio-space/inkwell/internal/typst"
)

// syncLog is a log destination the preview's goroutines may write concurrently.
type syncLog struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *syncLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *syncLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// ADR 0042 A7: a canary in the compiler's output (its diagnostics and error),
// the project path and the document reaches neither the preview frame, whose
// failure is exactly {"kind":"blocked"}, nor the app's log.
func TestBlockedFailureLeaksNothing(t *testing.T) {
	const canary = "CANARY-5b1b"
	root := filepath.Join(t.TempDir(), canary)
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	main := canary + ".typ"
	if err := os.WriteFile(filepath.Join(root, main), []byte("#let x = \""+canary+"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fc := &fakeCompiler{compile: func(context.Context, int) (*typst.Result, error) {
		res := &typst.Result{Diagnostics: []typst.Diagnostic{{Severity: typst.Error, File: main, Line: 1, Message: canary}}, DiagnosticsTruncated: true}
		return res, fmt.Errorf("typst: %s in %s: %w", canary, root, typst.ErrBlocked)
	}}
	var logs syncLog
	p := app.NewPreview(app.PreviewOptions{Engine: fc, Root: root, Main: main, Debounce: 10 * time.Millisecond, Log: slog.New(slog.NewTextHandler(&logs, nil))})
	c := attach(t, p)
	c.ask(t, `{"op":"view","first":1,"last":3}`)
	var frame []byte
	for timeout := time.After(60 * time.Second); frame == nil; {
		select {
		case b := <-c.sent:
			var m app.PreviewMessage
			if err := json.Unmarshal(b, &m); err == nil && m.Type == "result" {
				frame = b
			}
		case <-timeout:
			t.Fatal("no result frame")
		}
	}
	p.Close()
	if bytes.Contains(frame, []byte(canary)) {
		t.Errorf("the preview frame carries the canary: %s", frame)
	}
	var m struct {
		Failure     json.RawMessage `json:"failure"`
		Diagnostics []any           `json:"diagnostics"`
	}
	if err := json.Unmarshal(frame, &m); err != nil {
		t.Fatal(err)
	}
	if string(m.Failure) != `{"kind":"blocked"}` || len(m.Diagnostics) != 0 {
		t.Errorf("failure %s with %d diagnostics, want exactly {\"kind\":\"blocked\"} and none", m.Failure, len(m.Diagnostics))
	}
	if bytes.Contains([]byte(logs.String()), []byte(canary)) {
		t.Errorf("the log carries the canary:\n%s", logs.String())
	}
}
