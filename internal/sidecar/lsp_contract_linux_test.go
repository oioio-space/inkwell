package sidecar

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// startLSP runs `binary lsp` in dir: through Run with a copy of spec when it
// is not nil, unrestricted otherwise. It returns the server's input, its
// output and its end; the caller closes the first two.
func startLSP(ctx context.Context, binary, dir string, spec *Spec) (*io.PipeWriter, *io.PipeReader, <-chan error) {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	done := make(chan error, 1)
	go func() {
		var err error
		if spec != nil {
			s := *spec
			s.Stdin, s.Stdout = inR, outW
			var out Output
			out, err = Run(ctx, s)
			if err == nil && out.ExitCode != 0 {
				err = fmt.Errorf("exit %d: %s", out.ExitCode, out.Stderr)
			}
		} else {
			c := exec.CommandContext(ctx, binary, "lsp")
			c.Dir, c.Env, c.Stdin, c.Stdout = dir, []string{}, inR, outW
			err = c.Run()
		}
		_ = inR.CloseWithError(err)
		_ = outW.CloseWithError(err)
		done <- err
	}()
	return inW, outR, done
}

// writeLSP writes one framed message; a pipe write waits for its reader, so
// it gives up when ctx ends.
func writeLSP(ctx context.Context, w io.Writer, body []byte) error {
	written := make(chan error, 1)
	go func() { _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body); written <- err }()
	select {
	case err := <-written:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// readLSP reads one framed message of at most 4 MiB.
func readLSP(r *bufio.Reader) ([]byte, error) {
	n := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(line) == "" {
			break
		}
		if v, ok := strings.CutPrefix(strings.ToLower(line), "content-length:"); ok {
			n, _ = strconv.Atoi(strings.TrimSpace(v))
		}
	}
	if n <= 0 || n > 4<<20 {
		return nil, fmt.Errorf("LSP frame of %d bytes", n)
	}
	data := make([]byte, n)
	_, err := io.ReadFull(r, data)
	return data, err
}

