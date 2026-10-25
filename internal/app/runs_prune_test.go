package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// at is a run time: second s past 09:00 on 2026-10-17, in UTC.
func at(s int) time.Time { return time.Date(2026, time.October, 17, 9, 0, s, 0, time.UTC) }

// record appends a passing run of key at second s, with a file of size bytes when size > 0.
func record(t *testing.T, root string, key string, s int, size int) Run {
	t.Helper()
	run := Run{Time: at(s), Key: key, Result: RunResultPass, Author: "Sam"}
	if err := AppendRun(root, run); err != nil {
		t.Fatal(err)
	}
	if size > 0 {
		folder := AttachedFilesDir(root, key, run.Time)
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
		shot := filepath.Join(folder, "shot.png")
		if err := os.WriteFile(shot, make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return run
}

// seconds lists the seconds of root's runs, newest first.
func seconds(t *testing.T, root string) []int {
	t.Helper()
	all, err := ReadRunsFolder(root)
	if err != nil {
		t.Fatal(err)
	}
	found := make([]int, 0, len(all))
	for _, run := range all {
		found = append(found, run.Time.Second())
	}
	slices.Sort(found)
	slices.Reverse(found)
	return found
}

// TestPruneByRuns keeps the newest 80% of the run limit once it's passed, and leaves a log
// alone that's within it.
func TestPruneByRuns(t *testing.T) {
	root := t.TempDir()
	for s := range 5 {
		record(t, root, "checkout/empty-cart", s, 10)
	}
	if changed, err := PruneRuns(root, RunLimits{Runs: 5, Megabytes: 1}); err != nil || changed {
		t.Fatalf("at the limit, Prune changed %v, %v", changed, err)
	}
	record(t, root, "checkout/empty-cart", 5, 10)
	if changed, err := PruneRuns(root, RunLimits{Runs: 5, Megabytes: 1}); err != nil || !changed {
		t.Fatalf("past the limit, Prune changed %v, %v", changed, err)
	}
	if got := seconds(t, root); !slices.Equal(got, []int{5, 4, 3, 2}) {
		t.Errorf("kept the runs at seconds %v, not the newest 4", got)
	}
	for _, s := range []int{0, 1} {
		if _, err := os.Stat(AttachedFilesDir(root, "checkout/empty-cart", at(s))); err == nil {
			t.Errorf("the run at second %d kept its files", s)
		}
	}
	if _, err := os.Stat(AttachedFilesDir(root, "checkout/empty-cart", at(2))); err != nil {
		t.Errorf("a kept run lost its files: %v", err)
	}
}

// TestPruneByMegabytes stops at 80% of the MB limit, whichever limit comes first.
func TestPruneByMegabytes(t *testing.T) {
	root := t.TempDir()
	for s := range 4 {
		record(t, root, "checkout/empty-cart", s, 300<<10)
	}
	// 4 runs of 300 KB pass 1 MB; 80% of it, 819 KB, holds 2 of them.
	if changed, err := PruneRuns(root, RunLimits{Runs: 1000, Megabytes: 1}); err != nil ||
		!changed {
		t.Fatalf("past the MB limit, Prune changed %v, %v", changed, err)
	}
	if got := seconds(t, root); !slices.Equal(got, []int{3, 2}) {
		t.Errorf("kept the runs at seconds %v, not the newest 2", got)
	}
}

// TestPruneKeepsEachScenariosNewest keeps an old scenario's only run line, without its files,
// so it isn't "Never run" after a trim.
func TestPruneKeepsEachScenariosNewest(t *testing.T) {
	root := t.TempDir()
	record(t, root, "checkout/card-declined", 0, 10)
	for s := 1; s <= 5; s++ {
		record(t, root, "checkout/empty-cart", s, 10)
	}
	if _, err := PruneRuns(root, RunLimits{Runs: 5, Megabytes: 1}); err != nil {
		t.Fatal(err)
	}
	if got := seconds(t, root); !slices.Equal(got, []int{5, 4, 3, 2, 0}) {
		t.Errorf("kept the runs at seconds %v; want the newest 4 and card-declined's", got)
	}
	if _, err := os.Stat(AttachedFilesDir(root, "checkout/card-declined", at(0))); err == nil {
		t.Error("card-declined's old run kept its files")
	}
}

// TestPruneKeepsUnreadableLines leaves a line it can't read where it was.
func TestPruneKeepsUnreadableLines(t *testing.T) {
	root := t.TempDir()
	for s := range 6 {
		record(t, root, "checkout/empty-cart", s, 0)
	}
	path := filepath.Join(RunsDir(root), "2026-10-17.log")
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(text, "not a run\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := PruneRuns(root, RunLimits{Runs: 5, Megabytes: 1}); err != nil {
		t.Fatal(err)
	}
	lines, err := ReadDayLogLines(path)
	if err != nil || len(lines) != 5 || lines[4] != "not a run" {
		t.Errorf("after the trim the log is %q, %v", lines, err)
	}
}

// TestLimits reads a limits file, defaults a missing one, and refuses a bad one.
func TestLimits(t *testing.T) {
	root := t.TempDir()
	if limits, err := ReadRunLimits(root); err != nil || limits != DefaultRunLimits() {
		t.Errorf("with no file, ReadLimits = %+v, %v", limits, err)
	}
	if err := WriteRunLimits(root, RunLimits{Runs: 5, Megabytes: 2}); err != nil {
		t.Fatal(err)
	}
	if limits, err := ReadRunLimits(root); err != nil ||
		limits != (RunLimits{Runs: 5, Megabytes: 2}) {
		t.Errorf("ReadLimits = %+v, %v", limits, err)
	}
	path := filepath.Join(root, RunLimitsFileName)
	for _, bad := range []string{"runs 0\n", "runs many\n", "days 3\n"} {
		if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadRunLimits(root); err == nil {
			t.Errorf("ReadLimits took %q", bad)
		}
	}
}

// TestMergeLines keeps remote's lines and local's new ones, and doesn't bring back a line
// remote dropped.
func TestMergeLines(t *testing.T) {
	base := []string{"a", "b"}
	local := []string{"a", "b", "c"}
	remote := []string{"b", "d"}
	if got := MergeRunLines(base, local, remote); !slices.Equal(got, []string{"b", "d", "c"}) {
		t.Errorf("MergeLines = %q", got)
	}
}

// TestReadWithRefCopy reads runs from runs/ and from the ref's copy in the git dir, once each,
// and finds a run's files in the copy. A worktree's .git file leads to the shared git dir.
func TestReadWithRefCopy(t *testing.T) {
	repo := t.TempDir()
	capabilities := filepath.Join(repo, "capabilities")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	copied := RunsRefCopy(capabilities)
	if want := filepath.Join(repo, ".git", "givens", "tree"); copied != want {
		t.Fatalf("RefCopy = %s, want %s", copied, want)
	}
	both := record(t, capabilities, "checkout/empty-cart", 1, 0)
	record(t, copied, "checkout/empty-cart", 1, 0)
	inRef := record(t, copied, "checkout/card-declined", 2, 10)
	all, err := ReadRuns(capabilities)
	if err != nil || len(all) != 2 {
		t.Fatalf("Read = %+v, %v; want each run once", all, err)
	}
	if got := AttachedFilesFolder(capabilities, inRef); !strings.HasPrefix(got, copied) {
		t.Errorf("FolderOf a run in the ref = %s", got)
	}
	if got := AttachedFilesFolder(capabilities, both); !strings.HasPrefix(got, capabilities) {
		t.Errorf("FolderOf a run without files in the ref = %s", got)
	}
	worktree := filepath.Join(t.TempDir(), "wt")
	gitdir := filepath.Join(repo, ".git", "worktrees", "wt")
	if err := os.MkdirAll(gitdir, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, text := range map[string]string{
		filepath.Join(worktree, ".git"):    "gitdir: " + gitdir + "\n",
		filepath.Join(gitdir, "commondir"): "../..\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := GitCommonDir(filepath.Join(worktree, "capabilities")); got != filepath.Join(repo,
		".git") {
		t.Errorf("a worktree's GitCommonDir = %s", got)
	}
}
