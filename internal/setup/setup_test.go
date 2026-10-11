package setup

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// script writes body to a .sh file in a temp folder and returns its request.
func script(t *testing.T, body string) Request {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "setup.sh")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return Request{Script: path, Dir: dir, Env: []string{"GIVENS_QUIET=1"}}
}

func TestOutputAndExitCode(t *testing.T) {
	process, err := Start(script(t, "echo one\necho \"two $GIVENS_QUIET\" >&2\nexit 3\n"))
	if err != nil {
		t.Fatal(err)
	}
	result, ended := process.Wait(10 * time.Second)
	if !ended || result.ExitCode != 3 || result.Err != nil {
		t.Fatalf("result = %+v, ended %v", result, ended)
	}
	if got := process.Output(); !slices.Equal(got, []string{"one", "two 1"}) {
		t.Errorf("output = %q", got)
	}
}

func TestAnswer(t *testing.T) {
	process, err := Start(script(t, "read -r name\necho \"hello $name\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Answer("Sam"); err != nil {
		t.Fatal(err)
	}
	if result, ended := process.Wait(10 * time.Second); !ended || result.ExitCode != 0 {
		t.Fatalf("result = %+v, ended %v", result, ended)
	}
	if got := process.Output(); !slices.Equal(got, []string{"hello Sam"}) {
		t.Errorf("output = %q", got)
	}
}

// TestStopEndsTheGroup stops a script whose child would outlive it, and keeps its output.
func TestStopEndsTheGroup(t *testing.T) {
	request := script(t, "echo started\nsleep 30 &\nwait\n")
	process, err := Start(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, ended := process.Wait(300 * time.Millisecond); ended {
		t.Fatal("the script ended by itself")
	}
	began := time.Now()
	if err := process.Stop(); err != nil {
		t.Fatal(err)
	}
	result, ended := process.Wait(time.Second)
	if !ended || result.Err != ErrStopped || time.Since(began) > 5*time.Second {
		t.Fatalf("result = %+v, ended %v, took %v", result, ended, time.Since(began))
	}
	if got := process.Output(); !slices.Equal(got, []string{"started"}) {
		t.Errorf("output = %q", got)
	}
}

// TestBackgroundChildDoesNotHang ends a setup whose leftover child holds the output open.
func TestBackgroundChildDoesNotHang(t *testing.T) {
	process, err := Start(script(t, "sleep 30 &\necho done\n"))
	if err != nil {
		t.Fatal(err)
	}
	result, ended := process.Wait(5 * time.Second)
	if !ended || result.ExitCode != 0 {
		t.Fatalf("result = %+v, ended %v", result, ended)
	}
}
