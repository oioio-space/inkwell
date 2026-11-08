package nativeperf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTargetRequiresMeasuredX11PIDXIDAndNominalRefresh(t *testing.T) {
	file := filepath.Join(t.TempDir(), "target.json")
	for _, body := range []string{`{"pid":1,"xid":100,"backend":"wayland","refreshHz":60}`, `{"pid":2,"xid":100,"backend":"x11","refreshHz":60}`, `{"pid":1,"xid":0,"backend":"x11","refreshHz":60}`, `{"pid":1,"xid":100,"backend":"x11","refreshHz":0}`} {
		if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadTarget(file, 1); err == nil {
			t.Fatal("unverified injection target accepted", body)
		}
	}
	if err := os.WriteFile(file, []byte(`{"pid":1,"xid":100,"backend":"x11","refreshHz":59.95}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadTarget(file, 1); err != nil || got.RefreshHz != 59.95 {
		t.Fatal(got, err)
	}
}
