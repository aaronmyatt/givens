package app

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"givens/internal/kit"
	"givens/internal/runs"
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

// copyShop copies the sample shop into a fresh folder, since opening a project can write to
// it (a missing kit is put back) and testdata must stay as checked in.
func copyShop(t *testing.T) string {
	t.Helper()
	shop := t.TempDir()
	// Ref: https://pkg.go.dev/os#CopyFS
	if err := os.CopyFS(shop, os.DirFS(sampleShop)); err != nil {
		t.Fatal(err)
	}
	return shop
}

// TestOpenProjectFolder opens the sample shop's root and finds its capabilities folder.
func TestOpenProjectFolder(t *testing.T) {
	core := newApp(t)
	shop := copyShop(t)
	window, err := core.Open(shop)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(shop, "capabilities")
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
	window, err := newApp(t).Open(filepath.Join(copyShop(t), "capabilities"))
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
	window, err := newApp(t).Open(copyShop(t))
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

// TestNoCopyWithoutSetup offers no Copy for a scenario with nothing to copy, only ways to
// write its setup.
func TestNoCopyWithoutSetup(t *testing.T) {
	shop := copyShop(t)
	script := filepath.Join(shop, "capabilities", "checkout", "scripts", "empty-cart.sh")
	if err := os.Remove(script); err != nil {
		t.Fatal(err)
	}
	core := newApp(t)
	window, err := core.Open(shop)
	if err != nil {
		t.Fatal(err)
	}
	if err := core.MarkTrusted(window.Path()); err != nil {
		t.Fatal(err)
	}
	if err := window.Select("Empty cart"); err != nil {
		t.Fatal(err)
	}
	// Whole labels, as "Copy steps" holds "Copy" and is still offered.
	offers := window.Offers()
	if slices.Contains(offers, OfferCopy) || !slices.Contains(offers, OfferCreateScript) {
		t.Errorf("offers %q, want Create script and no Copy", offers)
	}
}

// TestLeaveDropIn clears a When mark, then leaves the Drop in by selecting something else:
// asked first while marks aren't recorded, closed once "Leave" is chosen.
func TestLeaveDropIn(t *testing.T) {
	shop := copyShop(t)
	// The fixture ships without the kit; projects get it from "Create capabilities folder".
	kitPath := filepath.Join(shop, "capabilities", "_shared", "givens.sh")
	if err := os.WriteFile(kitPath, []byte(kit.Script), 0o644); err != nil {
		t.Fatal(err)
	}
	core := newApp(t)
	window, err := core.Open(shop)
	if err != nil {
		t.Fatal(err)
	}
	if err := core.MarkTrusted(window.Path()); err != nil {
		t.Fatal(err)
	}
	session, err := window.DropIn("Card declined")
	if err != nil {
		t.Fatal(err)
	}
	if status := session.Wait(10 * time.Second); status != SessionReady {
		t.Fatalf("setup ended %q, want ready: %v", status, session.Output())
	}
	var when CheckItem
	for _, item := range session.Checklist() {
		if item.Kind == "when" {
			when = item
		}
	}
	for _, mark := range []string{MarkDone, MarkClear, MarkDone} {
		if err := session.Mark(when.Text, mark); err != nil {
			t.Fatal(err)
		}
	}
	if err := window.Select("Checkout"); err != nil || window.Session() == nil {
		t.Fatalf("selecting left at once (%v); marks would be lost unasked", err)
	}
	if err := window.Choose(OfferLeave); err != nil || window.Session() != nil {
		t.Errorf("after Leave the Drop in is %v (%v), want closed", window.Session(), err)
	}
}

// TestHistoryOf lists one scenario's runs, rows included, newest first, in words.
func TestHistoryOf(t *testing.T) {
	now := time.Date(2026, time.October, 10, 9, 0, 0, 0, time.UTC)
	recorded := []runs.Run{
		{Time: now.AddDate(0, 0, -3), Key: "checkout/card-declined", Result: runs.ResultPass},
		{Time: now.Add(-time.Hour), Key: "checkout/card-declined", Result: runs.ResultFail,
			Author: "Sam", Note: "banner overlaps"},
		{Time: now.AddDate(0, 0, -1), Key: "checkout/card-declined/cards",
			Result: runs.ResultPass, Values: []runs.Pair{{Name: "card", Value: "expired"}}},
		{Time: now, Key: "checkout/empty-cart", Result: runs.ResultPass},
	}
	got := historyOf(t.TempDir(), recorded, "checkout/card-declined", now)
	want := []string{
		"Failed today at 08:00 by Sam",
		"Passed yesterday at 09:00 · card=expired",
		"Passed on 2026-10-07 at 09:00",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for index, line := range got {
		if line.Text != want[index] {
			t.Errorf("run %d is %q, want %q", index, line.Text, want[index])
		}
		if len(line.Files) != 0 {
			t.Errorf("run %d lists files with no folder: %q", index, line.Files)
		}
	}
	if got[0].Note != "banner overlaps" || got[0].Result != runs.ResultFail {
		t.Errorf("the newest run is %+v, want the failed one with its note", got[0])
	}
}

// TestCloseEditor closes a clean editor at once, asks before losing changes, and closes it
// when choosing something else.
func TestCloseEditor(t *testing.T) {
	window, err := newApp(t).Open(copyShop(t))
	if err != nil {
		t.Fatal(err)
	}
	editor, err := window.Edit("checkout/about.md")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(window.Offers(), OfferPreview) {
		t.Errorf("about.md offers %v, want Preview", window.Offers())
	}
	editor.text += "\nMore notes."
	if err := window.Select("Sign in"); err != nil || window.Editor() == nil {
		t.Fatalf("selecting closed an editor with unsaved changes unasked (%v)", err)
	}
	if err := window.Choose(OfferDiscardChanges); err != nil || window.Editor() != nil {
		t.Fatalf("Discard changes left the editor %v (%v)", window.Editor(), err)
	}
	if window.Selected() != "Sign in" {
		t.Errorf("selected %q after closing, want Sign in", window.Selected())
	}
	if _, err := window.Edit("checkout/scripts/card-declined.sh"); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(window.Offers(), OfferPreview) {
		t.Errorf("a script offers %v; Preview has nothing to show for it", window.Offers())
	}
	if err := window.Choose(OfferCloseEditor); err != nil || window.Editor() != nil {
		t.Errorf("Close left a clean editor open (%v)", err)
	}
}

// TestScriptsReadableAndEditable shows each helper's code, and opens a scenario's setup
// script in the editor from its page.
func TestScriptsReadableAndEditable(t *testing.T) {
	window, err := newApp(t).Open(copyShop(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, helper := range window.Helpers() {
		lines := strings.Split(helper.Code, "\n")
		if !strings.HasPrefix(lines[0], helper.Name+"()") || lines[len(lines)-1] != "}" {
			t.Errorf("%s's code is %q, want its whole function", helper.Name, helper.Code)
		}
	}
	if err := window.Select("Card declined"); err != nil {
		t.Fatal(err)
	}
	if err := window.Choose(OfferEditScript); err != nil {
		t.Fatal(err)
	}
	if editor := window.Editor(); editor == nil ||
		!strings.HasSuffix(editor.Path(), "card-declined.sh") {
		t.Errorf("Edit script opened %v, want card-declined.sh", editor)
	}
}

// TestAttachDetach attaches two files, replaces one by name, and takes one off again.
func TestAttachDetach(t *testing.T) {
	session, folder := &Session{}, t.TempDir()
	first, again := filepath.Join(folder, "a.png"), filepath.Join(folder, "b", "a.png")
	other := filepath.Join(folder, "c.log")
	for _, path := range []string{first, again, other} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := session.Attach(path); err != nil {
			t.Fatal(err)
		}
	}
	if got := session.Attachments(); !slices.Equal(got, []string{again, other}) {
		t.Errorf("Attachments = %q", got)
	}
	if err := session.Detach("a.png"); err != nil {
		t.Fatal(err)
	}
	if err := session.Detach("a.png"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Detach of a file not attached = %v", err)
	}
	if err := session.Attach("relative.png"); err == nil {
		t.Error("Attach took a relative path")
	}
}

// TestGherkinOf writes "Copy steps" text: tags, the Scenario: line, then the steps; an outline
// as a Scenario Outline.
func TestGherkinOf(t *testing.T) {
	scenario := Scenario{Name: "Card declined", Tags: []string{"@error"},
		Steps: []string{"Given a shopper", "When they pay"}}
	if got := gherkinOf(scenario); got !=
		"@error\nScenario: Card declined\n  Given a shopper\n  When they pay\n" {
		t.Errorf("gherkinOf = %q", got)
	}
	outline := Scenario{Name: "Cards", Steps: []string{"Given a <card> card"}}
	if got := gherkinOf(outline); got != "Scenario Outline: Cards\n  Given a <card> card\n" {
		t.Errorf("gherkinOf of an outline = %q", got)
	}
}
