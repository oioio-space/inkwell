// SPDX-License-Identifier: MPL-2.0

package invite_test

import (
	"crypto/ed25519"
	"testing"

	"github.com/oioio-space/inkwell/protocol/trust"
)

type signer struct{ key ed25519.PrivateKey }

func testDevice(testing.TB) signer         { return signer{ed25519.NewKeyFromSeed(make([]byte, 32))} }
func (s signer) Public() ed25519.PublicKey { return s.key.Public().(ed25519.PublicKey) }
func (s signer) Sign(domain string, msg []byte) []byte {
	return ed25519.Sign(s.key, append([]byte("inkwell/"+domain+"/v1\x00"), msg...))
}

func newProject(t testing.TB) struct{ list *trust.List } {
	t.Helper()
	g, id := trust.Genesis(testDevice(t), "book", "automerge:x")
	l := trust.New(id)
	if err := l.Apply(g); err != nil {
		t.Fatal(err)
	}
	return struct{ list *trust.List }{l}
}
