package runs

import (
	"os"
	"path/filepath"
	"strings"

	"givens/internal/assert"
)

// A project can keep its runs off main: in the git ref refs/givens/runs, which internal/runref
// commits, fetches and pushes. On each device the ref is checked out in
// <git common dir>/givens/tree, laid out like a capabilities folder's runs/, so the run log
// and its files are read from there just as from runs/. See capabilities/keep-runs-off-main.

// Where the ref's copy lives inside the git common dir. git ignores folders it doesn't know
// in there, as git-lfs's .git/lfs shows. Ref: https://git-scm.com/docs/gitrepository-layout
const (
	RefDirName  = "givens"
	refTreeName = "tree"
	// gitDepthMax bounds the walk up from a capabilities folder to its repository's root.
	gitDepthMax = 64
	// gitFileBytesMax bounds a .git file or commondir file: one path.
	gitFileBytesMax = 4096
)

// OffMainFileName is the file left in capabilities/runs/ once its runs moved to the ref: it
// says where they went, for anyone who looks.
const OffMainFileName = "off-main.md"

// RefCopy is where this project's refs/givens/runs is checked out:
// <git common dir>/givens/tree, or "" when capabilities isn't in a git repository. It's found
// without running git, so reading runs stays a few file reads.
func RefCopy(capabilities string) string {
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	common := GitCommonDir(capabilities)
	if common == "" {
		return ""
	}
	assert.That(filepath.IsAbs(common), "the git dir is absolute")
	return filepath.Join(common, RefDirName, refTreeName)
}

// GitCommonDir is the .git folder of the repository holding start, or "" when there's none.
// A worktree or submodule has a .git file naming its git dir, whose commondir file names the
// shared one. Ref: https://git-scm.com/docs/gitrepository-layout#_description
func GitCommonDir(start string) string {
	assert.That(filepath.IsAbs(start), "the walk starts at an absolute path")
	dir := filepath.Clean(start)
	for range gitDepthMax {
		dotGit := filepath.Join(dir, ".git")
		info, err := os.Stat(dotGit)
		if err == nil && info.IsDir() {
			return dotGit
		}
		if err == nil && info.Mode().IsRegular() {
			return linkedCommonDir(dotGit)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	return ""
}

// linkedCommonDir follows a .git file ("gitdir: <path>") to its git dir, then that dir's
// commondir file, if it has one, to the shared git dir.
func linkedCommonDir(dotGit string) string {
	assert.That(filepath.Base(dotGit) == ".git", "a .git file is read")
	gitdir, ok := strings.CutPrefix(readSmall(dotGit), "gitdir: ")
	if !ok || gitdir == "" {
		return ""
	}
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(filepath.Dir(dotGit), gitdir)
	}
	common := readSmall(filepath.Join(gitdir, "commondir"))
	if common == "" {
		return filepath.Clean(gitdir)
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(gitdir, common)
	}
	assert.That(filepath.IsAbs(common), "the common dir is absolute")
	return filepath.Clean(common)
}

// readSmall reads a one-line file, trimmed, or "" when it's missing or too big.
func readSmall(path string) string {
	assert.That(filepath.IsAbs(path), "the path is absolute")
	info, err := os.Stat(path)
	if err != nil || info.Size() > gitFileBytesMax {
		return ""
	}
	text, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(text))
}

// FolderOf is the folder holding run's files: in the ref's copy when it's there, or else
// under capabilities/runs/files.
func FolderOf(capabilities string, run Run) string {
	assert.That(run.Key != "", "a run has a key")
	assert.That(!run.Time.IsZero(), "a run has a time")
	if copied := RefCopy(capabilities); copied != "" {
		folder := FilesDir(copied, run.Key, run.Time)
		if _, err := os.Stat(folder); err == nil {
			return folder
		}
	}
	return FilesDir(capabilities, run.Key, run.Time)
}

// MergeLines merges a day's log three ways: remote's lines, then local lines that base didn't
// have, each once. A line base had and remote dropped stays dropped, so a trim on one device
// isn't undone by another.
func MergeLines(base []string, local []string, remote []string) []string {
	assert.That(len(local) <= LineLengthBytesMax, "lines stay bounded")
	assert.That(len(remote) <= LineLengthBytesMax, "lines stay bounded")
	inBase := make(map[string]bool, len(base))
	for _, line := range base {
		inBase[line] = true
	}
	merged := make([]string, 0, len(remote)+len(local))
	seen := make(map[string]bool, len(remote)+len(local))
	for _, line := range remote {
		if line != "" && !seen[line] {
			merged, seen[line] = append(merged, line), true
		}
	}
	for _, line := range local {
		if line != "" && !seen[line] && !inBase[line] {
			merged, seen[line] = append(merged, line), true
		}
	}
	assert.That(len(merged) <= len(remote)+len(local), "a merge adds no line")
	return merged
}
