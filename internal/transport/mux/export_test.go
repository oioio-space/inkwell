package mux

import "github.com/oioio-space/inkwell/internal/transport"

// GateCount returns how many peers currently have a large-message gate, and
// the lock-free counter that mirrors it, for the bookkeeping tests.
func GateCount(t transport.Transport) (gates int, counter int32) {
	m := t.(*sub).m
	m.gmu.RLock()
	defer m.gmu.RUnlock()
	return len(m.gates), m.ngates.Load()
}
