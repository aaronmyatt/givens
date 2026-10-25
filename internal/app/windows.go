package app

import (
	"errors"
	"os"
	"path/filepath"

	"givens/internal/assert"
)

// InstallCommand is "Install givens command" in the Givens menu: it links the givens command
// into binDir, a folder on the user's PATH, and returns what Givens says, naming where it
// went. The Swift UI passes the folder it picked; tests pass a temporary one.
// Needed by "Install the givens command".
// Ref: https://pkg.go.dev/os#Symlink
func (a *App) InstallCommand(binDir string) (string, error) {
	assert.That(a != nil, "install runs on an app")
	assert.That(filepath.IsAbs(binDir), "the install folder is an absolute path")
	assert.That(len(binDir) <= PathLengthMax, "the path fits PathLengthMax")
	command, err := a.givensCommand()
	if err != nil {
		return "", err
	}
	installed := filepath.Join(binDir, "givens")
	if err := os.Remove(installed); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.Symlink(command, installed); err != nil {
		return "", err
	}
	return "Installed givens in " + binDir + ". Open a new terminal window to use it.", nil
}

// givensCommand is the givens command: Config.Command, or else the running program, which is
// the givens binary under serve.
func (a *App) givensCommand() (string, error) {
	assert.That(a != nil, "givens command runs on an app")
	assert.That(len(a.config.Command) <= PathLengthMax, "the path fits PathLengthMax")
	if a.config.Command != "" {
		return a.config.Command, nil
	}
	// Ref: https://pkg.go.dev/os#Executable
	return os.Executable()
}
