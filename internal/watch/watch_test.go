package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestChanged(t *testing.T) {
	root := t.TempDir()
	feature := filepath.Join(root, "checkout", "checkout.feature")
	if err := os.MkdirAll(filepath.Dir(feature), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(feature, []byte("Feature: Checkout\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	watcher, err := New(root, start)
	if err != nil {
		t.Fatal(err)
	}
	later := start.Add(Interval)
	if changed, err := watcher.Changed(later); err != nil || changed {
		t.Fatalf("nothing changed, but Changed = %v, %v", changed, err)
	}
	if err := os.WriteFile(feature, []byte("Feature: Checkout\n  Scenario: Pay\n"),
		0o644); err != nil {
		t.Fatal(err)
	}
	if changed, err := watcher.Changed(later.Add(Interval / 2)); err != nil || changed {
		t.Fatalf("looked again before Interval passed: %v, %v", changed, err)
	}
	if changed, err := watcher.Changed(later.Add(Interval)); err != nil || !changed {
		t.Fatalf("an edit wasn't seen: %v, %v", changed, err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, err := watcher.Changed(later.Add(2 * Interval)); err != nil || changed {
		t.Fatalf("a hidden folder counted as a change: %v, %v", changed, err)
	}
}
