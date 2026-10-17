package runref

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"givens/internal/runs"
)

// device is one clone of a shared repository, with its capabilities folder and its store.
type device struct {
	root         string
	capabilities string
	store        Store
}

// run runs git in dir for a test, away from the developer's own git config. Only the final
// newline is trimmed: a run line ends in tabs when it has no author or note.
func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimRight(string(output), "\n")
}

// shared makes a bare remote with one commit on main, and n clones of it.
// Ref: https://git-scm.com/docs/git#Documentation/git.txt-GITCONFIGGLOBAL
func shared(t *testing.T, n int) (string, []device) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	remote := filepath.Join(t.TempDir(), "remote.git")
	run(t, filepath.Dir(remote), "init", "--quiet", "--bare", "--initial-branch=main", remote)
	seed := filepath.Join(t.TempDir(), "seed")
	run(t, filepath.Dir(seed), "clone", "--quiet", remote, seed)
	if err := os.MkdirAll(filepath.Join(seed, "capabilities"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seed, "capabilities", "GIVENS.md"), []byte("x"),
		0o644); err != nil {
		t.Fatal(err)
	}
	run(t, seed, "add", ".")
	run(t, seed, "-c", "user.name=Sam", "-c", "user.email=sam@example.com", "commit",
		"--quiet", "-m", "seed")
	run(t, seed, "push", "--quiet", "origin", "main")
	devices := make([]device, 0, n)
	for index := range n {
		root := filepath.Join(t.TempDir(), "clone"+string(rune('a'+index)))
		run(t, filepath.Dir(root), "clone", "--quiet", remote, root)
		capabilities := filepath.Join(root, "capabilities")
		store, ok := Open(capabilities)
		if !ok {
			t.Fatalf("no store for %s", capabilities)
		}
		devices = append(devices, device{root: root, capabilities: capabilities, store: store})
	}
	return remote, devices
}

// recordAt returns a write that records a passing run of key at second s, with a screenshot.
func recordAt(key string, s int) func(string) error {
	return func(tree string) error {
		at := time.Date(2026, time.October, 17, 9, 0, s, 0, time.UTC)
		folder := runs.FilesDir(tree, key, at)
		if err := os.MkdirAll(folder, 0o755); err != nil {
			return err
		}
		shot := []byte("screenshot of " + key + " at " + at.String())
		if err := os.WriteFile(filepath.Join(folder, "shot.png"), shot, 0o644); err != nil {
			return err
		}
		return runs.Append(tree, runs.Run{Time: at, Key: key, Result: runs.ResultPass})
	}
}

// remoteKeys lists, sorted, the run key and second of each run the remote's ref holds.
func remoteKeys(t *testing.T, remote string) []string {
	t.Helper()
	names := run(t, remote, "ls-tree", "-r", "--name-only", Ref)
	found := make([]string, 0, 8)
	for _, name := range strings.Split(names, "\n") {
		if !strings.HasSuffix(name, ".log") {
			continue
		}
		for _, line := range strings.Split(run(t, remote, "show", Ref+":"+name), "\n") {
			if parsed, err := runs.Parse(line); err == nil {
				found = append(found, parsed.Key+"@"+parsed.Time.Format("05"))
			}
		}
	}
	slices.Sort(found)
	return found
}

// TestShareAndFetch records on one device and sees the run, with its file, on another. The
// developer's working tree and branch don't change.
func TestShareAndFetch(t *testing.T) {
	remote, devices := shared(t, 2)
	first, second := devices[0], devices[1]
	if err := first.store.Change(recordAt("checkout/card-declined", 1),
		NetworkTimeout); err != nil {
		t.Fatal(err)
	}
	if got := remoteKeys(t, remote); !slices.Equal(got, []string{"checkout/card-declined@01"}) {
		t.Fatalf("the remote holds %q", got)
	}
	if status := run(t, first.root, "status", "--porcelain"); status != "" {
		t.Errorf("recording changed the working tree:\n%s", status)
	}
	if err := second.store.Change(nil, NetworkTimeout); err != nil {
		t.Fatal(err)
	}
	all, err := runs.Read(second.capabilities)
	if err != nil || len(all) != 1 {
		t.Fatalf("the other device reads %+v, %v", all, err)
	}
	names, err := runs.Files(second.capabilities, all[0])
	if err != nil || !slices.Equal(names, []string{"shot.png"}) {
		t.Errorf("the other device has the files %q, %v", names, err)
	}
	if unshared, err := second.store.Unshared(); err != nil || unshared != 0 {
		t.Errorf("Unshared = %d, %v", unshared, err)
	}
}

// TestTwoDevicesMerge records on both devices, the second without fetching first; the remote
// keeps both runs.
func TestTwoDevicesMerge(t *testing.T) {
	remote, devices := shared(t, 2)
	if err := devices[0].store.Change(recordAt("checkout/card-declined", 1),
		NetworkTimeout); err != nil {
		t.Fatal(err)
	}
	if err := devices[1].store.Change(recordAt("checkout/empty-cart", 2),
		NetworkTimeout); err != nil {
		t.Fatal(err)
	}
	want := []string{"checkout/card-declined@01", "checkout/empty-cart@02"}
	if got := remoteKeys(t, remote); !slices.Equal(got, want) {
		t.Errorf("the remote holds %q, want %q", got, want)
	}
}

