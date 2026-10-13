// Package branch copies a capabilities folder out of another git branch, without checking that
// branch out, so Givens can read it.
//
// `git archive` streams the folder as it is on the branch as a tar, which is unpacked into a
// folder Givens owns. The working tree, the index and .git are never written to.
// Ref: https://git-scm.com/docs/git-archive
package branch

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"givens/internal/assert"
)

// Limits. Tiger Style: put a limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const (
	// gitTimeout bounds each git call; an archive of a large capabilities folder included.
	gitTimeout = 20 * time.Second
	// ArchiveBytesMax bounds what one branch's capabilities folder unpacks to.
	ArchiveBytesMax = 256 << 20
	// EntriesMax bounds the files and folders one archive holds.
	EntriesMax = 100000
	// BranchesMax bounds the branches List returns.
	BranchesMax = 1000
	// NameLengthMax bounds a branch name; git allows longer, nobody types them.
	NameLengthMax = 256
)

// ErrNoBranch is returned for a branch the repository doesn't have.
var ErrNoBranch = errors.New("no such branch")

// ErrNoCapabilities is returned when the branch has no capabilities folder at that path.
var ErrNoCapabilities = errors.New("no capabilities folder on that branch")

// List returns the repository's local branches, then its remote ones, as git names them:
// "main", "origin/gift-cards". A remote's HEAD is left out: it only points at another.
// Ref: https://git-scm.com/docs/git-for-each-ref
func List(dir string) ([]string, error) {
	assert.That(filepath.IsAbs(dir), "dir is absolute")
	output, err := git(dir, "for-each-ref", "--format=%(refname)", "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, 16)
	for _, ref := range strings.Split(strings.TrimSpace(output), "\n") {
		if ref == "" || strings.HasSuffix(ref, "/HEAD") || len(names) >= BranchesMax {
			continue
		}
		if local, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
			names = append(names, local)
		} else if remote, ok := strings.CutPrefix(ref, "refs/remotes/"); ok {
			names = append(names, remote)
		}
	}
	assert.That(len(names) <= BranchesMax, "branches stay bounded")
	return names, nil
}

// Extract unpacks the capabilities folder at capabilities, as it is on name, into the empty
// folder into, replacing anything there. It returns where the copied capabilities folder is.
func Extract(capabilities string, name string, into string) (string, error) {
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	assert.That(filepath.IsAbs(into), "into is absolute")
	if err := checkName(name); err != nil {
		return "", err
	}
	root, inside, err := locate(capabilities)
	if err != nil {
		return "", err
	}
	// --verify --quiet prints nothing and fails for a name that isn't a commit.
	// Ref: https://git-scm.com/docs/git-rev-parse#Documentation/git-rev-parse.txt---verify
	if _, err := git(root, "rev-parse", "--verify", "--quiet", name+"^{commit}"); err != nil {
		return "", fmt.Errorf("%w: %s", ErrNoBranch, name)
	}
	if err := os.RemoveAll(into); err != nil {
		return "", err
	}
	if err := archiveInto(root, name, inside, into); err != nil {
		return "", errors.Join(err, os.RemoveAll(into))
	}
	copied := filepath.Join(into, filepath.FromSlash(inside))
	if _, err := os.Stat(copied); err != nil {
		return "", errors.Join(fmt.Errorf("%w: %s on %s", ErrNoCapabilities, inside, name),
			os.RemoveAll(into))
	}
	assert.That(strings.HasPrefix(copied, into), "the copy is inside into")
	return copied, nil
}

