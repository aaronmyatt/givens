package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitShop makes the sample shop a git repository, committed on main with a branch "gift-cards"
// at the same commit, and opens it. Git's own config is kept out of the way.
// Ref: https://git-scm.com/docs/git#Documentation/git.txt-GITCONFIGGLOBAL
func gitShop(t *testing.T) (*App, *Window) {
	t.Helper()
	shop := copyShop(t)
	for _, args := range [][]string{
		{"init", "--quiet", "--initial-branch=main"},
		{"config", "user.name", "Sam"},
		{"config", "user.email", "sam@example.com"},
		{"add", "."},
		{"commit", "--quiet", "-m", "Sample shop"},
		{"branch", "gift-cards"},
	} {
		command := exec.Command("git", args...)
		command.Dir = shop
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	core := newApp(t)
	window, err := core.Open(filepath.Join(shop, "capabilities"))
	if err != nil {
		t.Fatal(err)
	}
	return core, window
}

// TestBranchWindowRefusesChanges asks a branch's window for each change the Needs calls can
// ask for. Each is refused, and the copy stays as git archive wrote it.
func TestBranchWindowRefusesChanges(t *testing.T) {
	_, window := gitShop(t)
	branchWindow, err := window.OpenBranch("gift-cards")
	if err != nil || branchWindow == nil {
		t.Fatalf("opening gift-cards gave %v, %v", branchWindow, err)
	}
	if err := branchWindow.Select("Checkout"); err != nil {
		t.Fatal(err)
	}
	_, editErr := branchWindow.Edit("checkout/checkout.feature")
	story := StoryDraft{Name: "Refunds", AsA: "a shop owner", IWant: "refunds", SoThat: "ok"}
	refused := map[string]error{
		"edit":        editErr,
		"newStory":    branchWindow.NewStory(story),
		"newScenario": branchWindow.NewScenario(ScenarioDraft{Name: "Gift card"}),
		"rename":      branchWindow.Rename("Card declined", "Card refused"),
		"delete":      branchWindow.Delete("Empty cart"),
		"newHelper":   branchWindow.NewHelper("checkout", "seed_gift_card"),
		"saveHelper":  branchWindow.SaveHelper("seed_gift_card() { :; }\n"),
	}
	for call, err := range refused {
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("%s in a branch's window gave %v, not ErrReadOnly", call, err)
		}
	}
	if _, err := os.Stat(filepath.Join(branchWindow.Path(), "refunds")); err == nil {
		t.Error("newStory wrote a refunds folder into the branch's copy")
	}
	if _, err := window.Edit("checkout/checkout.feature"); err != nil {
		t.Errorf("the working tree's window refused an edit: %v", err)
	}
}

// TestBranchCopiesCleanedUp closes a branch's window, which removes its copy, and launches
// with a copy left from before, which is removed too.
func TestBranchCopiesCleanedUp(t *testing.T) {
	core, window := gitShop(t)
	branchWindow, err := window.OpenBranch("gift-cards")
	if err != nil || branchWindow == nil {
		t.Fatalf("opening gift-cards gave %v, %v", branchWindow, err)
	}
	copied := core.branchCopy(window.Path(), "gift-cards")
	if _, err := os.Stat(copied); err != nil {
		t.Fatalf("the branch wasn't copied to %s: %v", copied, err)
	}
	if err := branchWindow.CloseWindow(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(copied)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("closing the branch's window left %s: %v", filepath.Dir(copied), err)
	}
}
