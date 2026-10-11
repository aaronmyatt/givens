package kit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	cases := []struct {
		text string
		want int
		ok   bool
	}{
		{"# givens kit 2\nset -e\n", 2, true},
		{"# givens kit 12", 12, true},
		{"#!/bin/bash\n# givens kit 2\n", 0, false},
		{"# givens kit two\n", 0, false},
		{"# givens kit -1\n", 0, false},
	}
	for _, c := range cases {
		if got, ok := Version(c.text); got != c.want || ok != c.ok {
			t.Errorf("Version(%q) = %d, %v; want %d, %v", c.text, got, ok, c.want, c.ok)
		}
	}
	if ShippedVersion() < 1 {
		t.Error("the shipped kit has no version")
	}
}

// TestKitLoadsHelpers runs a setup script that calls a shared and a capability helper.
func TestKitLoadsHelpers(t *testing.T) {
	root := filepath.Join(t.TempDir(), "capabilities")
	const source = `source "$(dirname "${BASH_SOURCE[0]}")/../../_shared/givens.sh"` + "\n"
	files := map[string]string{
		"_shared/givens.sh":           Script,
		"_shared/users.sh":            `seed_user() { ok "seeded $1"; }` + "\n",
		"shop/_shared/cart.sh":        `seed_cart() { ok "cart in ${GIVENS_ROOT##*/}"; }` + "\n",
		"shop/scripts/full-cart.sh":   source + "seed_user sam\nseed_cart\nfalse\nok never\n",
		"other/scripts/no-helpers.sh": source + "seed_cart\n",
	}
	for rel, text := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	output, err := exec.Command("bash", filepath.Join(root, "shop/scripts/full-cart.sh")).
		CombinedOutput()
	if err == nil || string(output) != "✓ seeded sam\n✓ cart in capabilities\n" {
		t.Errorf("full-cart: %v\n%s", err, output)
	}
	output, err = exec.Command("bash", filepath.Join(root, "other/scripts/no-helpers.sh")).
		CombinedOutput()
	if err == nil || !strings.Contains(string(output), "seed_cart") {
		t.Errorf("another capability's helper ran: %v\n%s", err, output)
	}
}
