// Package watch notices changes to a capabilities folder by polling: it lists every file's
// path, size and modification time, and compares that with the last look.
//
// Polling keeps Givens on the standard library and works the same for the CLI, the steps and
// the app. Cost: one stat per file per look; a 1000-file folder is about a millisecond.
// FSEvents was the other option; see tasks/foundation/notes.md.
package watch

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"givens/internal/assert"
)

// Limits. Tiger Style: put a limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const (
	// Interval is the least time between two looks. Changes show within 2 seconds, so it's
	// well under that.
	Interval = 500 * time.Millisecond
	// FilesMax bounds the files one look lists.
	FilesMax = 20000
)

// file is what a look records for one file.
type file struct {
	size     int64
	modified time.Time
}

// Watcher remembers the last look at one folder.
type Watcher struct {
	root   string
	last   map[string]file
	looked time.Time
}

// New takes a first look at root.
func New(root string, now time.Time) (*Watcher, error) {
	assert.That(filepath.IsAbs(root), "root is absolute")
	assert.That(!now.IsZero(), "now is a time")
	files, err := look(root)
	if err != nil {
		return nil, err
	}
	return &Watcher{root: root, last: files, looked: now}, nil
}

// Changed looks again when Interval has passed since the last look, and is true when a file
// was added, removed or changed since. now is the wall clock: callers pass time.Now().
func (w *Watcher) Changed(now time.Time) (bool, error) {
	assert.That(w != nil, "changed runs on a watcher")
	assert.That(!now.IsZero(), "now is a time")
	if now.Sub(w.looked) < Interval {
		return false, nil
	}
	return w.Look(now)
}

// Look looks again now, whatever the time since the last look, and is true when a file was
// added, removed or changed since. A reader that must not see stale files uses it; one that
// polls on a timer uses Changed.
func (w *Watcher) Look(now time.Time) (bool, error) {
	assert.That(w != nil, "look runs on a watcher")
	assert.That(!now.IsZero(), "now is a time")
	files, err := look(w.root)
	if err != nil {
		return false, err
	}
	changed := !same(w.last, files)
	w.last, w.looked = files, now
	return changed, nil
}

// look lists every file under root. Hidden folders are skipped: .git changes on every commit.
func look(root string) (map[string]file, error) {
	assert.That(filepath.IsAbs(root), "root is absolute")
	files := make(map[string]file, 256)
	// WalkDir visits each entry once and never follows symlinks, so it ends.
	// Ref: https://pkg.go.dev/path/filepath#WalkDir
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && path != root && entry.Name()[0] == '.' {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		if len(files) >= FilesMax {
			return fmt.Errorf("%s holds more than %d files", root, FilesMax)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		files[path] = file{size: info.Size(), modified: info.ModTime()}
		return nil
	})
	assert.That(len(files) <= FilesMax, "a look stays bounded")
	return files, err
}

// same is true when two looks list the same files with the same sizes and times.
func same(before map[string]file, after map[string]file) bool {
	assert.That(len(before) <= FilesMax, "before stays bounded")
	assert.That(len(after) <= FilesMax, "after stays bounded")
	if len(before) != len(after) {
		return false
	}
	for path, was := range before {
		now, ok := after[path]
		if !ok || now.size != was.size || !now.modified.Equal(was.modified) {
			return false
		}
	}
	return true
}
