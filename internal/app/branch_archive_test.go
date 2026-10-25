package app

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

// repository makes a git repository with app/capabilities/a.feature on main, and a branch
// "gift-cards" that adds b.feature. It returns the capabilities folder.
func repository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	capabilities := filepath.Join(root, "app", "capabilities")
	run := func(args ...string) {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = root
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	write := func(name string) {
		t.Helper()
		if err := os.MkdirAll(capabilities, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(capabilities, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "--quiet", "--initial-branch=main")
	run("config", "user.name", "Sam")
	run("config", "user.email", "sam@example.com")
	write("a.feature")
	run("add", "--all")
	run("commit", "--quiet", "--message", "a")
	run("checkout", "--quiet", "-b", "gift-cards")
	write("b.feature")
	run("add", "--all")
	run("commit", "--quiet", "--message", "b")
	run("checkout", "--quiet", "main")
	return capabilities
}

// TestListAndExtract lists both branches and copies gift-cards' capabilities out, leaving the
// working tree on main as it was.
func TestListAndExtract(t *testing.T) {
	capabilities := repository(t)
	names, err := ListBranches(capabilities)
	if err != nil || !slices.Equal(names, []string{"gift-cards", "main"}) {
		t.Fatalf("List = %q, %v", names, err)
	}
	into := filepath.Join(t.TempDir(), "copy")
	copied, err := ExtractBranch(capabilities, "gift-cards", into)
	if err != nil {
		t.Fatal(err)
	}
	if copied != filepath.Join(into, "app", "capabilities") {
		t.Errorf("Extract = %s", copied)
	}
	if text, err := os.ReadFile(filepath.Join(copied, "b.feature")); string(text) != "b.feature" {
		t.Errorf("the copy's b.feature = %q, %v", text, err)
	}
	if _, err := os.Stat(filepath.Join(capabilities, "b.feature")); !os.IsNotExist(err) {
		t.Errorf("the working tree changed: %v", err)
	}
}

// TestExtractRefuses turns down a missing branch, a name that looks like an option, and a
// branch without the folder.
func TestExtractRefuses(t *testing.T) {
	capabilities := repository(t)
	into := filepath.Join(t.TempDir(), "copy")
	for _, name := range []string{"no-such-branch", "--output=/tmp/x", ""} {
		if _, err := ExtractBranch(capabilities, name, into); !errors.Is(err, ErrNoBranch) {
			t.Errorf("Extract(%q) = %v", name, err)
		}
	}
	other := filepath.Join(filepath.Dir(capabilities), "elsewhere")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractBranch(other, "main", into); !errors.Is(err, ErrNoCapabilities) {
		t.Errorf("Extract of a folder main doesn't have = %v", err)
	}
	if _, err := os.Stat(into); !os.IsNotExist(err) {
		t.Errorf("a failed Extract left %s behind: %v", into, err)
	}
}

// TestUnpackStaysInside refuses an archive entry that climbs out of its folder.
func TestUnpackStaysInside(t *testing.T) {
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	if err := writer.WriteHeader(&tar.Header{Name: "../escape", Typeflag: tar.TypeReg,
		Size: 1, Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	into := filepath.Join(t.TempDir(), "inside")
	if err := unpack(&archive, into); err == nil {
		t.Error("unpack wrote ../escape")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(into), "escape")); !os.IsNotExist(err) {
		t.Errorf("escape was written: %v", err)
	}
}
