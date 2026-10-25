package trust_test

import (
	"os"
	"testing"

	"github.com/oioio-space/inkwell/internal/identity"
	"github.com/oioio-space/inkwell/internal/trust"
)

// Compile-time proof that the real device satisfies the signer contract.
var _ trust.Signer = (*identity.Device)(nil)

func TestDeviceIsTrustSigner(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatalf("os.OpenRoot() = %v", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	dev, err := identity.LoadOrCreate(root)
	if err != nil {
		t.Fatalf("LoadOrCreate() = %v", err)
	}

	genesis, id := trust.Genesis(dev, "book", "automerge:abc")
	l := trust.New(id)
	if err := l.Apply(genesis); err != nil {
		t.Fatalf("Apply(device-signed genesis) = %v", err)
	}

	att, err := trust.NewAttestation(dev, id, "doc", []trust.Hash{{1}}, 1)
	if err != nil {
		t.Fatalf("NewAttestation(device) = %v", err)
	}
	if err := l.Verify(att); err != nil {
		t.Errorf("Verify(device-signed attestation) = %v, want nil", err)
	}

	att.Seq++ // tampering invalidates the signature
	if err := l.Verify(att); err == nil {
		t.Error("Verify(tampered attestation) = nil, want error")
	}
}
