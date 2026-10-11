package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"givens/internal/assert"
	"givens/internal/kit"
	"givens/internal/starter"
)

// A project window with no capabilities folder says so and offers to make one.
const (
	MessageNoCapabilities = "No capabilities here yet"
	OfferCreateFolder     = "Create capabilities folder"
	// gitattributes keeps both branches' run lines in a merge.
	// Ref: https://git-scm.com/docs/gitattributes#_built_in_merge_drivers
	gitattributes = "runs/** merge=union\n"
)

// createCapabilities makes the window's project a capabilities folder: GIVENS.md, the kit,
// the "Add a story" starter with its setup scripts, and .gitattributes for runs/. It writes
// nothing over a file that's there. See "Create a capabilities folder".
func (w *Window) createCapabilities() error {
	assert.That(w != nil, "create runs on a window")
	assert.That(!w.found, "the window has no capabilities folder yet")
	root := filepath.Join(w.path, capabilitiesFolderName)
	if _, err := os.Stat(root); err == nil {
		return errors.New("there's already a capabilities folder here")
	}
	files := map[string]string{
		"GIVENS.md":         starter.Guide(),
		"_shared/givens.sh": kit.Script,
		".gitattributes":    gitattributes,
	}
	for relative, text := range files {
		if err := writeNew(filepath.Join(root, relative), text); err != nil {
			return err
		}
	}
	// Ref: https://pkg.go.dev/io/fs#WalkDir
	err := fs.WalkDir(starter.Files(), "add-a-story", func(path string, entry fs.DirEntry,
		err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		text, err := fs.ReadFile(starter.Files(), path)
		if err != nil {
			return err
		}
		return writeNew(filepath.Join(root, filepath.FromSlash(path)), string(text))
	})
	if err != nil {
		return err
	}
	w.path, w.found = root, true
	if err := w.reload(); err != nil {
		return err
	}
	return w.app.rememberRecent(root)
}
