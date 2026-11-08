// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const casesSHA256 = "08e47a36d9aead288664930505584f353fff113ab854f2800db1e4f5b3540450"

// notGenerated lists the committed files that this generator does not produce:
// repo.json comes from cmd/repo-vectors (Node), policy.json is kept by hand
// while the policy codec is absent (generators bullet of docs/protocol/spec.md).
var notGenerated = []string{"repo.json", "policy.json"}

// TestGenerateMatchesCommittedCorpus proves the rule of docs/protocol/spec.md,
// section "Générateurs": the committed vectors/*.json are exactly what
// protocol/cmd/vectors regenerates from the pinned speccheck cases.
func TestGenerateMatchesCommittedCorpus(t *testing.T) {
	casesPath := filepath.Join("..", "..", "testdata", "ed25519-speccheck-cases.json")
	raw, err := os.ReadFile(casesPath)
	if err != nil {
		t.Fatal(err)
	}
	if sum := sha256.Sum256(raw); hex.EncodeToString(sum[:]) != casesSHA256 {
		t.Fatalf("SHA-256(%s) = %x, want %s", casesPath, sum, casesSHA256)
	}

	out := t.TempDir()
	generate(out, casesPath)

	generated, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(generated) == 0 {
		t.Fatal("generate wrote no file")
	}
	seen := map[string]bool{}
	for _, e := range generated {
		name := e.Name()
		seen[name] = true
		got, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join("..", "..", "vectors", name))
		if err != nil {
			t.Errorf("%s generated but not committed: %v", name, err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from vectors/%s: %s", name, name, firstDiff(got, want))
		}
	}

	committed, err := os.ReadDir(filepath.Join("..", "..", "vectors"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range committed {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") || seen[name] || slices.Contains(notGenerated, name) {
			continue
		}
		t.Errorf("vectors/%s is committed but not generated (and not listed in notGenerated)", name)
	}
}

// firstDiff describes the first differing line between got and want.
func firstDiff(got, want []byte) string {
	g := strings.Split(string(got), "\n")
	w := strings.Split(string(want), "\n")
	for i := range min(len(g), len(w)) {
		if g[i] != w[i] {
			return "line " + strconv.Itoa(i+1) + ": got " + clip(g[i]) + ", want " + clip(w[i])
		}
	}
	return "length: got " + strconv.Itoa(len(g)) + " lines, want " + strconv.Itoa(len(w))
}

func clip(s string) string {
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}
