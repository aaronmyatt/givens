// Package runref keeps a project's runs off main: in the git ref refs/givens/runs, which it
// commits, fetches, merges and pushes itself. The ref is checked out in
// <git common dir>/givens/tree (runs.RefCopy), where internal/runs reads and writes runs as
// it does in capabilities/runs. The developer's working tree, their index and every branch
// are left alone. See capabilities/keep-runs-off-main.
//
// Git's plumbing builds each commit from the copy: `add` into an index of Givens's own, then
// `write-tree` and `commit-tree`.
// Ref: https://git-scm.com/book/en/v2/Git-Internals-Plumbing-and-Porcelain
package runref

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"givens/internal/assert"
	"givens/internal/runs"
)

// The ref, where a fetch keeps the remote's copy of it, and the remote it's shared through.
// A fetch takes refs/givens/* by pattern, so a remote without the ref isn't an error.
// Ref: https://git-scm.com/docs/git-fetch#_configured_remote_tracking_branches
const (
	Ref          = "refs/givens/runs"
	originRef    = "refs/givens-origin/runs"
	fetchRefspec = "+refs/givens/*:refs/givens-origin/*"
	Remote       = "origin"
)

// Limits. Tiger Style: put a limit on everything.
const (
	// localTimeout bounds a git call that doesn't touch the network.
	localTimeout = 20 * time.Second
	// NetworkTimeout bounds a fetch or push: room to send 250 MB the first time.
	NetworkTimeout = 120 * time.Second
	// OpenTimeout bounds the sync a project's opening starts. It runs in the background, but
	// a Record run waits on the store's lock meanwhile, so it stays short.
	OpenTimeout = 8 * time.Second
	// waitDelay bounds how long a git that timed out may hold its output pipes open.
	waitDelay = time.Second
	// syncTriesMax bounds how often a sync starts over after another device pushed first.
	syncTriesMax = 3
	// treeFilesMax bounds the files in the ref: 1000 runs of 33 files would be 33 000.
	treeFilesMax = 200_000
	// outputBytesMax bounds what Givens reads back from one git call: logs, not attachments.
	outputBytesMax = 64 << 20
)

// Who commits to the ref. A run line names who ran it, so the commits needn't; and a
// developer without user.email set can still keep runs.
// Ref: https://git-scm.com/book/en/v2/Git-Internals-Environment-Variables
const (
	committerName  = "Givens"
	committerEmail = "givens@localhost"
)

// ErrUnreachable is returned when the remote can't be reached: the runs stay committed on
// this device until a sync gets through.
var ErrUnreachable = errors.New("the remote can't be reached")

// Store is one repository's runs ref and its checked-out copy.
type Store struct {
	capabilities string // the project's capabilities folder; git runs from here
	gitDir       string // the repository's git common dir
	dir          string // <gitDir>/givens: the copy, its index and the lock
	tree         string // <dir>/tree, the copy: runs.RefCopy(capabilities)
}

// Open finds the store of the repository holding capabilities, or false when it isn't in a
// git repository. Nothing is written until the store is changed.
func Open(capabilities string) (Store, bool) {
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	gitDir := runs.GitCommonDir(capabilities)
	if gitDir == "" {
		return Store{}, false
	}
	store := Store{
		capabilities: capabilities, gitDir: gitDir,
		dir: filepath.Join(gitDir, runs.RefDirName), tree: runs.RefCopy(capabilities),
	}
	assert.That(filepath.Dir(store.tree) == store.dir, "the copy is in the store's folder")
	return store, true
}

// Tree is the ref's copy, laid out like a capabilities folder: runs/ and the limits file.
func (s Store) Tree() string {
	assert.That(s.tree != "", "the store was opened")
	assert.That(filepath.IsAbs(s.tree), "the copy is absolute")
	return s.tree
}

// Kept is true once this device keeps a copy of the ref.
func (s Store) Kept() bool {
	assert.That(s.tree != "", "the store was opened")
	assert.That(filepath.IsAbs(s.tree), "the copy is absolute")
	_, err := os.Stat(s.tree)
	return err == nil
}

// HasRemote is true when the repository has the remote runs are shared through.
func (s Store) HasRemote() bool {
	assert.That(s.gitDir != "", "the store was opened")
	assert.That(Remote != "", "runs are shared through a named remote")
	_, err := s.git(localTimeout, "", "remote", "get-url", Remote)
	return err == nil
}