// archiveInto streams `git archive` of inside on name straight into unpack, so a large
// folder is never held in memory whole. A path the branch doesn't have makes git fail.
// Ref: https://pkg.go.dev/os/exec#Cmd.StdoutPipe
func archiveInto(root string, name string, inside string, into string) error {
	assert.That(filepath.IsAbs(root), "root is absolute")
	assert.That(!strings.HasPrefix(name, "-"), "a name isn't an option")
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "archive", "--format=tar", name, "--", inside)
	command.Dir = root
	var stderr bytes.Buffer
	command.Stderr = &stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	if err := command.Start(); err != nil {
		return err
	}
	unpacked := unpack(stdout, into)
	if unpacked != nil {
		cancel()
	}
	if err := command.Wait(); err != nil && unpacked == nil {
		return fmt.Errorf("%w: %s on %s (%s)", ErrNoCapabilities, inside, name,
			strings.TrimSpace(stderr.String()))
	}
	return unpacked
}

// checkName turns down a branch name git could read as an option, or that's too long.
func checkName(name string) error {
	assert.That(NameLengthMax > 0, "names have a limit")
	if name == "" || len(name) > NameLengthMax || strings.HasPrefix(name, "-") ||
		strings.ContainsAny(name, " \t\n\r\x00") {
		return fmt.Errorf("%w: %q isn't a branch name", ErrNoBranch, name)
	}
	assert.That(!strings.HasPrefix(name, "-"), "a name isn't an option")
	return nil
}

// locate finds the repository holding capabilities, and the capabilities folder's path inside
// it, slash-separated as git archive wants. Symlinks are resolved first: git reports the
// resolved root, like /private/var for /var.
// Ref: https://git-scm.com/docs/git-rev-parse#Documentation/git-rev-parse.txt---show-toplevel
func locate(capabilities string) (string, string, error) {
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	resolved, err := filepath.EvalSymlinks(capabilities)
	if err != nil {
		return "", "", err
	}
	output, err := git(resolved, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", fmt.Errorf("%s isn't in a git repository", capabilities)
	}
	root := strings.TrimSpace(output)
	inside, err := filepath.Rel(root, resolved)
	if err != nil || !filepath.IsLocal(inside) {
		return "", "", fmt.Errorf("%s isn't inside %s", resolved, root)
	}
	assert.That(filepath.IsAbs(root), "git reports an absolute root")
	return root, filepath.ToSlash(inside), nil
}

// unpack writes the folders and regular files of a tar into into. Links and anything else are
// skipped; a path that would land outside into is refused.
// Ref: https://pkg.go.dev/archive/tar#Reader
func unpack(archive io.Reader, into string) error {
	assert.That(filepath.IsAbs(into), "into is absolute")
	reader := tar.NewReader(archive)
	written := int64(0)
	for entries := 0; entries <= EntriesMax; entries++ {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		// IsLocal is false for absolute paths and ones that climb out with "..".
		// Ref: https://pkg.go.dev/path/filepath#IsLocal
		if !filepath.IsLocal(header.Name) {
			return fmt.Errorf("the archive holds %q, outside its folder", header.Name)
		}
		target := filepath.Join(into, filepath.FromSlash(header.Name))
		switch header.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			written += header.Size
			if written > ArchiveBytesMax {
				return fmt.Errorf("the branch's capabilities are over %d MB", ArchiveBytesMax>>20)
			}
			err = writeFile(target, reader, header.Size)
		}
		if err != nil {
			return err
		}
	}
	return fmt.Errorf("the archive holds more than %d entries", EntriesMax)
}

// writeFile writes size bytes from source into a new file at target, making its folder.
func writeFile(target string, source io.Reader, size int64) error {
	assert.That(size >= 0, "a size isn't negative")
	assert.That(size <= ArchiveBytesMax, "a file fits the archive's limit")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.CopyN(file, source, size); err != nil {
		return errors.Join(err, file.Close())
	}
	return file.Close()
}

// git runs git in dir and returns its standard output, away from prompts for credentials.
// Ref: https://git-scm.com/docs/git#Documentation/git.txt-GITTERMINALPROMPT
func git(dir string, args ...string) (string, error) {
	assert.That(filepath.IsAbs(dir), "git runs in an absolute folder")
	assert.That(len(args) > 0, "git has a command")
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
