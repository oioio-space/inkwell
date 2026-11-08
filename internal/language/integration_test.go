package language

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

type testConn struct {
	ctx    context.Context
	cancel context.CancelFunc
	input  chan []byte
	output chan []byte
}

func (c *testConn) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

func (c *testConn) Context() context.Context { return c.ctx }
func (c *testConn) Send(b []byte) error {
	select {
	case c.output <- b:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

func (c *testConn) Receive() ([]byte, error) {
	select {
	case b := <-c.input:
		return b, nil
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
}

func (c *testConn) send(t testing.TB, method string, id int, params any) {
	t.Helper()
	m := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if id > 0 {
		m["id"] = id
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case c.input <- b:
	case <-c.ctx.Done():
		t.Fatal("stream stopped")
	}
}

func (c *testConn) reply(t testing.TB, id int) map[string]any {
	t.Helper()
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case b := <-c.output:
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			if m["method"] == "inkwell/unavailable" {
				t.Fatal("confined tinymist unavailable")
			}
			if m["id"] == float64(id) {
				if m["error"] != nil {
					t.Fatalf("request %d: %s", id, b)
				}
				return m
			}
		case <-timeout.C:
			t.Fatalf("no reply %d", id)
		}
	}
}

// Empty protocol envelopes must never be counted as useful language responses.
func TestPerformanceResultsRejectEmpty(t *testing.T) {
	for _, method := range []string{"textDocument/completion", "textDocument/signatureHelp", "textDocument/hover"} {
		for _, result := range []any{nil, map[string]any{}, []any{}, map[string]any{"items": []any{}}, map[string]any{"signatures": []any{}}, map[string]any{"contents": ""}} {
			if usefulPerformanceResult(method, result) {
				t.Fatalf("%s accepted empty result %#v", method, result)
			}
		}
	}
}

func usefulPerformanceResult(method string, result any) bool {
	value, _ := result.(map[string]any)
	switch method {
	case "textDocument/completion":
		items, ok := result.([]any)
		if !ok {
			items, _ = value["items"].([]any)
		}
		for _, item := range items {
			entry, _ := item.(map[string]any)
			label, _ := entry["label"].(string)
			if strings.Contains(label, "text") {
				return true
			}
		}
	case "textDocument/signatureHelp":
		signatures, _ := value["signatures"].([]any)
		for _, signature := range signatures {
			entry, _ := signature.(map[string]any)
			label, _ := entry["label"].(string)
			if strings.Contains(label, "l3add") {
				return true
			}
		}
	case "textDocument/hover":
		contents, _ := json.Marshal(value["contents"])
		return strings.Contains(string(contents), "Adds two numbers")
	}
	return false
}

// BenchmarkLanguageLatency measures 40 edit/request cycles in a multi-file manuscript.
// Every response must contain useful fixture-specific content; no response is retried.
func BenchmarkLanguageLatency(t *testing.B) {
	root := t.TempDir()
	fixture := filepath.Join("..", "..", "testdata", "perf", "lsp")
	sources := map[string]string{}
	totalLines := 0
	for _, name := range []string{"main.typ", "charter.typ", "chapter-one.typ", "chapter-two.typ"} {
		data, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		sources[name] = string(data)
		totalLines += strings.Count(string(data), "\n")
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if totalLines < 3000 {
		t.Fatalf("performance fixture shrank to %d lines", totalLines)
	}
	text := sources["main.typ"]
	ctx, stop := context.WithCancel(t.Context())
	c := &testConn{ctx: ctx, input: make(chan []byte, 16), output: make(chan []byte, 128)}
	service := New(root)
	done := make(chan struct{})
	go func() { defer close(done); service.Serve(c) }()
	defer func() { stop(); <-done }()
	c.send(t, "initialize", 1, map[string]any{"capabilities": map[string]any{}, "rootUri": nil})
	c.reply(t, 1)
	c.send(t, "initialized", 0, map[string]any{})
	c.send(t, "textDocument/didOpen", 0, map[string]any{"textDocument": map[string]any{"uri": "file:///main.typ", "languageId": "typst", "version": 1, "text": text}})
	type measure struct {
		Samples   []float64        `json:"samples_ms"`
		Revisions []int            `json:"revisions"`
		Positions []map[string]int `json:"positions"`
		P95       float64          `json:"p95_ms"`
	}
	methods := []struct {
		name, method    string
		line, character int
	}{
		{"completion", "textDocument/completion", 4, 4},
		{"signature", "textDocument/signatureHelp", 3, 10},
		{"hover", "textDocument/hover", 3, 3},
	}
	measures := map[string]*measure{}
	for _, req := range methods {
		measures[req.name] = &measure{}
	}
	report := map[string]any{"id": "PERF-LSP", "runner": "pinned tinymist over Wails-compatible Stream, stdio sandbox", "status": "fail", "threshold_ms": 100, "fixture_files": 4, "fixture_lines": totalLines, "edit_cycles": 40, "scope": "40 incremental expression edits at the imported charter call; alternating completion-prefix and argument positions, varying hover positions; no measured request retries"}
	for name, m := range measures {
		report[name] = m
	}
	writeReport := func() {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Error(err)
			return
		}
		if output := os.Getenv("INKWELL_PERF_OUT"); output != "" {
			if err := os.WriteFile(filepath.Join(output, "PERF-LSP.json"), append(data, '\n'), 0o600); err != nil {
				t.Error(err)
			}
		} else {
			fmt.Println(string(data))
		}
	}
	defer writeReport()
	id := 10
	for cycle := range 40 {
		lines := strings.Split(text, "\n")
		oldCall := lines[3]
		edit := fmt.Sprintf("#l3add(1, %d)", cycle+3)
		lines[3] = edit
		text = strings.Join(lines, "\n")
		startPosition := map[string]int{"line": 3, "character": 0}
		endPosition := map[string]int{"line": 3, "character": len(oldCall)}
		version := cycle + 2
		c.send(t, "textDocument/didChange", 0, map[string]any{"textDocument": map[string]any{"uri": "file:///main.typ", "version": version}, "contentChanges": []any{map[string]any{"range": map[string]any{"start": startPosition, "end": endPosition}, "text": edit}}})
		for _, req := range methods {
			id++
			character := req.character
			switch req.name {
			case "completion":
				character = 3 + cycle%2
			case "signature":
				if cycle%2 == 0 {
					character = 7
				}
			case "hover":
				character = 2 + cycle%4
			}
			params := map[string]any{"textDocument": map[string]any{"uri": "file:///main.typ"}, "position": map[string]int{"line": req.line, "character": character}}
			start := time.Now()
			c.send(t, req.method, id, params)
			reply := c.reply(t, id)
			elapsed := float64(time.Since(start)) / float64(time.Millisecond)
			m := measures[req.name]
			m.Samples = append(m.Samples, elapsed)
			m.Revisions = append(m.Revisions, version)
			m.Positions = append(m.Positions, map[string]int{"line": req.line, "character": character})
			if !usefulPerformanceResult(req.method, reply["result"]) {
				report["failed_request"] = map[string]any{"method": req.method, "revision": version, "response": reply, "elapsed_ms": elapsed}
				t.Fatalf("%s revision %d returned no useful result", req.method, version)
			}
		}
	}
	report["status"] = "pass"
	for _, req := range methods {
		m := measures[req.name]
		ordered := append([]float64{}, m.Samples...)
		sort.Float64s(ordered)
		m.P95 = ordered[37]
		if m.P95 > 100 {
			report["status"] = "fail"
			t.Errorf("%s p95 = %.3f > 100ms", req.name, m.P95)
		}
		t.Logf("%s p95 %.3fms", req.name, m.P95)
	}
}

// Closing a window must release its sidecar before a new stream can initialize.
func TestReconnectReleasesServer(t *testing.T) {
	service := New(t.TempDir())
	for range 2 {
		ctx, cancel := context.WithCancel(t.Context())
		c := &testConn{ctx: ctx, input: make(chan []byte, 16), output: make(chan []byte, 128)}
		done := make(chan struct{})
		go func() { defer close(done); service.Serve(c) }()
		c.send(t, "initialize", 1, map[string]any{"capabilities": map[string]any{}})
		c.reply(t, 1)
		c.send(t, "initialized", 0, map[string]any{})
		c.send(t, "textDocument/didOpen", 0, map[string]any{"textDocument": map[string]any{"uri": "file:///main.typ", "languageId": "typst", "version": 1, "text": "#text(1)"}})
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("closed stream did not release language server")
		}
	}
}

// The old virtual stream may survive page destruction without its context ending.
func TestNewStreamSupersedesAbandonedWindow(t *testing.T) {
	service := New(t.TempDir())
	var previous <-chan struct{}
	for range 2 {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		c := &testConn{ctx: ctx, cancel: cancel, input: make(chan []byte, 16), output: make(chan []byte, 128)}
		done := make(chan struct{})
		go func() { defer close(done); service.Serve(c) }()
		c.send(t, "initialize", 1, map[string]any{"capabilities": map[string]any{}})
		c.reply(t, 1)
		if previous != nil {
			select {
			case <-previous:
			case <-time.After(2 * time.Second):
				t.Fatal("old window not released")
			}
		}
		previous = done
	}
}
