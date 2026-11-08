package transport_test

import (
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/transport"
)

// TestLinkQualityThresholds proves the grades of decision Q4 (plan W33 §7): a
// link is graded by its worst figure, each threshold included in the worse
// grade, resyncs never graded, and unknown until an RTT or a loss exists.
func TestLinkQualityThresholds(t *testing.T) {
	ms := time.Millisecond
	tests := []struct {
		name string
		link transport.Link
		want transport.Quality
	}{
		{"nothing measured", transport.Link{Downs10m: 5}, transport.QualityUnknown},
		{"good rtt", transport.Link{RTT: transport.FairRTT - ms}, transport.QualityGood},
		{"fair rtt at threshold", transport.Link{RTT: transport.FairRTT}, transport.QualityFair},
		{"poor rtt at threshold", transport.Link{RTT: transport.PoorRTT}, transport.QualityPoor},
		{"good loss", transport.Link{LossPct: new(transport.FairLoss - 0.1)}, transport.QualityGood},
		{"fair loss at threshold", transport.Link{LossPct: new(transport.FairLoss)}, transport.QualityFair},
		{"poor loss at threshold", transport.Link{LossPct: new(transport.PoorLoss)}, transport.QualityPoor},
		{"fair downs at threshold", transport.Link{RTT: ms, Downs10m: transport.FairDowns}, transport.QualityFair},
		{"poor downs at threshold", transport.Link{RTT: ms, Downs10m: transport.PoorDowns}, transport.QualityPoor},
		{"worst figure wins", transport.Link{RTT: ms, LossPct: new(transport.PoorLoss)}, transport.QualityPoor},
		{"resyncs not graded", transport.Link{RTT: ms, Resyncs10m: 50, Resyncs: 50}, transport.QualityGood},
	}
	for _, tt := range tests {
		if got := tt.link.Quality(); got != tt.want {
			t.Errorf("%s: Quality() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

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