// git runs git on the store's repository with the store's own index, and, for the commands
// that read or write files, the copy as the work tree: the developer's index and working tree
// aren't touched. It never prompts.
// Ref: https://git-scm.com/docs/git#Documentation/git.txt-GITINDEXFILE
func (s Store) git(timeout time.Duration, stdin string, args ...string) (string, error) {
	assert.That(s.dir != "", "the store was opened")
	assert.That(timeout > 0, "git has a time limit")
	return s.gitWith(filepath.Join(s.dir, "index"), timeout, stdin, args...)
}

// gitWith is git with another index: a merge reads the remote's tree into one of its own.
func (s Store) gitWith(index string, timeout time.Duration, stdin string,
	args ...string) (string, error) {
	assert.That(filepath.IsAbs(index), "the index is absolute")
	assert.That(len(args) > 0, "git has a command")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	full := []string{"--git-dir=" + s.gitDir}
	// git refuses a work tree whose folder isn't there, so only these two are given one; the
	// copy is made before either runs.
	if args[0] == "add" || args[0] == "checkout-index" {
		full = append(full, "--work-tree="+s.tree)
	}
	full = append(full, args...)
	command := exec.CommandContext(ctx, "git", full...)
	command.Dir = s.capabilities
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C",
		"GIT_INDEX_FILE="+index,
		"GIT_AUTHOR_NAME="+committerName, "GIT_AUTHOR_EMAIL="+committerEmail,
		"GIT_COMMITTER_NAME="+committerName, "GIT_COMMITTER_EMAIL="+committerEmail)
	// A timed-out git is killed, but a helper it started, like ssh, can keep the output pipes
	// open; WaitDelay stops waiting for them, so the timeout holds.
	// Ref: https://pkg.go.dev/os/exec#Cmd.WaitDelay
	command.WaitDelay = waitDelay
	command.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	if stdout.Len() > outputBytesMax {
		return "", fmt.Errorf("git %s wrote more than %d bytes", args[0], outputBytesMax)
	}
	return stdout.String(), nil
}

// rev is the commit ref points at, or "" when there's no such ref.
// Ref: https://git-scm.com/docs/git-for-each-ref
func (s Store) rev(ref string) (string, error) {
	assert.That(strings.HasPrefix(ref, "refs/"), "a ref is named in full")
	assert.That(s.gitDir != "", "the store was opened")
	output, err := s.git(localTimeout, "", "for-each-ref", "--format=%(objectname)", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// commit commits the copy as it is, on parent ("" for none), and points Ref at it. It
// returns parent when the copy holds just what parent does.
// Ref: https://git-scm.com/docs/git-commit-tree
func (s Store) commit(parent string) (string, error) {
	assert.That(s.tree != "", "the store was opened")
	assert.That(!strings.ContainsAny(parent, " \n"), "a parent is one commit")
	if err := os.MkdirAll(s.tree, 0o755); err != nil {
		return "", err
	}
	// --force adds what a global ignore, like *.log, would leave out.
	if _, err := s.git(localTimeout, "", "add", "--all", "--force"); err != nil {
		return "", err
	}
	tree, err := s.git(localTimeout, "", "write-tree")
	if err != nil {
		return "", err
	}
	tree = strings.TrimSpace(tree)
	if parent != "" {
		parentTree, err := s.git(localTimeout, "", "rev-parse", parent+"^{tree}")
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(parentTree) == tree {
			return parent, s.point(Ref, parent)
		}
	}
	// --no-gpg-sign: a developer who signs commits isn't asked for a passphrase per run.
	args := []string{"commit-tree", "--no-gpg-sign", "-m", "Givens runs", tree}
	if parent != "" {
		args = append(args, "-p", parent)
	}
	commit, err := s.git(localTimeout, "", args...)
	if err != nil {
		return "", err
	}
	commit = strings.TrimSpace(commit)
	return commit, s.point(Ref, commit)
}

// point sets ref to commit. Ref: https://git-scm.com/docs/git-update-ref
func (s Store) point(ref string, commit string) error {
	assert.That(strings.HasPrefix(ref, "refs/givens"), "only Givens's own refs are set")
	assert.That(commit != "", "a ref points at a commit")
	_, err := s.git(localTimeout, "", "update-ref", ref, commit)
	return err
}
