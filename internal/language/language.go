// Package language relays a bounded, allowlisted language protocol over Wails.
// Documents remain opaque text: all CRDT ownership and revision checks are in the webview.
package language

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/sidecar"
	"github.com/oioio-space/inkwell/internal/typst"
)

// StreamName is the authenticated Wails stream, without any additional port.
const (
	StreamName = "lsp"
	maxFrame   = 4 << 20
)

// Service opens a fresh verified, confined tinymist for each stream generation.
// A generation ends with the stream; responses never cross connections.
type Service struct {
	root     string
	store    *sidecar.Store
	packages string
	fonts    []string
	mu       sync.Mutex
	serveMu  sync.Mutex
	cancel   context.CancelFunc
	redact   *strings.Replacer
}

// New uses only the fixed project directory, never a webview-selected workdir.
func New(root string) *Service {
	root = filepath.Clean(root)
	fileURI := (&url.URL{Scheme: "file", Path: root}).String()
	escaped := (&url.URL{Path: root}).EscapedPath()
	redact := strings.NewReplacer(fileURI+"/", "file:///", fileURI, "file:///", escaped+"/", "", escaped, ".", root+"/", "", root, ".", url.QueryEscape(root), ".", url.PathEscape(root), ".")
	return &Service{root: root, store: &sidecar.Store{}, redact: redact}
}

// Serve relays one JSON object per stream frame and Content-Length on stdio.
func (s *Service) Serve(c bridge.Conn) {
	// A reloaded webview supersedes its old virtual stream even if Wails has
	// not observed the abandoned window's disconnect yet.
	ctx, stop := context.WithCancel(c.Context())
	closeStream := func() {
		stop()
		if closer, ok := c.(io.Closer); ok {
			_ = closer.Close()
		}
	}
	s.mu.Lock()
	previous := s.cancel
	s.cancel = closeStream
	s.mu.Unlock()
	if previous != nil {
		previous()
	}
	defer closeStream()
	// Resource settings and the actual process remain owned by one generation.
	s.serveMu.Lock()
	defer s.serveMu.Unlock()
	if ctx.Err() != nil {
		return
	}
	in, writeIn := io.Pipe()
	out, writeOut := io.Pipe()
	defer func() { _ = in.Close() }()
	defer func() { _ = writeIn.Close() }()
	defer func() { _ = out.Close() }()
	// A disconnected stream may be blocked writing stdin while exec.Wait is
	// waiting for its stdin copier. Unblock every pipe independently of Receive.
	closePipes := context.AfterFunc(ctx, func() {
		_ = in.Close()
		_ = writeIn.Close()
		_ = out.Close()
		_ = writeOut.Close()
	})
	defer closePipes()
	engine, err := typst.New(typst.Config{})
	if err != nil {
		slog.Warn("language resources unavailable", "err", err)
		unavailable(c)
		return
	}
	packages, fonts, readPaths, cleanup, err := engine.LanguagePaths(s.root)
	if err != nil {
		unavailable(c)
		return
	}
	defer cleanup()
	s.packages, s.fonts = packages, fonts
	path, err := s.store.Resolve(sidecar.Tinymist)
	if err != nil {
		slog.Warn("language sidecar unavailable", "err", err)
		unavailable(c)
		return
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = writeOut.Close() }()
		defer func() { _ = in.Close() }()
		result, err := sidecar.Run(ctx, sidecar.Spec{Path: path, Dir: s.root, Args: []string{"lsp"}, Stdin: in, Stdout: writeOut, ReadPaths: readPaths, NoNetwork: true, MemoryLimit: 4 << 30})
		if ctx.Err() == nil {
			slog.Warn("language sidecar stopped", "err", err, "exit", result.ExitCode, "truncated", result.Truncated)
		}
	}()
	go s.relayOutput(c, out, stop)
	defer func() { stop(); _ = writeIn.Close(); _ = out.Close(); <-done }()
	for {
		b, err := c.Receive()
		if err != nil || ctx.Err() != nil {
			return
		}
		safe, err := s.client(b)
		if err != nil {
			reject(c, b)
			continue
		}
		if _, err := fmt.Fprintf(writeIn, "Content-Length: %d\r\n\r\n", len(safe)); err != nil {
			return
		}
		if _, err := writeIn.Write(safe); err != nil {
			return
		}
	}
}

func (s *Service) relayOutput(c bridge.Conn, out io.Reader, stop context.CancelFunc) {
	defer stop()
	if closer, ok := c.(io.Closer); ok {
		defer func() { _ = closer.Close() }()
	}
	r := bufio.NewReader(out)
	for {
		b, err := readFrame(r)
		if err != nil {
			unavailable(c)
			return
		}
		b, err = s.server(b)
		if err != nil {
			slog.Warn("language frame refused", "err", err)
		}
		if err == nil {
			if err := c.Send(b); err != nil {
				return
			}
		}
	}
}

func unavailable(c bridge.Conn) {
	_ = c.Send([]byte(`{"jsonrpc":"2.0","method":"inkwell/unavailable","params":{}}`))
}

func reject(c bridge.Conn, b []byte) {
	var msg struct {
		ID json.RawMessage `json:"id"`
	}
	if json.Unmarshal(b, &msg) != nil || len(msg.ID) == 0 {
		return
	}
	reply, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "error": map[string]any{"code": -32601, "message": "Method or path refused"}})
	_ = c.Send(reply)
}

