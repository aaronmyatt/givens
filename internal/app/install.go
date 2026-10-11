package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"givens/internal/assert"
)

const (
	// InstallFoldersMax bounds the folders "Install givens command" offers.
	InstallFoldersMax = 16
	// loginPathWait bounds how long the login shell may take to say its PATH.
	loginPathWait = 3 * time.Second
	// writeOK is W_OK from <unistd.h>: access(2) checks the folder can be written to.
	// Ref: https://man.freebsd.org/cgi/man.cgi?query=access&sektion=2
	writeOK = 2
)

// InstallFolders lists the folders "Install givens command" offers: those on the developer's
// PATH that exist and that they can write to, ~/.local/bin and the like first. The Mac app
// starts with launchd's short PATH, so the PATH comes from their login shell.
func (a *App) InstallFolders() []string {
	assert.That(a != nil, "install folders runs on an app")
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	folders := writableFolders(loginPath(), home)
	assert.That(len(folders) <= InstallFoldersMax, "the folders fit InstallFoldersMax")
	return folders
}

// writableFolders keeps the absolute folders in a PATH list that exist and can be written
// to, once each. The usual homes for a command come first, in this order: ~/.local/bin,
// ~/bin, Homebrew's and /usr/local/bin; folders a tool manages, like nvm's, come after.
func writableFolders(pathList string, home string) []string {
	assert.That(len(pathList) <= 1<<16, "a PATH stays bounded")
	assert.That(home == "" || filepath.IsAbs(home), "home is absolute")
	usual := []string{filepath.Join(home, ".local", "bin"), filepath.Join(home, "bin"),
		"/opt/homebrew/bin", "/usr/local/bin"}
	found := make([]string, 0, 16)
	seen := make(map[string]bool, 16)
	for _, folder := range filepath.SplitList(pathList) {
		folder = filepath.Clean(folder)
		if !filepath.IsAbs(folder) || seen[folder] {
			continue
		}
		seen[folder] = true
		info, err := os.Stat(folder)
		if err == nil && info.IsDir() && syscall.Access(folder, writeOK) == nil {
			found = append(found, folder)
		}
	}
	rank := func(folder string) int {
		for index, first := range usual {
			if folder == first && home != "" {
				return index
			}
		}
		return len(usual)
	}
	// A stable sort keeps the rest in PATH order. Ref: https://pkg.go.dev/sort#SliceStable
	sort.SliceStable(found, func(a, b int) bool { return rank(found[a]) < rank(found[b]) })
	return found[:min(len(found), InstallFoldersMax)]
}

// loginPath asks the developer's shell for its PATH, as a new terminal window would have it,
// with this process's PATH after it. -i reads ~/.zshrc too, where most PATH changes live; the
// marker skips anything the shell's startup files print.
// Ref: https://pkg.go.dev/os/exec#CommandContext
// Ref: https://zsh.sourceforge.io/Doc/Release/Files.html#Startup_002fShutdown-Files
func loginPath() string {
	shell := os.Getenv("SHELL")
	if !filepath.IsAbs(shell) {
		shell = "/bin/zsh"
	}
	assert.That(filepath.IsAbs(shell), "the shell is an absolute path")
	ctx, cancel := context.WithTimeout(context.Background(), loginPathWait)
	defer cancel()
	out, err := exec.CommandContext(ctx, shell, "-l", "-i", "-c",
		`printf '\ngivens-path:%s' "$PATH"`).Output()
	_, path, found := strings.Cut(string(out), "givens-path:")
	if err != nil || !found {
		return os.Getenv("PATH")
	}
	assert.That(found, "the shell said its PATH")
	return strings.TrimSpace(path) + string(filepath.ListSeparator) + os.Getenv("PATH")
}
