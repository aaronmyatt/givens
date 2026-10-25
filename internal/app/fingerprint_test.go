package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"givens/internal/project"
)

// copyShopCapabilities copies the sample shop's capabilities folder and returns its path.
func copyShopCapabilities(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "capabilities")
	if err := os.CopyFS(root, os.DirFS("../../testdata/sample-shop/capabilities")); err != nil {
		t.Fatal(err)
	}
	return root
}

// fingerprintOf loads root and fingerprints one scenario by its run key.
func fingerprintOf(t *testing.T, root string, key string) []RunPair {
	t.Helper()
	loaded, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := loaded.Find(key)
	if err != nil {
		t.Fatal(err)
	}
	parts, err := FingerprintOf(loaded, scenario, nil)
	if err != nil {
		t.Fatal(err)
	}
	return parts
}

// edit replaces old with replacement in one file under root.
func edit(t *testing.T, root string, rel string, old string, replacement string) {
	t.Helper()
	path := filepath.Join(root, rel)
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), old) {
		t.Fatalf("%s has no %q", rel, old)
	}
	updated := strings.Replace(string(text), old, replacement, 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestChanged(t *testing.T) {
	root := copyShopCapabilities(t)
	declined := fingerprintOf(t, root, "checkout/card-declined")
	invite := fingerprintOf(t, root, "invites/accept-invite")
	edit(t, root, "_shared/users.sh", `ok "seeded an address`, `ok "seeded one address`)
	if got := FingerprintChanged(declined,
		fingerprintOf(t, root, "checkout/card-declined")); got != "seed_address" {
		t.Errorf("after seed_address changed, Changed = %q", got)
	}
	if got := FingerprintChanged(invite,
		fingerprintOf(t, root, "invites/accept-invite")); got != "" {
		t.Errorf("Accept invite doesn't call seed_address, but Changed = %q", got)
	}
	before := fingerprintOf(t, root, "checkout/3-d-secure-failed")
	edit(t, root, "checkout/scripts/pay-with-saved-card.sh", "seed_cart 1", "seed_cart 2")
	if got := FingerprintChanged(before,
		fingerprintOf(t, root, "checkout/3-d-secure-failed")); got !=
		"checkout/pay-with-saved-card setup" {
		t.Errorf("after a setup it builds on changed, Changed = %q", got)
	}
	before = fingerprintOf(t, root, "checkout/card-declined")
	edit(t, root, "checkout/checkout.feature", "the order stays pending", "the order fails")
	if got := FingerprintChanged(before,
		fingerprintOf(t, root, "checkout/card-declined")); got != "steps" {
		t.Errorf("after a step changed, Changed = %q", got)
	}
}
