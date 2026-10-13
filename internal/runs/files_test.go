package runs

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestCopyFilesAndList copies two files into a run's folder, leaves the originals, and lists
// them back sorted.
func TestCopyFilesAndList(t *testing.T) {
	root, from := t.TempDir(), t.TempDir()
	run := Run{
		Time: time.Date(2026, time.October, 12, 21, 38, 5, 0, time.FixedZone("MYT", 8*3600)),
		Key:  "checkout/card-declined",
	}
	paths := []string{filepath.Join(from, "z.log"), filepath.Join(from, "declined.png")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte(filepath.Base(path)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := CopyFiles(root, run, paths); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "runs", "files", "checkout", "card-declined",
		"2026-10-12T133805Z")
	if got := FilesDir(root, run.Key, run.Time); got != want {
		t.Errorf("FilesDir = %s, want %s", got, want)
	}
	names, err := Files(root, run)
	if err != nil || !slices.Equal(names, []string{"declined.png", "z.log"}) {
		t.Errorf("Files = %q, %v", names, err)
	}
	if _, err := os.Stat(paths[0]); err != nil {
		t.Errorf("the original was moved: %v", err)
	}
}

// TestCopyFilesFailsWhole removes the run's folder when one file can't be copied.
func TestCopyFilesFailsWhole(t *testing.T) {
	root, from := t.TempDir(), t.TempDir()
	run := Run{Time: time.Date(2026, time.October, 12, 9, 0, 0, 0, time.UTC), Key: "a/b"}
	good := filepath.Join(from, "good.txt")
	if err := os.WriteFile(good, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyFiles(root, run, []string{good, filepath.Join(from, "gone.txt")}); err == nil {
		t.Fatal("CopyFiles copied a file that isn't there")
	}
	if _, err := os.Stat(FilesDir(root, run.Key, run.Time)); !os.IsNotExist(err) {
		t.Errorf("the run's folder was left behind: %v", err)
	}
	if names, err := Files(root, run); err != nil || len(names) != 0 {
		t.Errorf("Files = %q, %v", names, err)
	}
}

// TestCheckFileTooBig turns down a file over FileBytesMax. Truncate makes a sparse file, so
// the test writes no data.
// Ref: https://pkg.go.dev/os#Truncate
func TestCheckFileTooBig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "huge.mov")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, FileBytesMax+1); err != nil {
		t.Fatal(err)
	}
	if err := CheckFile(path); err == nil || !strings.Contains(err.Error(), "more than 100 MB") {
		t.Errorf("CheckFile = %v", err)
	}
	if err := CheckFile(filepath.Dir(path)); err == nil {
		t.Error("CheckFile took a folder")
	}
}

// TestNote keeps a long note as note.md, reads it back, and leaves it out of Files.
func TestNote(t *testing.T) {
	root := t.TempDir()
	run := Run{Time: time.Date(2026, time.October, 12, 9, 0, 0, 0, time.UTC), Key: "a/b"}
	note := "\n## Retry button\n\n- below the fold\n"
	if got := Summary(note); got != "Retry button" {
		t.Errorf("Summary = %q", got)
	}
	if got, err := ReadNote(root, run); err != nil || got != "" {
		t.Errorf("ReadNote with no note = %q, %v", got, err)
	}
	if err := WriteNote(root, run, note); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadNote(root, run); err != nil || got != note {
		t.Errorf("ReadNote = %q, %v", got, err)
	}
	if names, err := Files(root, run); err != nil || len(names) != 0 {
		t.Errorf("Files lists the note: %q, %v", names, err)
	}
}
