package app

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"maps"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
)

// noncePlaceholder is written in frontend/index.html wherever the CSP nonce goes.
const noncePlaceholder = "__CSP_NONCE__"

// CSPNonce returns asset-server middleware that replaces the nonce placeholder
// in HTML pages with a random value drawn once per launch. The page's CSP then
// allows exactly the <style> elements CodeMirror creates with that nonce
// (EditorView.cspNonce) instead of falling back to 'unsafe-inline'. Other
// responses pass through untouched.
func CSPNonce() func(http.Handler) http.Handler {
	raw := make([]byte, 18)
	_, _ = rand.Read(raw) // crypto/rand.Read never fails on supported platforms (Go ≥ 1.24)
	nonce := []byte(base64.StdEncoding.EncodeToString(raw))
	placeholder := []byte(noncePlaceholder)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only pages are rewritten; everything else (assets, runtime calls,
			// streams) is served directly, never buffered.
			if r.Method != http.MethodGet || !isPage(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, r)
			body := rec.Body.Bytes()
			if strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
				body = bytes.ReplaceAll(body, placeholder, nonce)
				rec.Header().Set("Content-Length", strconv.Itoa(len(body)))
			}
			maps.Copy(w.Header(), rec.Header())
			w.WriteHeader(rec.Code)
			_, _ = w.Write(body)
		})
	}
}

func isPage(path string) bool {
	return path == "/" || strings.HasSuffix(path, ".html")
}
