package termedit

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"givens/internal/app"
)

type fakes struct{}

func (fakes) WriteText(string) error    { return nil }
func (fakes) ReadText() (string, error) { return "", nil }
func (fakes) Move(path string) error    { return os.RemoveAll(path) }
func (fakes) Now() time.Time            { return time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC) }

// editing opens Card declined in a copy of the sample shop, with a fake editor that runs the
// given edits in turn, each a bash snippet whose $1 is the file.
func editing(t *testing.T, edits ...string) (*app.Editor, string) {
	t.Helper()
	dir := t.TempDir()
	shop := filepath.Join(dir, "shop", "capabilities")
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(shop,
		os.DirFS("../../testdata/sample-shop/capabilities")); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "editor.log")
	script := "#!/bin/bash\necho \"$*\" >> " + log + "\ncount=$(wc -l < " + log + ")\n" +
		"edit=" + dir + "/edit-$((count)).sh\n" +
		`if [ -f "$edit" ]; then bash "$edit" "${!#}"; fi` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "vim"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	for index, edit := range edits {
		name := filepath.Join(dir, "edit-"+string(rune('1'+index))+".sh")
		if err := os.WriteFile(name, []byte(edit), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", filepath.Join(dir, "vim"))
	core, err := app.New(app.Config{AppDataDir: filepath.Join(dir, "data"), Clipboard: fakes{},
		Trash: fakes{}, Clock: fakes{}})
	if err != nil {
		t.Fatal(err)
	}
	window, err := core.Open(shop)
	if err != nil {
		t.Fatal(err)
	}
	editor, err := window.Edit("checkout/card-declined")
	if err != nil {
		t.Fatal(err)
	}
	return editor, log
}

// finishAll runs the editor and Finish until it settles, as Run does without a terminal.
func finishAll(t *testing.T, edit *Edit) Outcome {
	t.Helper()
	for range opensMax {
		if err := edit.Command().Run(); err != nil {
			t.Fatal(err)
		}
		outcome, again, err := edit.Finish()
		if err != nil {
			t.Fatal(err)
		}
		if !again {
			return outcome
		}
	}
	t.Fatal("the editor kept reopening")
	return NoChanges
}

func TestSavedAtTheScenariosLine(t *testing.T) {
	editor, log := editing(t, `sed -i '' 's/the order stays pending/the order is cancelled/' "$1"`)
	edit, err := Begin(editor)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(edit.File()) })
	if outcome := finishAll(t, edit); outcome != Saved {
		t.Fatalf("the edit ended %d", outcome)
	}
	saved, _ := os.ReadFile(editor.Path())
	if !strings.Contains(string(saved), "the order is cancelled") {
		t.Fatalf("the file holds:\n%s", saved)
	}
	calls, _ := os.ReadFile(log)
	if !strings.HasPrefix(string(calls), "+"+strconv.Itoa(editor.Line())+" ") {
		t.Errorf("vim was called with %q, not at line %d", calls, editor.Line())
	}
	if _, err := os.Stat(edit.File()); !os.IsNotExist(err) {
		t.Error("the temporary file is left behind")
	}
}

func TestNoChanges(t *testing.T) {
	editor, _ := editing(t)
	before, _ := os.ReadFile(editor.Path())
	edit, err := Begin(editor)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(edit.File()) })
	if outcome := finishAll(t, edit); outcome != NoChanges {
		t.Fatalf("the edit ended %d", outcome)
	}
	after, _ := os.ReadFile(editor.Path())
	if string(after) != string(before) {
		t.Fatal("the file changed")
	}
}

func TestProblemsReopenUntilFixed(t *testing.T) {
	broken := `perl -pi -e 's/^(    When they place the order)$/` +
		`    Check the order is pending\n$1/' "$1"`
	fixed := `sed -i '' -e '/^# givens: /d' -e '/Check the order is pending/d' "$1"`
	editor, _ := editing(t, broken, `grep -q '^# givens: line' "$1" && `+fixed)
	before, _ := os.ReadFile(editor.Path())
	edit, err := Begin(editor)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(edit.File()) })
	if err := edit.Command().Run(); err != nil {
		t.Fatal(err)
	}
	if _, again, err := edit.Finish(); err != nil || !again {
		t.Fatalf("a broken edit didn't reopen: %v", err)
	}
	shown, _ := os.ReadFile(edit.File())
	if !strings.HasPrefix(string(shown), "# givens: line ") {
		t.Fatalf("the reopened file starts:\n%.200s", shown)
	}
	if after, _ := os.ReadFile(editor.Path()); string(after) != string(before) {
		t.Fatal("a broken edit was saved")
	}
	if outcome := finishAll(t, edit); outcome != Saved {
		t.Fatalf("the fixed edit ended %d", outcome)
	}
}

func TestChangedOnDisk(t *testing.T) {
	editor, _ := editing(t, `sed -i '' 's/stays pending/is cancelled/' "$1"`)
	edit, err := Begin(editor)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(edit.File()) })
	if err := os.WriteFile(editor.Path(), []byte(editor.Text()+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if outcome := finishAll(t, edit); outcome != ChangedOnDisk {
		t.Fatalf("the edit ended %d", outcome)
	}
	kept, _ := os.ReadFile(edit.File())
	if !strings.Contains(string(kept), "is cancelled") {
		t.Fatal("the edit wasn't kept")
	}
}

// Each editor Givens knows opens at the scenario's line, the way it takes one.
func TestLineArgs(t *testing.T) {
	cases := map[string][]string{
		"vim":   {"+12", "f.feature"},
		"nano":  {"+12", "f.feature"},
		"subl":  {"f.feature:12"},
		"zed":   {"f.feature:12"},
		"code":  {"-g", "f.feature:12"},
		"mate":  {"-l", "12", "f.feature"},
		"gedit": {"f.feature"},
	}
	for name, want := range cases {
		if got := lineArgs(name, 12, "f.feature"); !slices.Equal(got, want) {
			t.Errorf("lineArgs(%q) = %q, want %q", name, got, want)
		}
	}
	if got := lineArgs("vim", 0, "f.feature"); !slices.Equal(got, []string{"f.feature"}) {
		t.Errorf("with no line, lineArgs = %q", got)
	}
}
