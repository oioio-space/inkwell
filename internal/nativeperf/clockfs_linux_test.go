package nativeperf

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestNativeClockAssetsAreFreshFineAndLeaveProductAssetsIntact(t *testing.T) {
	assets := ClockAssets(fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("editor")}})
	if data, err := fs.ReadFile(assets, "index.html"); err != nil || string(data) != "editor" {
		t.Fatal(string(data), err)
	}
	previous := -1.0
	for _, name := range []string{"a/1.json", "a/2.json", "b/1.json"} {
		r := httptest.NewRecorder()
		http.FileServerFS(assets).ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/__native_perf_clock/"+name, nil))
		var stamp struct {
			MS           float64 `json:"ms"`
			ResolutionMS float64 `json:"resolutionMs"`
		}
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &stamp) != nil {
			t.Fatal(r.Code, r.Body.String())
		}
		if stamp.MS <= previous || stamp.ResolutionMS <= 0 || stamp.ResolutionMS > .01 {
			t.Fatal("clock not monotonic/fine", stamp)
		}
		previous = stamp.MS
	}
}
