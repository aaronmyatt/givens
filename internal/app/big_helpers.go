// The big file's helpers: code that two or more of big.go's stories use, kept apart so that
// big.go reads as the stories alone. Each helper says what it does, not which story it serves;
// a function only one story uses belongs in that story's section of big.go. See
// tasks/one-big-file/plan.md.
//
// TestBigFile checks this file holds no Feature, Rule or Scenario headings. If this file grows
// past big.go, that's the tripwire: move a group that belongs together to a file of its own,
// and say why in big.go's header.

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"givens/internal/assert"
)

// repoDepthMax bounds the walk up from a capabilities folder to its git repo's root.
const repoDepthMax = 64

// repoRoot is the root of the git repository holding capabilities, or, outside one, the
// folder holding capabilities. The kit moves there before running anything.
func repoRoot(capabilities string) string {
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	assert.That(repoDepthMax > 0, "the walk up has a limit")
	dir := filepath.Dir(capabilities)
	for depth := 0; depth < repoDepthMax; depth++ {
		// .git is a folder in a clone and a file in a worktree.
		// Ref: https://git-scm.com/docs/gitrepository-layout
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Dir(capabilities)
}

// remember adds folder to one of the app's folder lists: trusted, untrusted, recents.
func (a *App) remember(name string, folder string) error {
	assert.That(name != "", "a list has a file")
	assert.That(filepath.IsAbs(folder), "folders are absolute")
	folders, err := a.folders(name)
	if err != nil || slices.Contains(folders, folder) {
		return err
	}
	if len(folders) >= TrustedMax {
		return fmt.Errorf("Givens already keeps %d folders, the most it keeps", TrustedMax)
	}
	return a.writeFolders(name, append(folders, folder))
}

// forget takes folder out of one of the app's folder lists.
func (a *App) forget(name string, folder string) error {
	assert.That(name != "", "a list has a file")
	assert.That(filepath.IsAbs(folder), "folders are absolute")
	folders, err := a.folders(name)
	if err != nil || !slices.Contains(folders, folder) {
		return err
	}
	kept := slices.DeleteFunc(folders, func(other string) bool { return other == folder })
	return a.writeFolders(name, kept)
}

// writeFolders writes a folder list, one absolute path per line.
func (a *App) writeFolders(name string, folders []string) error {
	assert.That(name != "", "a list has a file")
	assert.That(len(folders) <= TrustedMax, "lists stay bounded")
	text := strings.Join(folders, "\n") + "\n"
	return os.WriteFile(filepath.Join(a.config.AppDataDir, name), []byte(text), 0o644)
}

// folders reads one of the app's folder lists, one absolute path per line.
func (a *App) folders(name string) ([]string, error) {
	assert.That(a != nil, "folders runs on an app")
	assert.That(a.config.AppDataDir != "", "lists live in the app's data folder")
	text, err := os.ReadFile(filepath.Join(a.config.AppDataDir, name))
	if errors.Is(err, os.ErrNotExist) {
		return make([]string, 0, 1), nil
	}
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(text)), "\n")
	if len(lines) > TrustedMax {
		return nil, fmt.Errorf("%s holds %d folders, more than %d", name, len(lines),
			TrustedMax)
	}
	folders := make([]string, 0, len(lines))
	for _, line := range lines {
		if filepath.IsAbs(line) {
			folders = append(folders, line)
		}
	}
	return folders, nil
}
