package app

import (
	"os"
	"path/filepath"
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
