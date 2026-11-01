package app

import (
	"testing"

	"github.com/oioio-space/inkwell/internal/transport/memnet"
	"github.com/oioio-space/inkwell/internal/trust"
)

func TestSmallOrderAttestationBeforeStorage(t *testing.T) {
	dev := newDevice(t)
	l := newProject(t, dev)
	m := join(t, memnet.New(1, memnet.Faults{}), dev, l)
	key := make([]byte, 32)
	key[0] = 1
	a := trust.Attestation{Project: l.Project(), Doc: "doc", Signer: key, Sig: [64]byte{1}}
	if err := m.w.Attest(a); err == nil {
		t.Fatal("accepted forged attestation")
	}
	if len(m.w.Attestations()) != 0 {
		t.Fatal("stored forged attestation")
	}
}
