package app

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/oioio-space/inkwell/internal/project"
)

// tokenPlaceholder is written in frontend/index.html where the asset token goes.
const tokenPlaceholder = "__ASSET_TOKEN__" // #nosec G101 -- a template marker, not a credential; the token is random per launch

// TokenHeader carries the asset token on every /project-* request. A header, not a
// query parameter, so the token never reaches a URL that something might log.
const TokenHeader = "X-Inkwell-Token" // #nosec G101 -- an HTTP header name, not a credential

// NewAssetToken draws the random per-launch token that guards the /project-*
// handlers. Only the pages this process serves carry it (see [PageToken]); another
// origin cannot read them, so it cannot learn the token.
func NewAssetToken() string {
	raw := make([]byte, 32)
	_, _ = rand.Read(raw) // crypto/rand.Read never fails on supported platforms (Go ≥ 1.24)
	return base64.RawURLEncoding.EncodeToString(raw)
}

// PageToken returns asset-server middleware that writes token into HTML pages
// in place of the placeholder, so the webview can send it back. Other responses
// pass through untouched.
func PageToken(token string) func(http.Handler) http.Handler {
	return pageRewrite(tokenPlaceholder, token)
}

// imageTypes is the media type of each image extension the project lists. A
// fixed table, not mime.TypeByExtension, whose answer depends on the system's
// MIME database (it has no .bmp on a bare Linux).
var imageTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif",
	".webp": "image/webp", ".svg": "image/svg+xml", ".bmp": "image/bmp",
}

// tokenOK compares the presented token with the launch token in constant time
// (both hashed first, so the length does not leak either).
func tokenOK(got, want string) bool {
	a, b := sha256.Sum256([]byte(got)), sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}

// ProjectImages serves project files to the webview on /project-text and
// /project-image, both guarded by token in [TokenHeader] (403 otherwise). Images
// are inert data URLs, never active SVG documents. HEAD on either route answers
// the file's [project.Project.Version] as its ETag, or 404 once it is gone.
func ProjectImages(p *project.Project, token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/project-image" && r.URL.Path != "/project-text" {
			next.ServeHTTP(w, r)
			return
		}
		if !tokenOK(r.Header.Get(TokenHeader), token) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		rel := r.URL.Query().Get("path")
		if r.Method == http.MethodHead {
			// The version of the file, read from its metadata only: the webview
			// polls it to follow an open file edited, renamed or deleted outside.
			version, err := p.Version(rel)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("ETag", strconv.Quote(version))
			w.Header().Set("Cache-Control", "no-store")
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path == "/project-text" {
			text, err := p.ReadProjectText(rel)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write([]byte(text.Body))
			return
		}
		data, err := p.ReadImage(rel)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		kind, ok := imageTypes[strings.ToLower(path.Ext(rel))]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte("data:" + kind + ";base64," + base64.StdEncoding.EncodeToString(data))) // #nosec G705 -- base64 text served as text/plain with nosniff; the frontend only uses it as an <img> source
	})
}
