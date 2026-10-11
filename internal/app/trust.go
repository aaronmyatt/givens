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

// Trust lives in the app's own data folder, never in a project, so a cloned repo can't mark
// itself trusted. See capabilities/trust-a-project.
const (
	trustedFileName   = "trusted.txt"
	untrustedFileName = "untrusted.txt"
	// TrustedMax bounds the trusted folders Givens remembers.
	TrustedMax = 1024
)

// MarkTrusted records a project's folder as trusted, as choosing "Trust" on the "Run scripts
// from this project?" prompt does. path is a capabilities folder, or the project folder holding
// one, as Open takes. Drop in scenarios that aren't about trust use it to start trusted.
func (a *App) MarkTrusted(path string) error {
	assert.That(a != nil, "mark trusted runs on an app")
	assert.That(path != "", "a trusted folder has a path")
	assert.That(len(path) <= PathLengthMax, "the path fits PathLengthMax")
	folder, _, err := findCapabilities(path)
	if err != nil {
		return err
	}
	if err := a.forget(untrustedFileName, folder); err != nil {
		return err
	}
	return a.remember(trustedFileName, folder)
}

// markUntrusted records "Don't trust" for a capabilities folder: its scripts stay off until
// the developer chooses "Trust this project".
func (a *App) markUntrusted(folder string) error {
	assert.That(a != nil, "mark untrusted runs on an app")
	assert.That(filepath.IsAbs(folder), "a window's folder is absolute")
	if err := a.forget(trustedFileName, folder); err != nil {
		return err
	}
	return a.remember(untrustedFileName, folder)
}

// untrusted is true when the developer chose "Don't trust" for folder.
func (a *App) untrusted(folder string) (bool, error) {
	assert.That(a != nil, "untrusted runs on an app")
	assert.That(filepath.IsAbs(folder), "a window's folder is absolute")
	folders, err := a.folders(untrustedFileName)
	if err != nil {
		return false, err
	}
	return slices.Contains(folders, folder), nil
}

// remember adds folder to one of the app's folder lists.
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

// Trusted is true when folder, a capabilities folder as a window holds it, was marked trusted.
func (a *App) Trusted(folder string) (bool, error) {
	assert.That(a != nil, "trusted runs on an app")
	assert.That(filepath.IsAbs(folder), "a window's folder is absolute")
	trusted, err := a.folders(trustedFileName)
	if err != nil {
		return false, err
	}
	return slices.Contains(trusted, folder), nil
}

// trustedFolders reads the trusted folders.
func (a *App) trustedFolders() ([]string, error) {
	assert.That(a != nil, "trusted folders runs on an app")
	assert.That(a.config.AppDataDir != "", "trust lives in the app's data folder")
	return a.folders(trustedFileName)
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
