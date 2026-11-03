//go:build darwin || windows

package typst

import (
	"context"
	"github.com/oioio-space/inkwell/internal/sidecar"
	"strings"
	"testing"
)

func TestUnprovenPlatformRefusesDocuments(t *testing.T) {
	// No document-origin exception: spec 9.11 also refuses personal documents.
	e := &Engine{}
	for _, query := range []bool{false, true} {
		var err error
		if query {
			_, err = e.Query(context.Background(), Project{}, "main.typ", "heading")
		} else {
			_, err = e.Compile(context.Background(), Project{}, "main.typ")
		}
		if err == nil || !strings.Contains(err.Error(), "aperçu indisponible sur ce système pour les documents reçus") {
			t.Fatalf("%v", err)
		}
	}
}

func TestUnprovenPlatformRefusesOtherLaunches(t *testing.T) {
	_, anchorsErr := (&Engine{}).Anchors(context.Background(), Project{}, "main.typ")
	_, runErr := sidecar.Run(context.Background(), sidecar.Spec{})
	supervisorErr := sidecar.NewSupervisor(sidecar.SupervisorConfig{}).Run(context.Background())
	for _, err := range []error{anchorsErr, runErr, supervisorErr} {
		if err == nil || !strings.Contains(err.Error(), "aperçu indisponible sur ce système pour les documents reçus") {
			t.Fatalf("refusal: %v", err)
		}
	}
}
