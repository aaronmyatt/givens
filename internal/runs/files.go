package runs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"givens/internal/assert"
)

// Attached files: proof of what a run saw, like a screenshot. Each run keeps its own folder,
// runs/files/<run key>/<time>/, found from the run line's key and time, so the line format
// doesn't change. Names under one run's folder can't collide with another branch's, so a merge
// only ever adds folders.

// Limits on attached files.
const (
	// FilesPerRunMax bounds the files one run keeps.
	FilesPerRunMax = 32
	// FileBytesMax bounds one attached file: room for a screen recording, not a disk image.
	FileBytesMax = 100 << 20
)

// filesFolderTimeLayout names a run's folder by its time in UTC, so folders sort by time and
// the name has no colon, which Finder shows as a slash.
// Ref: https://pkg.go.dev/time#pkg-constants
const filesFolderTimeLayout = "2006-01-02T150405Z"

// FilesDir is the folder holding the files attached to the run recorded against key at at.
func FilesDir(capabilities string, key string, at time.Time) string {
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	assert.That(key != "", "a run has a key")
	assert.That(!at.IsZero(), "a run has a time")
	return filepath.Join(Dir(capabilities), "files", filepath.FromSlash(key),
		at.UTC().Format(filesFolderTimeLayout))
}

// CheckFile returns an error unless path is a regular file of at most FileBytesMax bytes.
func CheckFile(path string) error {
	assert.That(path != "", "a file has a path")
	assert.That(filepath.IsAbs(path), "an attached file's path is absolute")
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s isn't a file", filepath.Base(path))
	}
	if info.Size() > FileBytesMax {
		return fmt.Errorf("%s is %d MB, more than %d MB", filepath.Base(path), info.Size()>>20,
			FileBytesMax>>20)
	}
	return nil
}

// CopyFiles copies each of paths into run's folder, keeping their names; the originals stay
// where they are. If any copy fails, the folder is removed again, so no run points at half its
// files.
func CopyFiles(capabilities string, run Run, paths []string) error {
	assert.That(len(paths) <= FilesPerRunMax, "attached files stay bounded")
	assert.That(namesUnique(paths), "attached files have different names")
	if len(paths) == 0 {
		return nil
	}
	folder := FilesDir(capabilities, run.Key, run.Time)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	for _, path := range paths {
		if err := copyFile(path, filepath.Join(folder, filepath.Base(path))); err != nil {
			return errors.Join(err, os.RemoveAll(folder))
		}
	}
	return nil
}

// copyFile copies one regular file of at most FileBytesMax bytes from source to target.
// Ref: https://pkg.go.dev/io#Copy
func copyFile(source string, target string) error {
	assert.That(source != target, "a file isn't copied onto itself")
	assert.That(filepath.IsAbs(target), "target is absolute")
	if err := CheckFile(source); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	// LimitReader stops a file that grew since CheckFile from filling the disk.
	// Ref: https://pkg.go.dev/io#LimitReader
	written, err := io.Copy(out, io.LimitReader(in, FileBytesMax+1))
	if err != nil {
		return errors.Join(err, out.Close())
	}
	if written > FileBytesMax {
		return errors.Join(fmt.Errorf("%s grew past %d MB while it was copied",
			filepath.Base(source), FileBytesMax>>20), out.Close())
	}
	return out.Close()
}

// Files lists the names of the files attached to run, sorted, its note.md left out. A run with
// no folder has none.
func Files(capabilities string, run Run) ([]string, error) {
	assert.That(run.Key != "", "a run has a key")
	assert.That(!run.Time.IsZero(), "a run has a time")
	// ReadDir returns entries sorted by name.
	// Ref: https://pkg.go.dev/os#ReadDir
	entries, err := os.ReadDir(FilesDir(capabilities, run.Key, run.Time))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, min(len(entries), FilesPerRunMax))
	for _, entry := range entries {
		if entry.Type().IsRegular() && entry.Name() != NoteFileName &&
			len(names) < FilesPerRunMax {
			names = append(names, entry.Name())
		}
	}
	assert.That(sort.StringsAreSorted(names), "names come back sorted")
	return names, nil
}

// namesUnique is true when no two of paths share a file name.
func namesUnique(paths []string) bool {
	assert.That(len(paths) <= FilesPerRunMax, "paths stay bounded")
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		if seen[filepath.Base(path)] {
			return false
		}
		seen[filepath.Base(path)] = true
	}
	return true
}

// The run's note, when it runs to more than one line: a run line is one line, so the whole
// note is kept beside the run's files and the line keeps its first line.
const (
	// NoteFileName is the note's file in a run's folder; no attached file may take the name.
	NoteFileName = "note.md"
	// NoteBytesMax bounds a note: pages of Markdown, not a book.
	NoteBytesMax = 64 << 10
)

// Summary is what a run line keeps of note: its first line that has text, any Markdown
// heading marks taken off. "## Retry button\nIt's below the fold" gives "Retry button".
func Summary(note string) string {
	assert.That(len(note) <= NoteBytesMax, "a note fits NoteBytesMax")
	for _, line := range strings.Split(note, "\n") {
		trimmed := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if trimmed != "" {
			return trimmed
		}
	}
	assert.That(strings.TrimSpace(note) == "" || strings.Trim(note, "#\n\t\r ") == "",
		"only a note with no text has no summary")
	return ""
}

// WriteNote keeps note whole as note.md in run's folder, making the folder if needed.
func WriteNote(capabilities string, run Run, note string) error {
	assert.That(len(note) <= NoteBytesMax, "a note fits NoteBytesMax")
	assert.That(note != "", "an empty note isn't written")
	folder := FilesDir(capabilities, run.Key, run.Time)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(folder, NoteFileName), []byte(note), 0o644)
}

// ReadNote is run's whole note from its note.md, or "" when it has none.
func ReadNote(capabilities string, run Run) (string, error) {
	assert.That(run.Key != "", "a run has a key")
	assert.That(!run.Time.IsZero(), "a run has a time")
	path := filepath.Join(FilesDir(capabilities, run.Key, run.Time), NoteFileName)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if info.Size() > NoteBytesMax {
		return "", fmt.Errorf("%s is %d bytes, more than %d", path, info.Size(), NoteBytesMax)
	}
	content, err := os.ReadFile(path)
	return string(content), err
}
