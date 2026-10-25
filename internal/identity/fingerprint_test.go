package identity_test

import (
	"crypto/ed25519"
	"slices"
	"testing"
	"unicode"

	"github.com/oioio-space/inkwell/internal/identity"
)

func fixedKey() ed25519.PublicKey {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	return ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
}

func TestFingerprintGolden(t *testing.T) {
	pub := fixedKey()
	tests := []struct {
		lang string
		want [4]string
	}{
		// The upstream French list is NFKD-normalised: é is "é".
		{"fr", [4]string{"sombre", "créditer", "breuvage", "gagner"}},
		{"en", [4]string{"tennis", "denial", "cart", "humor"}},
	}
	for _, tc := range tests {
		t.Run(tc.lang, func(t *testing.T) {
			got := identity.Fingerprint(pub, tc.lang)
			if got != tc.want {
				t.Errorf("Fingerprint(fixed key, %q) = %q, want %q", tc.lang, got, tc.want)
			}
			if again := identity.Fingerprint(pub, tc.lang); again != got {
				t.Errorf("Fingerprint cached call = %q, want %q", again, got)
			}
		})
	}
}

func TestFingerprintSameIndicesAcrossLanguages(t *testing.T) {
	pub := fixedKey()
	fr, en := identity.Wordlist("fr"), identity.Wordlist("en")
	gotFR, gotEN := identity.Fingerprint(pub, "fr"), identity.Fingerprint(pub, "en")
	for i := range gotFR {
		if slices.Index(fr, gotFR[i]) != slices.Index(en, gotEN[i]) {
			t.Errorf("word %d: fr %q and en %q have different indices", i, gotFR[i], gotEN[i])
		}
	}
}

func TestFingerprintDiffersPerKey(t *testing.T) {
	other := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	if identity.Fingerprint(fixedKey(), "en") == identity.Fingerprint(other, "en") {
		t.Error("two different keys share a fingerprint")
	}
}

func TestWordlists2048Unique4Prefix(t *testing.T) {
	for _, lang := range []string{"fr", "en"} {
		t.Run(lang, func(t *testing.T) {
			words := identity.Wordlist(lang)
			if len(words) != 2048 {
				t.Fatalf("Wordlist(%q) has %d words, want 2048", lang, len(words))
			}
			seen := map[string]bool{}
			prefixes := map[string]string{}
			for _, w := range words {
				if w == "" {
					t.Fatalf("Wordlist(%q) contains an empty word", lang)
				}
				if seen[w] {
					t.Errorf("duplicate word %q", w)
				}
				seen[w] = true
				// Upstream lists are NFKD: ignore combining accents.
				var r []rune
				for _, c := range w {
					if !unicode.IsMark(c) {
						r = append(r, c)
					}
				}
				p := string(r[:min(4, len(r))])
				if prev, dup := prefixes[p]; dup {
					t.Errorf("words %q and %q share 4-letter prefix %q", prev, w, p)
				}
				prefixes[p] = w
			}
		})
	}
}
