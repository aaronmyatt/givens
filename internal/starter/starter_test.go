package starter

import (
	"os"
	"strings"
	"testing"
)

// TestGuideMatchesGivens keeps the starter's GIVENS.md the same as Givens's own, up to the
// section about this repository, so the two can't drift.
func TestGuideMatchesGivens(t *testing.T) {
	own, err := os.ReadFile("../../capabilities/GIVENS.md")
	if err != nil {
		t.Fatal(err)
	}
	cut := strings.Index(string(own), "## This repository")
	if cut < 0 {
		t.Fatal("capabilities/GIVENS.md has no \"## This repository\" section")
	}
	if want := strings.TrimRight(string(own[:cut]), "\n") + "\n"; Guide() != want {
		t.Error("internal/starter/GIVENS.md differs from capabilities/GIVENS.md; copy it over")
	}
}
