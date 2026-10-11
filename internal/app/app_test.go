package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakes stands in for the clipboard, the Trash and the clock.
type fakes struct{ text string }

func (f *fakes) WriteText(text string) error { f.text = text; return nil }
func (f *fakes) ReadText() (string, error)   { return f.text, nil }
func (f *fakes) Move(string) error           { return nil }
func (f *fakes) Now() time.Time {
	return time.Date(2026, time.October, 10, 9, 0, 0, 0, time.UTC)
}

// newApp builds an app with fakes and a fresh data folder.
func newApp(t *testing.T) *App {
	t.Helper()
	f := &fakes{}
	core, err := New(Config{AppDataDir: t.TempDir(), Clipboard: f, Trash: f, Clock: f})
	if err != nil {
		t.Fatal(err)
	}
	return core
}

const sampleShop = "../../testdata/sample-shop"

// TestOpenProjectFolder opens the sample shop's root and finds its capabilities folder.
func TestOpenProjectFolder(t *testing.T) {
	core := newApp(t)
	window, err := core.Open(sampleShop)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(filepath.Join(sampleShop, "capabilities"))
	if err != nil {
		t.Fatal(err)
	}
	if window.Title() != want || window.Path() != want || core.Front() != window {
		t.Errorf("title %q, path %q, want %q", window.Title(), window.Path(), want)
	}
	again, err := core.Open(want)
	if err != nil || again != window || len(core.Windows()) != 1 {
		t.Errorf("reopening gave %p (want %p), %d windows, %v", again, window,
			len(core.Windows()), err)
	}
}

// TestCapabilityAndScenario reads what a window shows for Checkout and Card declined.
func TestCapabilityAndScenario(t *testing.T) {
	window, err := newApp(t).Open(filepath.Join(sampleShop, "capabilities"))
	if err != nil {
		t.Fatal(err)
	}
	checkout, err := window.Capability("Checkout")
	if err != nil {
		t.Fatal(err)
	}
	if len(checkout.Story) != 3 || len(checkout.Scenarios) != 5 || checkout.About == "" {
		t.Errorf("checkout = %+v", checkout)
	}
	declined, err := window.Scenario("Checkout/Card declined")
	if err != nil {
		t.Fatal(err)
	}
	if !declined.Error || declined.Setup == "" || declined.Steps[0] !=
		"Given a signed-in shopper with one item in the cart" {
		t.Errorf("card declined = %+v", declined)
	}
	if _, err := window.Scenario("Pay with card"); err == nil {
		t.Error("found a scenario that doesn't exist")
	}
}

// TestOpenFolderWithoutCapabilities opens a window that has no project yet.
func TestOpenFolderWithoutCapabilities(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	window, err := newApp(t).Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if window.project != nil || window.found {
		t.Errorf("a folder with no capabilities loaded a project: %+v", window.project)
	}
}

// TestMarkTrusted trusts the sample shop by its project folder and finds it by its
// capabilities folder, as a window holds it.
func TestMarkTrusted(t *testing.T) {
	core := newApp(t)
	capabilities, err := filepath.Abs(filepath.Join(sampleShop, "capabilities"))
	if err != nil {
		t.Fatal(err)
	}
	if trusted, err := core.Trusted(capabilities); err != nil || trusted {
		t.Fatalf("trusted before marking: %v, %v", trusted, err)
	}
	for range 2 {
		if err := core.MarkTrusted(sampleShop); err != nil {
			t.Fatal(err)
		}
	}
	if trusted, err := core.Trusted(capabilities); err != nil || !trusted {
		t.Errorf("not trusted after marking: %v, %v", trusted, err)
	}
	folders, err := core.trustedFolders()
	if err != nil || len(folders) != 1 {
		t.Errorf("trusted folders = %q, %v", folders, err)
	}
}

// TestFilesWrittenByOtherToolsAppear adds a scenario behind the window's back and waits for
// the window to show it, as "Files written by other tools appear" does.
func TestFilesWrittenByOtherToolsAppear(t *testing.T) {
	root := filepath.Join(t.TempDir(), "capabilities")
	if err := os.CopyFS(root, os.DirFS(filepath.Join(sampleShop, "capabilities"))); err != nil {
		t.Fatal(err)
	}
	window, err := newApp(t).Open(root)
	if err != nil {
		t.Fatal(err)
	}
	feature := filepath.Join(root, "sign-in", "sign-in.feature")
	text, err := os.ReadFile(feature)
	if err != nil {
		t.Fatal(err)
	}
	added := string(text) + "\n  Scenario: Remember me\n    Then they stay signed in\n"
	if err := os.WriteFile(feature, []byte(added), 0o644); err != nil {
		t.Fatal(err)
	}
	for range 40 {
		if _, err := window.Scenario("Remember me"); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Error("the added scenario didn't appear within 2 seconds")
}

// TestSuggestionsForALine offers steps for the line the Mac editor is on, and none for a line
// that isn't a step or a step already written out.
func TestSuggestionsForALine(t *testing.T) {
	window, err := newApp(t).Open(sampleShop)
	if err != nil {
		t.Fatal(err)
	}
	editor, err := window.Edit("sign-in/sign-in.feature")
	if err != nil {
		t.Fatal(err)
	}
	offered := editor.SuggestionsFor("    Given a shopper has")
	if len(offered) != 2 || offered[0].Label != "used in 1" {
		t.Errorf("offered %v, want the two \"a shopper has\" steps", offered)
	}
	lines := []string{"Feature: a shopper has", "Given ", "Given a shopper has an account"}
	for _, line := range lines {
		if got := editor.SuggestionsFor(line); len(got) != 0 {
			t.Errorf("%q offered %v, want none", line, got)
		}
	}
}

// TestWritableFolders offers PATH folders that exist and can be written to, ~/.local/bin
// first.
func TestWritableFolders(t *testing.T) {
	home := t.TempDir()
	mine := filepath.Join(home, ".local", "bin")
	shared := t.TempDir()
	locked := t.TempDir()
	if err := os.MkdirAll(mine, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	list := strings.Join([]string{shared, locked, "relative/bin", filepath.Join(home, "gone"),
		mine, shared}, string(filepath.ListSeparator))
	got := writableFolders(list, home)
	if len(got) != 2 || got[0] != mine || got[1] != shared {
		t.Errorf("offered %v, want [%s %s]", got, mine, shared)
	}
}
