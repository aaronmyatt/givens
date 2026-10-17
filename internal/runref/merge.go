package runref

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"givens/internal/assert"
	"givens/internal/runs"
)

// merge brings what the remote's ref holds into the copy, three ways: base is the remote's
// ref as this device last saw it. A day's log keeps the remote's lines and this device's new
// ones (runs.MergeLines). A file the remote added comes in; one it removed since base goes,
// so a trim on another device isn't undone here; one only this device has stays. The
// remote's limits file wins.
func (s Store) merge(base string, remote string) error {
	assert.That(s.tree != "", "the store was opened")
	assert.That(remote != base || remote != "", "there's something to merge")
	remoteFiles, err := s.files(remote)
	if err != nil {
		return err
	}
	baseFiles, err := s.files(base)
	if err != nil {
		return err
	}
	localFiles, err := s.localFiles()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.tree, 0o755); err != nil {
		return err
	}
	if err := s.mergeLogs(base, remote, localFiles, remoteFiles); err != nil {
		return err
	}
	incoming := make([]string, 0, 16)
	for name := range remoteFiles {
		if isLog(name) {
			continue
		}
		if name == runs.LimitsFileName || (!localFiles[name] && !baseFiles[name]) {
			incoming = append(incoming, name)
		}
	}
	for name := range localFiles {
		if !isLog(name) && !remoteFiles[name] && baseFiles[name] {
			if err := s.removeFile(name); err != nil {
				return err
			}
		}
	}
	return s.checkout(remote, incoming)
}

// isLog is true for a day's run log in the copy: runs/YYYY-MM-DD.log.
func isLog(name string) bool {
	assert.That(name != "", "a file has a name")
	assert.That(!strings.HasPrefix(name, "/"), "names are relative to the copy")
	return path.Dir(name) == "runs" && strings.HasSuffix(name, ".log")
}

