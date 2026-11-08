package language

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestClientPolicy(t *testing.T) {
	s := New(t.TempDir())
	for _, method := range []string{"workspace/executeCommand", "unknown", "workspace/didChangeConfiguration"} {
		if _, err := s.client([]byte(`{"jsonrpc":"2.0","id":1,"method":"` + method + `","params":{}}`)); err == nil {
			t.Errorf("accepted %s", method)
		}
	}
	for _, uri := range []string{"file:///../secret.typ", "file://remote/main.typ", "https://example.org/file.typ", "file:///%2e%2e/secret.typ"} {
		b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": uri}}})
		if _, err := s.client(b); err == nil {
			t.Errorf("accepted %s", uri)
		}
	}
	b, err := s.client([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"rootUri":"file:///etc","initializationOptions":{"command":"bad"}}}`))
	if err != nil || strings.Contains(string(b), "bad") || !strings.Contains(string(b), s.root) {
		t.Fatalf("initialize %s %v", b, err)
	}
}

func TestURITextIsOpaque(t *testing.T) {
	s := New(t.TempDir())
	b, err := s.client([]byte(`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///main.typ","text":"file:///etc/passwd"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	doc := m["params"].(map[string]any)["textDocument"].(map[string]any)
	if doc["text"] != "file:///etc/passwd" {
		t.Fatal("rewrote document text")
	}
	if doc["uri"] != "file://"+s.root+"/main.typ" {
		t.Fatalf("uri = %v", doc["uri"])
	}
}

func TestProtocolBoundsAndOutboundPolicy(t *testing.T) {
	s := New(t.TempDir())
	for _, raw := range []string{
		`{"jsonrpc":"2.0","method":"workspace/executeCommand","params":{"command":"rm"}}`,
		`{"jsonrpc":"2.0","method":"workspace/applyEdit","id":12,"params":{}}`,
		`{"jsonrpc":"2.0","method":"textDocument/publishDiagnostics","params":{"uri":"file:///etc/passwd","diagnostics":[]}}`,
	} {
		if _, err := s.server([]byte(raw)); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	for _, header := range []string{"Content-Length: 4194305\r\n\r\n", "Content-Length: -1\r\n\r\n", "Content-Length: 1\r\nContent-Length: 1\r\n\r\nx", "No-Length: 1\r\n\r\nx"} {
		if _, err := readFrame(bufio.NewReader(strings.NewReader(header))); err == nil {
			t.Error("accepted invalid framing")
		}
	}
	for _, mode := range []string{"auto", "enable"} {
		if _, err := s.client([]byte(`{"jsonrpc":"2.0","method":"workspace/didChangeConfiguration","params":{"settings":{"syntaxOnly":"` + mode + `"}}}`)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.client([]byte(`{"jsonrpc":"2.0","method":"workspace/didChangeConfiguration","params":{"settings":{"syntaxOnly":"enable","rootPath":"/etc"}}}`)); err == nil {
		t.Fatal("accepted arbitrary configuration")
	}
}

func FuzzFraming(f *testing.F) {
	f.Add([]byte("Content-Length: 2\r\n\r\n{}"))
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = readFrame(bufio.NewReader(bytes.NewReader(b))) })
}

func TestOnlyFrontendLanguageMethodsAreRelayed(t *testing.T) {
	s := New(t.TempDir())
	for _, method := range []string{"textDocument/rename", "textDocument/codeAction", "textDocument/documentLink", "textDocument/references", "workspace/symbol", "textDocument/definition", "completionItem/resolve", "textDocument/diagnostic", "textDocument/formatting", "textDocument/documentSymbol"} {
		b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": map[string]any{}})
		if _, err := s.client(b); err == nil {
			t.Errorf("unused method %s relayed", method)
		}
	}
}

func TestEveryServerRepresentationHidesAbsoluteProjectRoot(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "project with é"))
	rootURI := (&url.URL{Scheme: "file", Path: s.root}).String()
	cases := []any{
		map[string]any{"result": map[string]any{"changes": map[string]any{rootURI + "/main.typ": []any{}}}},
		map[string]any{"result": []any{map[string]any{"target": rootURI + "/main.typ"}}},
		map[string]any{"error": map[string]any{"message": "cannot read " + s.root + "/main.typ"}},
		map[string]any{"result": map[string]any{"contents": "[source](" + rootURI + "/main.typ)"}},
	}
	for _, value := range cases {
		b, _ := json.Marshal(value)
		safe, err := s.server(b)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(safe), s.root) || strings.Contains(string(safe), rootURI) {
			t.Fatalf("absolute project root escaped in %s", safe)
		}
	}
}

func TestFrontendCancellationIsRelayedWithStrictNumericID(t *testing.T) {
	s := New(t.TempDir())
	good := []byte(`{"jsonrpc":"2.0","method":"$/cancelRequest","params":{"id":7}}`)
	safe, err := s.client(good)
	if err != nil {
		t.Fatal("valid cancellation refused", err)
	}
	var decoded struct {
		Params struct {
			ID int `json:"id"`
		} `json:"params"`
	}
	if json.Unmarshal(safe, &decoded) != nil || decoded.Params.ID != 7 {
		t.Fatalf("cancellation not relayed: %s", safe)
	}
	for _, params := range []string{`{"id":"7"}`, `{"id":0}`, `{"id":1.5}`, `{"id":7,"command":"bad"}`} {
		if _, err := s.client([]byte(`{"jsonrpc":"2.0","method":"$/cancelRequest","params":` + params + `}`)); err == nil {
			t.Errorf("invalid cancellation accepted: %s", params)
		}
	}
}
