package app

import (
	"fmt"
	"path/filepath"
	"slices"

	"givens/internal/assert"
	"givens/internal/runs"
)

// Files attached to a Drop in: proof, like a screenshot, that "Record run" copies into the
// run's own folder in runs/. See internal/runs/files.go for where they go.

// Attach adds the file at path to the files "Record run" keeps with the run. A file with the
// same name as one already attached takes its place. Needed by "Attach proof to a run".
func (s *Session) Attach(path string) error {
	assert.That(s != nil, "attach runs on a session")
	assert.That(len(path) <= PathLengthMax, "the path fits PathLengthMax")
	if !filepath.IsAbs(path) {
		return fmt.Errorf("an attached file needs an absolute path, not %q", path)
	}
	if err := runs.CheckFile(path); err != nil {
		return err
	}
	name := filepath.Base(path)
	if name == runs.NoteFileName {
		return fmt.Errorf("%s is the run note's own name; rename the file to attach it", name)
	}
	kept := slices.DeleteFunc(s.attachments, func(attached string) bool {
		return filepath.Base(attached) == name
	})
	if len(kept) >= runs.FilesPerRunMax {
		return fmt.Errorf("a run keeps at most %d files", runs.FilesPerRunMax)
	}
	s.attachments = append(kept, path)
	assert.That(len(s.attachments) <= runs.FilesPerRunMax, "attachments stay bounded")
	return nil
}

// Detach takes the attached file called name off again.
func (s *Session) Detach(name string) error {
	assert.That(s != nil, "detach runs on a session")
	assert.That(len(s.attachments) <= runs.FilesPerRunMax, "attachments stay bounded")
	before := len(s.attachments)
	s.attachments = slices.DeleteFunc(s.attachments, func(attached string) bool {
		return filepath.Base(attached) == name
	})
	if len(s.attachments) == before {
		return fmt.Errorf("no file called %q is attached: %w", name, ErrNotFound)
	}
	return nil
}

// Attachments lists the paths of the attached files, in the order they were attached.
func (s *Session) Attachments() []string {
	assert.That(s != nil, "attachments runs on a session")
	assert.That(len(s.attachments) <= runs.FilesPerRunMax, "attachments stay bounded")
	return append(make([]string, 0, len(s.attachments)), s.attachments...)
}

// runFiles lists the paths of the files kept with run, for its line in the history. A folder
// that can't be read lists none: the run itself still shows.
func runFiles(capabilities string, run runs.Run) []string {
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	assert.That(run.Key != "", "a run has a key")
	names, err := runs.Files(capabilities, run)
	if err != nil {
		return []string{}
	}
	folder := runs.FilesDir(capabilities, run.Key, run.Time)
	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, filepath.Join(folder, name))
	}
	return paths
}
