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
	trustedFileName = "trusted.txt"
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
	trusted, err := a.trustedFolders()
	if err != nil {
		return err
	}
	if slices.Contains(trusted, folder) {
		return nil
	}
	if len(trusted) >= TrustedMax {
		return fmt.Errorf("Givens already trusts %d folders, the most it keeps", TrustedMax)
	}
	trusted = append(trusted, folder)
	text := strings.Join(trusted, "\n") + "\n"
	return os.WriteFile(filepath.Join(a.config.AppDataDir, trustedFileName), []byte(text), 0o644)
}

// Trusted is true when folder, a capabilities folder as a window holds it, was marked trusted.
func (a *App) Trusted(folder string) (bool, error) {
	assert.That(a != nil, "trusted runs on an app")
	assert.That(filepath.IsAbs(folder), "a window's folder is absolute")
	trusted, err := a.trustedFolders()
	if err != nil {
		return false, err
	}
	return slices.Contains(trusted, folder), nil
}

// trustedFolders reads the trusted folders, one absolute path per line.
func (a *App) trustedFolders() ([]string, error) {
	assert.That(a != nil, "trusted folders runs on an app")
	assert.That(a.config.AppDataDir != "", "trust lives in the app's data folder")
	text, err := os.ReadFile(filepath.Join(a.config.AppDataDir, trustedFileName))
	if errors.Is(err, os.ErrNotExist) {
		return make([]string, 0, 1), nil
	}
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(text)), "\n")
	if len(lines) > TrustedMax {
		return nil, fmt.Errorf("%s holds %d folders, more than %d", trustedFileName,
			len(lines), TrustedMax)
	}
	folders := make([]string, 0, len(lines))
	for _, line := range lines {
		if filepath.IsAbs(line) {
			folders = append(folders, line)
		}
	}
	return folders, nil
}
