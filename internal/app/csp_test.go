package app_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/oioio-space/inkwell/internal/app"
)

const page = `<meta http-equiv="Content-Security-Policy" content="style-src 'self' 'nonce-__CSP_NONCE__'"><meta name="csp-nonce" content="__CSP_NONCE__">`

func serve(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	return rec
}

func TestCSPNonceFillsIndexHTML(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Length", "999")
		_, _ = w.Write([]byte(page))
	})
	h := app.CSPNonce()(next)

	first := serve(t, h, "/").Body.String()
	if strings.Contains(first, "__CSP_NONCE__") {
		t.Fatalf("placeholder left in page: %s", first)
	}
	nonces := regexp.MustCompile(`nonce-([A-Za-z0-9+/=]+)'`).FindStringSubmatch(first)
	if len(nonces) != 2 || len(nonces[1]) < 22 {
		t.Fatalf("no strong nonce in page: %s", first)
	}
	if !strings.Contains(first, `content="`+nonces[1]+`"`) {
		t.Errorf("meta csp-nonce does not carry the CSP nonce: %s", first)
	}
	if again := serve(t, h, "/index.html").Body.String(); again != first {
		t.Errorf("nonce changed within one launch:\n%s\n%s", first, again)
	}
	if other := serve(t, app.CSPNonce()(next), "/").Body.String(); other == first {
		t.Error("two launches produced the same nonce")
	}
}

func TestCSPNonceLeavesOtherAssetsAlone(t *testing.T) {
	body := "body{color:red} /* __CSP_NONCE__ */"
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		_, _ = w.Write([]byte(body))
	})
	if got := serve(t, app.CSPNonce()(next), "/assets/x.css").Body.String(); got != body {
		t.Errorf("non-HTML asset modified: %q", got)
	}
	// A page path served with a non-HTML type is left alone too.
	if got := serve(t, app.CSPNonce()(next), "/").Body.String(); got != body {
		t.Errorf("non-HTML asset modified: %q", got)
	}
}
