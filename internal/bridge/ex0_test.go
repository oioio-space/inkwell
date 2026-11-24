package bridge_test

import (
	"testing"

	"github.com/oioio-space/inkwell/internal/bridge"
	"github.com/oioio-space/inkwell/internal/project"
)

// TestEX0FileListV2: the file list payload is version 2, without the kind byte
// (ADR 0027 rule 10). The bytes of the shared vector are checked by the vector
// test of storestream_test.go.
func TestEX0FileListV2(t *testing.T) {
	got, err := bridge.EncodeFileList([]project.File{{Path: "main.typ", Size: 4, Main: true}})
	if err != nil {
		t.Fatalf("R-9 file list v2: EncodeFileList: %v", err)
	}
	if len(got) == 0 || got[0] != 2 {
		t.Errorf("R-9 file list v2: version byte of %x is not 2", got)
	}
}
