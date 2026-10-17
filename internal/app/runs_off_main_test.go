package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

// TestRunsOffMainByDefault keeps runs off main in a git repository with a remote, offering
// "Move runs off main" only while capabilities/runs holds runs; and not in a folder outside
// git.
func TestRunsOffMainByDefault(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	shop := copyShop(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	for _, args := range [][]string{
		{"init", "--quiet", "--initial-branch=main"},
		{"init", "--quiet", "--bare", remote},
		{"remote", "add", "origin", remote},
	} {
		command := exec.Command("git", args...)
		command.Dir = shop
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	window, err := newApp(t).Open(filepath.Join(shop, "capabilities"))
	if err != nil {
		t.Fatal(err)
	}
	if !window.keepsRunsOffMain() {
		t.Error("a repository with a remote doesn't keep its runs off main")
	}
	if offers := window.Offers(); slices.Contains(offers, OfferMoveRunsOffMain) {
		t.Errorf("with no runs in capabilities/runs, the window offers %q", offers)
	}
	logs := filepath.Join(shop, "capabilities", "runs")
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	day := filepath.Join(logs, "2026-10-10.log")
	if err := os.WriteFile(day, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if offers := window.Offers(); !slices.Contains(offers, OfferMoveRunsOffMain) {
		t.Errorf("with runs in capabilities/runs, the window offers %q", offers)
	}
	plain, err := newApp(t).Open(filepath.Join(copyShop(t), "capabilities"))
	if err != nil {
		t.Fatal(err)
	}
	if plain.keepsRunsOffMain() {
		t.Error("a folder outside git keeps its runs off main")
	}
}
