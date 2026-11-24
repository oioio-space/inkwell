package app_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oioio-space/inkwell/internal/app"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/store"
)

const tinyPNG = "\x89PNG\r\n\x1a\n"

func imageServer(t *testing.T) (http.Handler, string) {
	t.Helper()
	h, token, _ := imageServerIn(t)
	return h, token
}

// imageServerIn is [imageServer] with the project folder, for tests that add files.
func imageServerIn(t *testing.T) (http.Handler, string, string) {
	t.Helper()
	dir := t.TempDir()
	base, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = base.Close() })
	proj, err := project.Create(base, "p", "automerge:root")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = proj.Close() })
	for rel, body := range map[string]string{"pic.png": tinyPNG, "notes.txt": "hello", "data.bin": "xx", "blob.dat": "a\x00b", "pic.svg": "<svg onload=alert(1)/>", "pic.bmp": "BM", "pic.webp": "RIFF", "PIC.JPEG": "\xff\xd8"} {
		if err := os.WriteFile(filepath.Join(dir, "p", rel), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	token := app.NewAssetToken()
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	return app.ProjectImages(proj, token, next), token, filepath.Join(dir, "p")
}

func get(t *testing.T, h http.Handler, route, rel, token string, withToken bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, route+"?path="+url.QueryEscape(rel), nil)
	if withToken {
		req.Header.Set(app.TokenHeader, token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestProjectImagesRequireToken(t *testing.T) {
	h, token := imageServer(t)
	for _, route := range []string{"/project-image", "/project-text"} {
		rel := map[string]string{"/project-image": "pic.png", "/project-text": "notes.txt"}[route]
		if got := get(t, h, route, rel, "", false).Code; got != http.StatusForbidden {
			t.Errorf("%s without token = %d, want 403", route, got)
		}
		if got := get(t, h, route, rel, "wrong", true).Code; got != http.StatusForbidden {
			t.Errorf("%s with wrong token = %d, want 403", route, got)
		}
		if got := get(t, h, route, rel, token, true).Code; got != http.StatusOK {
			t.Errorf("%s with the token = %d, want 200", route, got)
		}
	}
}

func TestProjectImagesServeInertDataURL(t *testing.T) {
	h, token := imageServer(t)
	rec := get(t, h, "/project-image", "pic.png", token, true)
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain (inert)", got)
	}
	if got := rec.Body.String(); !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Errorf("body = %q, want a png data URL", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	// A fixed table: the system MIME database may not know .bmp or .webp.
	for rel, want := range map[string]string{"pic.bmp": "image/bmp", "pic.webp": "image/webp", "PIC.JPEG": "image/jpeg", "pic.svg": "image/svg+xml"} {
		if got := get(t, h, "/project-image", rel, token, true).Body.String(); !strings.HasPrefix(got, "data:"+want+";base64,") {
			t.Errorf("/project-image %s = %q, want a %s data URL", rel, got, want)
		}
	}
	svg := get(t, h, "/project-image", "pic.svg", token, true)
	if !strings.HasPrefix(svg.Header().Get("Content-Type"), "text/plain") {
		t.Errorf("svg served as %q, want text/plain", svg.Header().Get("Content-Type"))
	}
}

func TestProjectImagesRefuse(t *testing.T) {
	h, token := imageServer(t)
	for _, tc := range []struct{ name, route, rel string }{
		{"traversal", "/project-text", "../secret.txt"},
		{"deep traversal", "/project-image", "a/../../pic.png"},
		{"absolute", "/project-text", "/etc/passwd"},
		{"backslash", "/project-text", `..\secret.txt`},
		{"not an image", "/project-image", "data.bin"},
		{"text as image", "/project-image", "notes.txt"},
		{"missing", "/project-text", "absent.txt"},
		{"empty", "/project-text", ""},
	} {
		if got := get(t, h, tc.route, tc.rel, token, true).Code; got != http.StatusNotFound {
			t.Errorf("%s: %s %q = %d, want 404", tc.name, tc.route, tc.rel, got)
		}
	}
}

// A listed file whose bytes are not text is told apart from a missing one: the
// webview learns the content class when it reads the file (ADR 0027 rule 6).
func TestProjectTextReportsBinaryContent(t *testing.T) {
	h, token := imageServer(t)
	if got := get(t, h, "/project-text", "blob.dat", token, true).Code; got != http.StatusUnsupportedMediaType {
		t.Errorf("/project-text of binary content = %d, want 415", got)
	}
	if got := get(t, h, "/project-text", "absent.dat", token, true).Code; got != http.StatusNotFound {
		t.Errorf("/project-text of a missing file = %d, want 404", got)
	}
}

// Bytes that are not text are told so whatever the size, from their start; a
// text over what a document holds gets its own status, not « missing » (EX0
// code review, finding 1).
func TestProjectTextOfBigFiles(t *testing.T) {
	h, token, dir := imageServerIn(t)
	big := int64(store.MaxValue) + 1<<20
	// A sparse file: zeros, so binary from its first byte, and nothing written past it.
	if err := os.WriteFile(filepath.Join(dir, "archive.zip"), []byte("PK\x03\x04\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(filepath.Join(dir, "archive.zip"), big); err != nil {
		t.Fatal(err)
	}
	if got := get(t, h, "/project-text", "archive.zip", token, true).Code; got != http.StatusUnsupportedMediaType {
		t.Errorf("/project-text of a %d-byte binary file = %d, want 415", big, got)
	}
	// Text for its first 4 KiB, over the limit in all.
	if err := os.WriteFile(filepath.Join(dir, "huge.log"), bytes.Repeat([]byte("line of text\n"), 400), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(filepath.Join(dir, "huge.log"), big); err != nil {
		t.Fatal(err)
	}
	if got := get(t, h, "/project-text", "huge.log", token, true).Code; got != http.StatusRequestEntityTooLarge {
		t.Errorf("/project-text of a %d-byte text = %d, want 413", big, got)
	}
}

func TestProjectImagesMethodAndPassThrough(t *testing.T) {
	h, token := imageServer(t)
	post := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/project-text?path=notes.txt", nil)
	post.Header.Set(app.TokenHeader, token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, post)
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST = %d, want 403 or 405", rec.Code)
	}
	if got := get(t, h, "/other", "", "", false).Code; got != http.StatusTeapot {
		t.Errorf("other path = %d, want pass-through", got)
	}
}

// HEAD answers whether the open file changed (ETag) or is gone (404), without
// reading it, so the webview can follow edits made outside Inkwell.
func TestProjectFilesHeadReportsVersion(t *testing.T) {
	h, token := imageServer(t)
	head := func(route, rel string, withToken bool) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodHead, route+"?path="+url.QueryEscape(rel), nil)
		if withToken {
			req.Header.Set(app.TokenHeader, token)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	for route, rel := range map[string]string{"/project-text": "notes.txt", "/project-image": "pic.png"} {
		rec := head(route, rel, true)
		if rec.Code != http.StatusOK || rec.Header().Get("ETag") == "" || rec.Body.Len() != 0 {
			t.Errorf("HEAD %s %s = %d, ETag %q, body %d bytes; want 200, an ETag, no body", route, rel, rec.Code, rec.Header().Get("ETag"), rec.Body.Len())
		}
		if got := head(route, rel, false).Code; got != http.StatusForbidden {
			t.Errorf("HEAD %s without token = %d, want 403", route, got)
		}
		if got := head(route, "gone.txt", true).Code; got != http.StatusNotFound {
			t.Errorf("HEAD %s of a missing file = %d, want 404", route, got)
		}
	}
}

func TestPageTokenFillsPlaceholderOnPagesOnly(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".js") {
			w.Header().Set("Content-Type", "text/javascript")
		} else {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		_, _ = w.Write([]byte(`<meta name="asset-token" content="__ASSET_TOKEN__">`))
	})
	h := app.PageToken("tok123")(next)
	if got := serve(t, h, "/").Body.String(); !strings.Contains(got, `content="tok123"`) {
		t.Errorf("page = %q, want the token", got)
	}
	if got := serve(t, h, "/app.js").Body.String(); !strings.Contains(got, "__ASSET_TOKEN__") {
		t.Errorf("script was rewritten: %q", got)
	}
	a, b := app.NewAssetToken(), app.NewAssetToken()
	if a == b || len(a) < 40 {
		t.Errorf("tokens %q %q are not distinct and strong", a, b)
	}
}