// Only methods actually used by the shipped editor cross this boundary.
var allowed = map[string]bool{
	"initialize": true, "initialized": true,
	"textDocument/didOpen": true, "textDocument/didChange": true,
	"textDocument/completion": true, "textDocument/signatureHelp": true, "textDocument/hover": true,
}

func (s *Service) client(b []byte) ([]byte, error) {
	if len(b) > maxFrame {
		return nil, errors.New("language: oversized frame")
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	method, _ := m["method"].(string)
	if method == "$/cancelRequest" && m["jsonrpc"] == "2.0" {
		p, ok := m["params"].(map[string]any)
		id, isNumber := p["id"].(float64)
		if !ok || len(p) != 1 || !isNumber || id <= 0 || id > 1<<53-1 || math.Trunc(id) != id {
			return nil, errors.New("language: invalid cancellation")
		}
		return json.Marshal(m)
	}
	if method == "workspace/didChangeConfiguration" && m["jsonrpc"] == "2.0" {
		p, ok := m["params"].(map[string]any)
		if !ok || len(p) != 1 {
			return nil, errors.New("language: configuration refused")
		}
		settings, ok := p["settings"].(map[string]any)
		if !ok || len(settings) != 1 || (settings["syntaxOnly"] != "auto" && settings["syntaxOnly"] != "enable") {
			return nil, errors.New("language: configuration refused")
		}
		p["settings"] = s.settings(settings["syntaxOnly"].(string))
		return json.Marshal(m)
	}
	if m["jsonrpc"] != "2.0" || !allowed[method] {
		return nil, errors.New("language: method refused")
	}
	if method == "initialize" {
		// Pin all settings that could start compilation/export or choose filesystem roots.
		p, ok := m["params"].(map[string]any)
		if !ok {
			return nil, errors.New("language: missing params")
		}
		root := (&url.URL{Scheme: "file", Path: s.root}).String()
		p["rootUri"], p["rootPath"], p["processId"] = root, s.root, nil
		p["workspaceFolders"] = []any{map[string]any{"uri": root, "name": "Project"}}
		p["initializationOptions"] = s.settings("auto")
		return json.Marshal(m)
	}
	if err := s.rewrite(m, true); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

func (s *Service) server(b []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	method, _ := m["method"].(string)
	// A shared document cannot ask the desktop to run commands or edit files.
	if method != "" && method != "textDocument/publishDiagnostics" {
		return nil, errors.New("language: server method refused")
	}
	if err := s.rewrite(m, false); err != nil {
		return nil, err
	}
	return json.Marshal(s.redactOutgoing(m))
}

// rewrite visits only protocol URI fields, never source text, messages or docstrings.
func (s *Service) rewrite(v any, inbound bool) error {
	switch x := v.(type) {
	case map[string]any:
		for k, v := range x {
			if k == "uri" || k == "targetUri" {
				raw, ok := v.(string)
				if !ok {
					return errors.New("language: invalid uri")
				}
				converted, err := s.uri(raw, inbound)
				if err != nil {
					return err
				}
				x[k] = converted
			} else if err := s.rewrite(v, inbound); err != nil {
				return err
			}
		}
	case []any:
		for _, v := range x {
			if err := s.rewrite(v, inbound); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) uri(raw string, inbound bool) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("language: invalid file uri")
	}
	rel := strings.TrimPrefix(u.Path, "/")
	if !inbound {
		rel, err = filepath.Rel(s.root, u.Path)
		if err != nil {
			return "", err
		}
		rel = filepath.ToSlash(rel)
	}
	if err := project.ValidatePath(rel); err != nil {
		return "", err
	}
	if inbound {
		return (&url.URL{Scheme: "file", Path: filepath.Join(s.root, filepath.FromSlash(rel))}).String(), nil
	}
	return (&url.URL{Scheme: "file", Path: "/" + rel}).String(), nil
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	length, total := -1, 0
	for {
		part, err := r.ReadSlice('\n')
		line := string(part)
		if err != nil {
			return nil, err
		}
		total += len(line)
		if total > 8192 {
			return nil, errors.New("language: oversized header")
		}
		if line == "\r\n" || line == "\n" {
			break
		}
		if value, ok := strings.CutPrefix(strings.ToLower(line), "content-length:"); ok {
			if length != -1 {
				return nil, errors.New("language: duplicate length")
			}
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, err
			}
		}
	}
	if length < 0 || length > maxFrame {
		return nil, errors.New("language: invalid length")
	}
	b := make([]byte, length)
	_, err := io.ReadFull(r, b)
	return b, err
}

func (s *Service) settings(mode string) map[string]any {
	return map[string]any{"rootPath": s.root, "exportPdf": "never", "compileStatus": "disable", "typstExtraArgs": []string{"--package-path", filepath.Join(s.root, ".typstdeps"), "--package-cache-path", s.packages}, "fontPaths": s.fonts, "systemFonts": false, "syntaxOnly": mode}
}

// redactOutgoing covers all representations, including URI-valued map keys,
// document-link targets, nested data, error messages and documentation strings.
func (s *Service) redactOutgoing(value any) any {
	switch v := value.(type) {
	case string:
		return s.redact.Replace(v)
	case map[string]any:
		clean := make(map[string]any, len(v))
		for key, item := range v {
			clean[s.redact.Replace(key)] = s.redactOutgoing(item)
		}
		return clean
	case []any:
		for i, item := range v {
			v[i] = s.redactOutgoing(item)
		}
	}
	return value
}
