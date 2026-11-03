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
	return pageRewrite(noncePlaceholder, base64.StdEncoding.EncodeToString(raw))
}

// pageRewrite returns asset-server middleware that replaces placeholder with
// value in HTML pages. Only pages are buffered and rewritten; everything else
// (assets, runtime calls, streams) is served directly.
func pageRewrite(placeholder, value string) func(http.Handler) http.Handler {
	from, to := []byte(placeholder), []byte(value)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || !isPage(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, r)
			body := rec.Body.Bytes()
			if strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
				body = bytes.ReplaceAll(body, from, to)
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
