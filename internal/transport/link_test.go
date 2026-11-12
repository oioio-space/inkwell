package transport_test

import (
	"testing"

	"github.com/oioio-space/inkwell/internal/transport"
)

// TestLinkNames pins the names the webview and the logs read.
func TestLinkNames(t *testing.T) {
	names := []struct{ got, want string }{
		{transport.LinkLAN.String(), "LinkLAN"},
		{transport.LinkDirect.String(), "LinkDirect"},
		{transport.LinkRelayed.String(), "LinkRelayed"},
		{transport.LinkKind(0).String(), "LinkKind(?)"},
		{transport.LossPath.String(), "path"},
		{transport.LossFirstHop.String(), "firstHop"},
		{transport.LossScope(0).String(), ""},
		{transport.QualityGood.String(), "good"},
		{transport.QualityFair.String(), "fair"},
		{transport.QualityPoor.String(), "poor"},
		{transport.QualityUnknown.String(), "unknown"},
	}
	for _, n := range names {
		if n.got != n.want {
			t.Errorf("name %q, want %q", n.got, n.want)
		}
	}
}
