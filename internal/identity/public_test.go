package identity

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/oioio-space/inkwell/protocol/vectors"

	"filippo.io/edwards25519"
)

func publicVectors(t testing.TB) (valid, invalid [][]byte) {
	t.Helper()
	data, err := vectors.FS.ReadFile("crypto.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Keys map[string]struct {
			Public string `json:"public_hex"`
		}
		Vectors []struct {
			ID     string `json:"id"`
			Expect struct {
				Keys []string `json:"keys_hex"`
			}
		}
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	decode := func(s string) []byte {
		b, err := hex.DecodeString(s)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	for _, k := range doc.Keys {
		valid = append(valid, decode(k.Public))
	}
	for _, v := range doc.Vectors {
		if v.ID == "small-order-keys" || v.ID == "noncanonical-public-keys" {
			for _, k := range v.Expect.Keys {
				invalid = append(invalid, decode(k))
			}
		}
	}
	if len(invalid) != 11 || len(valid) != 2 {
		t.Fatal("missing protocol vectors")
	}
	torsion, err := new(edwards25519.Point).SetBytes(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range valid {
		p, err := new(edwards25519.Point).SetBytes(key)
		if err != nil {
			t.Fatal(err)
		}
		invalid = append(invalid, new(edwards25519.Point).Add(p, torsion).Bytes())
	}
	offCurve := make([]byte, 32)
	offCurve[0] = 2
	invalid = append(invalid, offCurve)
	for y := 237; y <= 255; y++ {
		for _, sign := range []byte{0, 128} {
			k := bytes.Repeat([]byte{255}, 32)
			k[0] = byte(y)
			k[31] = 127 | sign
			invalid = append(invalid, k)
		}
	}
	return valid, invalid
}

func TestValidatePublicKey(t *testing.T) {
	valid, invalid := publicVectors(t)
	for _, key := range valid {
		if err := ValidatePublicKey(key); err != nil {
			t.Errorf("valid %x: %v", key, err)
		}
	}
	invalid = append(invalid, nil, make([]byte, 31), bytes.Repeat([]byte{255}, 32))
	for _, key := range invalid {
		if ValidatePublicKey(key) == nil {
			t.Errorf("accepted %x", key)
		}
	}
}

func FuzzValidatePublicKey(f *testing.F) {
	valid, invalid := publicVectors(f)
	for _, key := range append(valid, invalid...) {
		f.Add(key)
	}
	f.Fuzz(func(t *testing.T, key []byte) {
		before := bytes.Clone(key)
		err := ValidatePublicKey(key)
		if !bytes.Equal(before, key) {
			t.Fatal("mutated key")
		}
		if err != nil {
			return
		}
		p, err := new(edwards25519.Point).SetBytes(key)
		if err != nil || !bytes.Equal(p.Bytes(), key) {
			t.Fatal("accepted invalid encoding")
		}
		// Independently multiply by the integer subgroup order, without scalar reduction.
		order, _ := hex.DecodeString("edd3f55c1a631258d69cf7a2def9de1400000000000000000000000000000010")
		multiple := edwards25519.NewIdentityPoint()
		for bit := 252; bit >= 0; bit-- {
			multiple.Add(multiple, multiple)
			if order[bit/8]&(1<<uint(bit%8)) != 0 {
				multiple.Add(multiple, p)
			}
		}
		if multiple.Equal(edwards25519.NewIdentityPoint()) != 1 {
			t.Fatal("accepted mixed-order point")
		}
		for range 3 {
			p.Add(p, p)
		}
		if p.Equal(edwards25519.NewIdentityPoint()) == 1 {
			t.Fatal("accepted small-order point")
		}
	})
}

func TestPrimeOrderPublicKeys(t *testing.T) {
	torsion, err := new(edwards25519.Point).SetBytes(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	for i := range 20000 {
		pub, _, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidatePublicKey(pub); err != nil {
			t.Fatalf("honest key %d: %v", i, err)
		}
		if i < 8 {
			p, err := new(edwards25519.Point).SetBytes(pub)
			if err != nil {
				t.Fatal(err)
			}
			if ValidatePublicKey(new(edwards25519.Point).Add(p, torsion).Bytes()) == nil {
				t.Fatal("accepted mixed-order key")
			}
		}
	}
}
