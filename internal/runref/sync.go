package runref

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"givens/internal/assert"
	"givens/internal/runs"
)

// Locking: one writer at a time, the app or the givens command.
const (
	lockFileName = "lock"
	// lockTriesMax and lockWait bound how long a change waits for another: 10 s.
	lockTriesMax = 100
	lockWait     = 100 * time.Millisecond
	// lockStaleAfter is when a lock left by a Givens that stopped is taken over.
	lockStaleAfter = 10 * time.Minute
)

// Change lets write change the copy (add a run, its files, its note), commits it, and shares
// it: fetch, merge, trim to the limits, push. A nil write only shares. When the remote can't
// be reached, the change stays committed here and the error wraps ErrUnreachable.
func (s Store) Change(write func(tree string) error, timeout time.Duration) error {
	assert.That(s.tree != "", "the store was opened")
	assert.That(timeout > 0, "a change has a time limit")
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	if write != nil {
		if err := os.MkdirAll(s.tree, 0o755); err != nil {
			return err
		}
		if err := write(s.tree); err != nil {
			return err
		}
	}
	for range syncTriesMax {
		shared, err := s.syncOnce(timeout)
		if err != nil || shared {
			return err
		}
	}
	return fmt.Errorf("%s moved on %d times while Givens shared its runs; try again", Ref,
		syncTriesMax)
}

// syncOnce fetches, merges, trims and pushes once. It says false when another device pushed
// first, so the caller starts over from the fetch.
func (s Store) syncOnce(timeout time.Duration) (bool, error) {
	assert.That(timeout > 0, "a sync has a time limit")
	assert.That(s.tree != "", "the store was opened")
	base, err := s.rev(originRef)
	if err != nil {
		return false, err
	}
	// --no-write-fetch-head leaves the developer's FETCH_HEAD as it was.
	// Ref: https://git-scm.com/docs/git-fetch#Documentation/git-fetch.txt---no-write-fetch-head
	if _, err := s.git(timeout, "", "fetch", "--quiet", "--no-tags", "--no-write-fetch-head",
		Remote, fetchRefspec); err != nil {
		return false, s.keepHere(err)
	}
	remote, err := s.rev(originRef)
	if err != nil {
		return false, err
	}
	if remote != "" && remote != base {
		if err := s.merge(base, remote); err != nil {
			return false, err
		}
	}
	// Opening a project syncs: with nothing here and no ref on the remote, there's nothing
	// to share, and no ref is made.
	if local, err := s.localFiles(); err != nil || (remote == "" && len(local) == 0) {
		return err == nil, err
	}
	trimmed, err := s.trim()
	if err != nil {
		return false, err
	}
	// A trim starts the ref's history over, so the dropped files leave it.
	parent := remote
	if trimmed {
		parent = ""
	}
	commit, err := s.commit(parent)
	if err != nil || commit == remote {
		return err == nil, err
	}
	return s.push(commit, remote, timeout)
}

// keepHere commits the copy on this device's own tip, when the remote can't be reached.
func (s Store) keepHere(cause error) error {
	assert.That(cause != nil, "something stopped the sync")
	assert.That(s.tree != "", "the store was opened")
	local, err := s.rev(Ref)
	if err != nil {
		return err
	}
	if _, err := s.commit(local); err != nil {
		return errors.Join(err, cause)
	}
	return fmt.Errorf("%w: %v", ErrUnreachable, cause)
}

// trim writes the default limits file if the copy has none, then trims to the limits.
func (s Store) trim() (bool, error) {
	assert.That(s.tree != "", "the store was opened")
	if _, err := os.Stat(filepath.Join(s.tree, runs.LimitsFileName)); errors.Is(err,
		fs.ErrNotExist) {
		if err := runs.WriteLimits(s.tree, runs.DefaultLimits()); err != nil {
			return false, err
		}
	}
	limits, err := runs.ReadLimits(s.tree)
	if err != nil {
		return false, err
	}
	assert.That(limits.Runs > 0 && limits.Megabytes > 0, "limits keep something")
	return runs.Prune(s.tree, limits)
}