// lspPerimeter compares protocol values, not transport framing or arrival
// order. Notifications can be interleaved with responses by either server.
func lspPerimeter(t *testing.T, binary, dir string, confined bool) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	var confinement *Spec
	if confined {
		confinement = &Spec{Path: binary, Dir: dir, Args: []string{"lsp"}, tier: tierF}
	}
	in, out, done := startLSP(ctx, binary, dir, confinement)
	defer func() { _ = in.Close() }()
	defer func() { _ = out.Close() }()
	messages := make(chan map[string]any, 16)
	go func() {
		defer close(messages)
		r := bufio.NewReader(out)
		for {
			data, err := readLSP(r)
			var m map[string]any
			if err != nil || json.Unmarshal(data, &m) != nil {
				return
			}
			select {
			case messages <- m:
			case <-ctx.Done():
				return
			}
		}
	}()
	send := func(id int, method string, params any) {
		t.Helper()
		m := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
		if id != 0 {
			m["id"] = id
		}
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("S-5 perimeter: encode %s: %v", method, err)
		}
		if err := writeLSP(ctx, in, b); err != nil {
			t.Fatalf("S-5 perimeter: write %s: %v", method, err)
		}
	}
	var queuedDiagnostics []map[string]any
	receive := func(id int) map[string]any {
		t.Helper()
		for {
			select {
			case m, ok := <-messages:
				if !ok {
					t.Fatal("S-5 perimeter: LSP closed before response")
				}
				if v, ok := m["id"].(float64); ok && int(v) == id {
					return m
				}
				if m["method"] == "textDocument/publishDiagnostics" {
					queuedDiagnostics = append(queuedDiagnostics, m)
				}
			case <-ctx.Done():
				t.Fatalf("S-5 perimeter: response %d deadline", id)
			}
		}
	}
	values := map[string]any{}
	send(1, "initialize", map[string]any{"processId": nil, "rootUri": (&url.URL{Scheme: "file", Path: dir}).String(), "capabilities": map[string]any{}})
	initialized := receive(1)
	if initialized["error"] != nil || initialized["result"] == nil {
		t.Fatalf("S-5 perimeter: initialize: %v", initialized)
	}
	sortLSPCommands(initialized["result"])
	values["capabilities"] = initialized["result"]
	send(0, "initialized", map[string]any{})
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(dir, "main.typ")}).String()
	diagnostic := func(nonempty bool) any {
		t.Helper()
		for {
			var m map[string]any
			if len(queuedDiagnostics) > 0 {
				m = queuedDiagnostics[0]
				queuedDiagnostics = queuedDiagnostics[1:]
			} else {
				select {
				case next, ok := <-messages:
					if !ok {
						t.Fatal("S-5 perimeter: LSP closed before diagnostics")
					}
					m = next
				case <-ctx.Done():
					t.Fatal("S-5 perimeter: diagnostics deadline")
				}
			}
			if m["method"] != "textDocument/publishDiagnostics" {
				continue
			}
			p, ok := m["params"].(map[string]any)
			if !ok || p["uri"] != uri {
				continue
			}
			d, ok := p["diagnostics"].([]any)
			if !ok {
				continue
			}
			if (len(d) > 0) == nonempty {
				return d
			}
		}
	}
	send(0, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "typst", "version": 1, "text": "#let x =\n"}})
	values["diagnostics-before"] = diagnostic(true)
	send(0, "textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": uri, "version": 2}, "contentChanges": []any{map[string]any{"text": "#text(\"hello\")\n"}}})
	values["diagnostics-after"] = diagnostic(false)
	for i, method := range []string{"textDocument/completion", "textDocument/signatureHelp", "textDocument/hover"} {
		character := 1
		if i == 1 {
			character = 5
		}
		if i == 2 {
			character = 2
		}
		send(i+2, method, map[string]any{"textDocument": map[string]any{"uri": uri}, "position": map[string]any{"line": 0, "character": character}})
		m := receive(i + 2)
		if m["error"] != nil {
			t.Fatalf("S-5 perimeter: %s: %v", method, m)
		}
		value := m["result"]
		if i == 0 {
			items, ok := value.([]any)
			if !ok {
				if list, ok := value.(map[string]any); ok {
					items, _ = list["items"].([]any)
				}
			}
			var labels []string
			for _, item := range items {
				if entry, ok := item.(map[string]any); ok {
					if label, ok := entry["label"].(string); ok {
						labels = append(labels, label)
					}
				}
			}
			slices.Sort(labels)
			if len(labels) == 0 {
				t.Fatal("S-5 perimeter: no completion labels")
			}
			value = labels
		}
		if value == nil {
			t.Fatalf("S-5 perimeter: %s returned no value", method)
		}
		values[method] = value
	}
	send(8, "textDocument/completion", map[string]any{"textDocument": map[string]any{"uri": uri}, "position": map[string]any{"line": 0, "character": 1}})
	send(0, "$/cancelRequest", map[string]any{"id": 8})
	cancelled := receive(8)
	// LSP permits a result if computation won the cancellation race. Otherwise
	// RequestCancelled (-32800) is required; the next request must still work.
	if e, ok := cancelled["error"].(map[string]any); ok && e["code"] != float64(-32800) {
		t.Fatalf("S-5 perimeter: cancellation: %v", cancelled)
	}
	if cancelled["error"] == nil && cancelled["result"] == nil {
		t.Fatal("S-5 perimeter: cancellation has neither result nor error")
	}
	send(9, "textDocument/hover", map[string]any{"textDocument": map[string]any{"uri": uri}, "position": map[string]any{"line": 0, "character": 2}})
	if m := receive(9); m["error"] != nil || m["result"] == nil {
		t.Fatalf("S-5 perimeter: after cancellation: %v", m)
	}
	send(99, "shutdown", nil)
	if m := receive(99); m["error"] != nil {
		t.Fatalf("S-5 perimeter: shutdown: %v", m)
	}
	send(0, "exit", nil)
	_ = in.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("S-5 perimeter: LSP exit: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("S-5 perimeter: LSP exit deadline")
	}
	b, err := json.Marshal(values)
	if err != nil {
		t.Fatalf("S-5 perimeter: result encoding: %v", err)
	}
	return b
}

func testLSPPerimeter(t *testing.T) {
	if !requireLaunch(t) {
		return
	}
	requireEmbedded(t)
	binary, err := (&Store{Dir: sharedCache}).Resolve(Tinymist)
	if err != nil {
		t.Fatalf("S-5 perimeter: tinymist: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.typ"), []byte("#text(\"hello\")\n"), 0o600); err != nil {
		t.Fatalf("S-5 perimeter: document: %v", err)
	}
	baseline := lspPerimeter(t, binary, dir, false)
	confined := lspPerimeter(t, binary, dir, true)
	if !bytes.Equal(baseline, confined) {
		t.Fatalf("S-5 perimeter: LSP differs: unrestricted %s F %s", baseline, confined)
	}
}
