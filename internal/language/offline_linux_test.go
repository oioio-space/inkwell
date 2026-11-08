package language

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/sidecar"
)

func TestOfflineLanguage(t *testing.T) {
	isolated, err := sidecar.NetworkIsolation()
	if err != nil || !isolated {
		t.Fatalf("empty network namespace unavailable: %v", err)
	}
	ctx, stop := context.WithCancel(t.Context())
	c := &testConn{ctx: ctx, input: make(chan []byte, 16), output: make(chan []byte, 128)}
	service := New(t.TempDir())
	done := make(chan struct{})
	go func() { defer close(done); service.Serve(c) }()
	defer func() { stop(); <-done }()
	c.send(t, "initialize", 1, map[string]any{"capabilities": map[string]any{}, "rootUri": nil})
	initialized := c.reply(t, 1)
	t.Logf("capabilities: %+v", initialized["result"])
	c.send(t, "initialized", 0, map[string]any{})
	text := `#import "@preview/cmarker:0.1.10": render
#unknown_inkwell_test`
	c.send(t, "textDocument/didOpen", 0, map[string]any{"textDocument": map[string]any{"uri": "file:///main.typ", "languageId": "typst", "version": 7, "text": text}})
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	for {
		select {
		case b := <-c.output:
			var m struct {
				Method string `json:"method"`
				Params struct {
					URI         string `json:"uri"`
					Version     *int   `json:"version"`
					Diagnostics []struct {
						Message string `json:"message"`
					} `json:"diagnostics"`
				} `json:"params"`
			}
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			if m.Method == "inkwell/unavailable" {
				t.Fatal("offline language sidecar stopped")
			}
			if m.Method == "textDocument/publishDiagnostics" && len(m.Params.Diagnostics) > 0 {
				if m.Params.URI != "file:///main.typ" {
					t.Fatalf("wrong document %q", m.Params.URI)
				}
				t.Logf("diagnostic version: %v messages: %+v", m.Params.Version, m.Params.Diagnostics)
				if m.Params.Version != nil {
					t.Fatalf("pinned-server versionless compatibility assumption changed: %s", b)
				}
				if m.Params.Diagnostics[0].Message != "unknown variable: unknown_inkwell_test" {
					t.Fatalf("bundled package failed or wrong diagnostic: %s", b)
				}
				return
			}
		case <-timer.C:
			t.Fatal("no offline diagnostic")
		}
	}
}