// TestUnreachable keeps a run here while the remote is gone, counts it unshared, and shares it
// once the remote is back.
func TestUnreachable(t *testing.T) {
	remote, devices := shared(t, 1)
	store := devices[0].store
	if err := os.Rename(remote, remote+".away"); err != nil {
		t.Fatal(err)
	}
	err := store.Change(recordAt("checkout/empty-cart", 1), NetworkTimeout)
	if !errors.Is(err, ErrUnreachable) {
		t.Fatalf("with the remote gone, Change = %v", err)
	}
	if unshared, err := store.Unshared(); err != nil || unshared != 1 {
		t.Errorf("Unshared = %d, %v; want 1", unshared, err)
	}
	if err := os.Rename(remote+".away", remote); err != nil {
		t.Fatal(err)
	}
	if err := store.Change(nil, NetworkTimeout); err != nil {
		t.Fatal(err)
	}
	if got := remoteKeys(t, remote); !slices.Equal(got, []string{"checkout/empty-cart@01"}) {
		t.Errorf("the remote holds %q", got)
	}
	if unshared, err := store.Unshared(); err != nil || unshared != 0 {
		t.Errorf("after sharing, Unshared = %d, %v", unshared, err)
	}
}

// TestTrim keeps 4 runs of 6 under a limit of 5, drops the others' files from the ref's
// history, and doesn't bring them back from a device that still had them.
func TestTrim(t *testing.T) {
	remote, devices := shared(t, 2)
	first, second := devices[0], devices[1]
	limit := func(tree string) error {
		return runs.WriteLimits(tree, runs.Limits{Runs: 5, Megabytes: 250})
	}
	if err := first.store.Change(limit, NetworkTimeout); err != nil {
		t.Fatal(err)
	}
	for s := 1; s <= 5; s++ {
		if err := first.store.Change(recordAt("checkout/empty-cart", s),
			NetworkTimeout); err != nil {
			t.Fatal(err)
		}
	}
	if err := second.store.Change(nil, NetworkTimeout); err != nil {
		t.Fatal(err)
	}
	if err := first.store.Change(recordAt("checkout/empty-cart", 6), NetworkTimeout); err != nil {
		t.Fatal(err)
	}
	want := []string{"checkout/empty-cart@03", "checkout/empty-cart@04",
		"checkout/empty-cart@05", "checkout/empty-cart@06"}
	if got := remoteKeys(t, remote); !slices.Equal(got, want) {
		t.Fatalf("after the trim the remote holds %q, want %q", got, want)
	}
	objects := run(t, remote, "rev-list", "--objects", Ref)
	if strings.Contains(objects, "T090001Z") || strings.Contains(objects, "T090002Z") {
		t.Errorf("the ref's history still holds the dropped runs' files:\n%s", objects)
	}
	if err := second.store.Change(nil, NetworkTimeout); err != nil {
		t.Fatal(err)
	}
	if got := remoteKeys(t, remote); !slices.Equal(got, want) {
		t.Errorf("a device that had the old runs brought back %q", got)
	}
	all, err := runs.ReadFolder(second.store.Tree())
	if err != nil || len(all) != 4 {
		t.Errorf("the other device keeps %d runs, %v; want 4", len(all), err)
	}
}

// TestAdopt brings runs/ from the working tree into the copy, files and all.
func TestAdopt(t *testing.T) {
	_, devices := shared(t, 1)
	from := devices[0]
	if err := recordAt("checkout/card-declined", 1)(from.capabilities); err != nil {
		t.Fatal(err)
	}
	adopt := func(tree string) error { return Adopt(tree, from.capabilities) }
	if err := from.store.Change(adopt, NetworkTimeout); err != nil {
		t.Fatal(err)
	}
	all, err := runs.ReadFolder(from.store.Tree())
	if err != nil || len(all) != 1 {
		t.Fatalf("the copy holds %+v, %v", all, err)
	}
	if _, err := os.Stat(runs.FilesDir(from.store.Tree(), all[0].Key, all[0].Time)); err != nil {
		t.Errorf("the run's files weren't adopted: %v", err)
	}
}

// TestPushFirstWins pushes from a device whose view of the remote is stale: the push says to
// start over, not that the remote can't be reached.
func TestPushFirstWins(t *testing.T) {
	_, devices := shared(t, 2)
	first, second := devices[0], devices[1]
	if err := second.store.Change(recordAt("checkout/empty-cart", 1), NetworkTimeout); err != nil {
		t.Fatal(err)
	}
	stale, err := second.store.rev(originRef)
	if err != nil || stale == "" {
		t.Fatalf("the second device saw no remote tip: %q, %v", stale, err)
	}
	if err := first.store.Change(recordAt("checkout/card-declined", 2),
		NetworkTimeout); err != nil {
		t.Fatal(err)
	}
	if err := recordAt("checkout/empty-cart", 3)(second.store.Tree()); err != nil {
		t.Fatal(err)
	}
	commit, err := second.store.commit(stale)
	if err != nil {
		t.Fatal(err)
	}
	pushed, err := second.store.push(commit, stale, NetworkTimeout)
	if pushed || err != nil {
		t.Errorf("a push over a stale tip gave %v, %v; want false, nil", pushed, err)
	}
}

// TestNothingToShare syncs a device with no runs against a remote without the ref: nothing
// is made, here or there, as when a project is opened.
func TestNothingToShare(t *testing.T) {
	remote, devices := shared(t, 1)
	store := devices[0].store
	if err := store.Change(nil, NetworkTimeout); err != nil {
		t.Fatal(err)
	}
	if store.Kept() {
		t.Error("a sync with nothing to share made the copy")
	}
	if refs := run(t, remote, "for-each-ref", "refs/givens"); refs != "" {
		t.Errorf("a sync with nothing to share pushed %s", refs)
	}
}