// mergeLogs writes each day's log merged three ways, or removes it when no line is left.
func (s Store) mergeLogs(base string, remote string, localFiles map[string]bool,
	remoteFiles map[string]bool) error {
	assert.That(remote != "", "a merge has a remote")
	assert.That(len(localFiles) <= treeFilesMax, "files stay bounded")
	names := make([]string, 0, 16)
	for _, files := range []map[string]bool{localFiles, remoteFiles} {
		for name := range files {
			if isLog(name) && !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
	}
	remoteLogs, err := s.catFiles(remote, names)
	if err != nil {
		return err
	}
	baseLogs, err := s.catFiles(base, names)
	if err != nil {
		return err
	}
	for _, name := range names {
		local := []string{}
		if localFiles[name] {
			local, err = runs.ReadLines(filepath.Join(s.tree, filepath.FromSlash(name)))
			if err != nil {
				return err
			}
		}
		merged := runs.MergeLines(linesOf(baseLogs[name]), local, linesOf(remoteLogs[name]))
		if err := s.writeLog(name, merged); err != nil {
			return err
		}
	}
	return nil
}

// linesOf splits a log's text into its lines, blank ones left out.
func linesOf(text []byte) []string {
	assert.That(len(text) <= outputBytesMax, "a log fits outputBytesMax")
	lines := make([]string, 0, bytes.Count(text, []byte("\n"))+1)
	for _, line := range strings.Split(string(text), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	assert.That(len(lines) <= len(text)+1, "lines come from the text")
	return lines
}

// writeLog writes a day's log in the copy, or removes it when no line is left.
func (s Store) writeLog(name string, lines []string) error {
	assert.That(isLog(name), "a day's log is written")
	assert.That(len(lines) <= treeFilesMax*runs.FieldsMax, "lines stay bounded")
	if len(lines) == 0 {
		return s.removeFile(name)
	}
	target := filepath.Join(s.tree, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// files lists the files in rev's tree, by their paths in it, or none for "".
// Ref: https://git-scm.com/docs/git-ls-tree
func (s Store) files(rev string) (map[string]bool, error) {
	assert.That(!strings.ContainsAny(rev, " \n"), "a rev is one commit")
	assert.That(s.gitDir != "", "the store was opened")
	found := make(map[string]bool, 64)
	if rev == "" {
		return found, nil
	}
	output, err := s.git(localTimeout, "", "ls-tree", "-r", "-z", "--name-only", rev)
	if err != nil {
		return nil, err
	}
	for _, name := range strings.Split(output, "\x00") {
		if name != "" {
			found[name] = true
		}
	}
	if len(found) > treeFilesMax {
		return nil, fmt.Errorf("%s holds %d files, more than %d", Ref, len(found), treeFilesMax)
	}
	return found, nil
}

// localFiles lists the files in the copy, by their paths in it.
// Ref: https://pkg.go.dev/path/filepath#WalkDir
func (s Store) localFiles() (map[string]bool, error) {
	assert.That(s.tree != "", "the store was opened")
	found := make(map[string]bool, 64)
	err := filepath.WalkDir(s.tree, func(at string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if len(found) > treeFilesMax {
			return fmt.Errorf("%s holds more than %d files", s.tree, treeFilesMax)
		}
		if entry.Type().IsRegular() {
			relative, err := filepath.Rel(s.tree, at)
			if err != nil {
				return err
			}
			found[filepath.ToSlash(relative)] = true
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return found, nil
	}
	assert.That(len(found) <= treeFilesMax+1, "files stay bounded")
	return found, err
}

// catFiles reads the named files of rev's tree in one git call; a name rev doesn't have is
// left out. Ref: https://git-scm.com/docs/git-cat-file#_batch_output
func (s Store) catFiles(rev string, names []string) (map[string][]byte, error) {
	assert.That(len(names) <= treeFilesMax, "names stay bounded")
	assert.That(!strings.ContainsAny(rev, " \n"), "a rev is one commit")
	found := make(map[string][]byte, len(names))
	if rev == "" || len(names) == 0 {
		return found, nil
	}
	var asked strings.Builder
	for _, name := range names {
		asked.WriteString(rev + ":" + name + "\n")
	}
	output, err := s.git(localTimeout, asked.String(), "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(strings.NewReader(output))
	for _, name := range names {
		header, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("git cat-file stopped early: %w", err)
		}
		fields := strings.Fields(header)
		if len(fields) != 3 {
			continue // "<rev>:<name> missing"
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 || size > outputBytesMax {
			return nil, fmt.Errorf("git cat-file said %q", strings.TrimSpace(header))
		}
		content := make([]byte, size+1) // the content, then a newline
		if _, err := io.ReadFull(reader, content); err != nil {
			return nil, err
		}
		found[name] = content[:size]
	}
	return found, nil
}

// checkout writes the named files of rev's tree into the copy, through an index of its own
// so the store's index still says what the copy held.
// Ref: https://git-scm.com/docs/git-checkout-index
func (s Store) checkout(rev string, names []string) error {
	assert.That(rev != "", "files come from a commit")
	assert.That(len(names) <= treeFilesMax, "names stay bounded")
	if len(names) == 0 {
		return nil
	}
	index := filepath.Join(s.dir, "merge-index")
	defer func() { _ = os.Remove(index) }()
	if _, err := s.gitWith(index, localTimeout, "", "read-tree", rev); err != nil {
		return err
	}
	_, err := s.gitWith(index, localTimeout, strings.Join(names, "\n")+"\n", "checkout-index",
		"--force", "--stdin")
	return err
}

// removeFile removes a file from the copy, then each folder it leaves empty.
func (s Store) removeFile(name string) error {
	assert.That(name != "" && !strings.HasPrefix(name, "/"), "names are relative to the copy")
	assert.That(!strings.Contains(name, ".."), "a name stays inside the copy")
	target := filepath.Join(s.tree, filepath.FromSlash(name))
	if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	parent := filepath.Dir(target)
	// bound: each step goes one folder up, and stops at the copy.
	for parent != s.tree && strings.HasPrefix(parent, s.tree) {
		// Remove fails on a folder that isn't empty, which ends the walk.
		// Ref: https://pkg.go.dev/os#Remove
		if os.Remove(parent) != nil {
			return nil
		}
		parent = filepath.Dir(parent)
	}
	return nil
}
