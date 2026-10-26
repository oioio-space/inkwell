package mux_test

import (
	"testing"

	"go.uber.org/goleak"

	"github.com/oioio-space/inkwell/internal/transport/memnet"
	"github.com/oioio-space/inkwell/internal/transport/mux"
)

// TestSplitNoChannelsStartsNoGoroutine: with no channel there is nothing to
// route and no sub-transport to close, so Split must not start a reader that
// nobody can stop.
func TestSplitNoChannelsStartsNoGoroutine(t *testing.T) {
	node := memnet.New(1, memnet.Faults{}).Join("a")
	if got := mux.Split(node); len(got) != 0 {
		t.Errorf("Split(t) = %v, want no channel", got)
	}
	if got := mux.Split(node, 0xFF); len(got) != 0 {
		t.Errorf("Split(t, reserved) = %v, want no channel", got)
	}
	goleak.VerifyNone(t)
	_ = node.Close()
}