// push sends commit to the remote's ref. After a trim it replaces what's there, but only if
// that's still remote, the tip this commit was merged with. It says false when another
// device pushed first. Ref: https://git-scm.com/docs/git-push (--force-with-lease)
func (s Store) push(commit string, remote string, timeout time.Duration) (bool, error) {
	assert.That(commit != "", "a commit is pushed")
	assert.That(commit != remote, "a push sends something new")
	// --no-verify: the project's pre-push hooks are for its code, not for runs.
	args := []string{"push", "--quiet", "--no-verify", Remote}
	if remote != "" {
		args = append(args, "--force-with-lease="+Ref+":"+remote)
	}
	if _, err := s.git(timeout, "", append(args, Ref+":"+Ref)...); err != nil {
		message := err.Error()
		if strings.Contains(message, "[rejected]") || strings.Contains(message, "stale info") {
			return false, nil
		}
		return false, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	return true, s.point(originRef, commit)
}

// Unshared counts the run lines in the copy that the remote's ref, as last seen, doesn't
// hold: what this device has yet to share.
func (s Store) Unshared() (int, error) {
	assert.That(s.tree != "", "the store was opened")
	local, err := s.localFiles()
	if err != nil {
		return 0, err
	}
	names := make([]string, 0, 16)
	for name := range local {
		if isLog(name) {
			names = append(names, name)
		}
	}
	origin, err := s.rev(originRef)
	if err != nil {
		return 0, err
	}
	shared, err := s.catFiles(origin, names)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, name := range names {
		lines, err := runs.ReadLines(filepath.Join(s.tree, filepath.FromSlash(name)))
		if err != nil {
			return 0, err
		}
		have := make(map[string]bool, len(lines))
		for _, line := range linesOf(shared[name]) {
			have[line] = true
		}
		for _, line := range lines {
			if !have[line] {
				count++
			}
		}
	}
	assert.That(count >= 0, "a count isn't negative")
	return count, nil
}

// lock takes the store's lock, waiting for another Givens to finish, and returns its release.
// A lock older than lockStaleAfter was left by a Givens that stopped, and is taken over.
// Ref: https://pkg.go.dev/os#O_EXCL
func (s Store) lock() (func(), error) {
	assert.That(s.dir != "", "the store was opened")
	path := filepath.Join(s.dir, lockFileName)
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}
	for range lockTriesMax {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_ = file.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if info, err := os.Stat(path); err == nil && time.Since(info.ModTime()) > lockStaleAfter {
			_ = os.Remove(path)
			continue
		}
		time.Sleep(lockWait)
	}
	assert.That(lockTriesMax > 0, "the lock was tried")
	return nil, fmt.Errorf("%s is held by another Givens", path)
}

// Adopt brings a capabilities folder's runs/ into the copy at tree: each day's lines, merged
// with what the copy has, and each attached file the copy doesn't have. Used once, when a
// project starts keeping its runs off main.
func Adopt(tree string, capabilities string) error {
	assert.That(filepath.IsAbs(tree), "the copy is absolute")
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	logs, err := filepath.Glob(filepath.Join(runs.Dir(capabilities), "*.log"))
	if err != nil {
		return err
	}
	for _, from := range logs {
		lines, err := runs.ReadLines(from)
		if err != nil {
			return err
		}
		target := filepath.Join(runs.Dir(tree), filepath.Base(from))
		have, err := runs.ReadLines(target)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		merged := runs.MergeLines(nil, lines, have)
		if err := os.WriteFile(target, []byte(strings.Join(merged, "\n")+"\n"), 0o644); err != nil {
			return err
		}
	}
	return adoptFiles(tree, capabilities)
}

// adoptFiles copies each file under capabilities/runs/files into the copy, unless it's there.
func adoptFiles(tree string, capabilities string) error {
	assert.That(filepath.IsAbs(tree), "the copy is absolute")
	assert.That(filepath.IsAbs(capabilities), "capabilities is absolute")
	from := filepath.Join(runs.Dir(capabilities), "files")
	copied := 0
	err := filepath.WalkDir(from, func(at string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.Type().IsRegular() {
			return err
		}
		if copied++; copied > treeFilesMax {
			return fmt.Errorf("%s holds more than %d files", from, treeFilesMax)
		}
		relative, err := filepath.Rel(from, at)
		if err != nil {
			return err
		}
		target := filepath.Join(runs.Dir(tree), "files", relative)
		if _, err := os.Stat(target); err == nil {
			return nil
		}
		return copyFile(at, target)
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// copyFile copies one attached file, which CopyFiles already held to runs.FileBytesMax.
// Ref: https://pkg.go.dev/io#Copy
func copyFile(source string, target string) error {
	assert.That(source != target, "a file isn't copied onto itself")
	assert.That(filepath.IsAbs(target), "the target is absolute")
	if err := runs.CheckFile(source); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, io.LimitReader(in, runs.FileBytesMax)); err != nil {
		return errors.Join(err, out.Close())
	}
	return out.Close()
}
