package project

import (
	"fmt"
	"testing"
)

// PoC: a legal tree (≤ MaxNodes, ≤ MaxDepth) makes project.json exceed
// maxManifest; the project no longer opens.
func TestPoCTreeBricksManifest(t *testing.T) {
	base := newBase(t)
	p, err := Create(base, "p", "automerge:abc")
	if err != nil {
		t.Fatal(err)
	}
	var es []TreeEntry
	for i := range MaxNodes {
		es = append(es, blobEntry(fmt.Sprintf("assets/img/photo-%05d-holiday.png", i), []byte{byte(i), byte(i >> 8)}))
	}
	if _, err := p.ApplyTree(es); err != nil {
		t.Fatalf("ApplyTree: %v", err)
	}
	_ = p.Close()
	if _, err := Open(base, "p"); err != nil {
		t.Errorf("VULN: project no longer opens after a legal tree: %v", err)
	}
}
