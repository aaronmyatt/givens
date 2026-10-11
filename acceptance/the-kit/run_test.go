package thekit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Limits on a script run, so a kit that waits forever fails the scenario instead of hanging.
const (
	runDurationMax = 10 * time.Second
	// A script whose output hasn't grown for this long, while it's still running, is taken to
	// be waiting for an answer.
	quietDurationMin = 400 * time.Millisecond
	pollInterval     = 50 * time.Millisecond
	waitDelayMax     = time.Second
)

// lockedBuffer collects a child's stdout and stderr while steps read it from another goroutine.
type lockedBuffer struct {
	mutex  sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(chunk []byte) (int, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.buffer.Write(chunk)
}

func (b *lockedBuffer) String() string {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.buffer.String()
}

// runRequest says how to run one command.
type runRequest struct {
	command  string   // run with bash -c in the sample shop's folder
	terminal bool     // run under a pseudo-terminal, as a developer's terminal would
	answers  []string // typed, each followed by Enter, whenever the script waits for input
	env      []string // extra variables, such as GIVENS_QUIET=1
}

// terminalCommand wraps command in script(1), which runs it under a new pseudo-terminal and
// copies our stdin to it, so the kit sees a terminal on stdin, stdout, stderr and /dev/tty.
// Ref: https://man.freebsd.org/cgi/man.cgi?query=script&sektion=1 (macOS's script)
// Ref: https://man7.org/linux/man-pages/man1/script.1.html (util-linux script)
func terminalCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "linux" {
		return exec.CommandContext(ctx, "script", "-qec", "bash -c "+shellQuote(command),
			"/dev/null")
	}
	return exec.CommandContext(ctx, "script", "-q", "/dev/null", "bash", "-c", command)
}

// cleanEnv is this process's environment without any GIVENS_ variable, so a run is "not from
// Givens" unless a step says otherwise.
func cleanEnv() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, variable := range os.Environ() {
		if !strings.HasPrefix(variable, "GIVENS_") {
			env = append(env, variable)
		}
	}
	return env
}

// run runs a request in the sample shop and keeps its output, with colour codes and carriage
// returns removed, and its exit code. It runs in a new session (setsid), so the script has no
// controlling terminal unless request.terminal gives it one: opening /dev/tty fails, as in CI.
// Ref: https://pkg.go.dev/syscall#SysProcAttr
// Ref: https://pkg.go.dev/os/exec#Cmd (WaitDelay bounds a child that keeps the pipes open)
func (s *state) run(request runRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), runDurationMax)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", request.command)
	if request.terminal {
		cmd = terminalCommand(ctx, request.command)
	}
	cmd.Dir = s.Project
	cmd.Env = append(cleanEnv(), request.env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.WaitDelay = waitDelayMax
	output := &lockedBuffer{}
	cmd.Stdout, cmd.Stderr = output, output
	var stdin io.WriteCloser
	if request.terminal {
		pipe, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		stdin = pipe
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	waitErr := s.answer(stdin, output, done, request.answers)
	s.Output = plainText(output.String())
	if ctx.Err() != nil {
		return fmt.Errorf("%q was still running after %s; it printed:\n%s",
			request.command, runDurationMax, s.Output)
	}
	s.ExitCode = cmd.ProcessState.ExitCode()
	var exitErr *exec.ExitError
	if waitErr != nil && !errors.As(waitErr, &exitErr) {
		return waitErr
	}
	return nil
}

// answer types each answer once the script is waiting for input, then waits for it to exit.
// It records whether the script was still running, and waiting, when the first answer was typed.
func (s *state) answer(
	stdin io.Writer, output *lockedBuffer, done chan error, answers []string,
) error {
	s.waited = false
	for index, answer := range answers {
		waiting, waitErr, exited := waitForInput(output, done)
		if exited {
			return waitErr
		}
		if !waiting {
			break
		}
		if index == 0 {
			s.waited = true
		}
		if _, err := io.WriteString(stdin, answer+"\n"); err != nil {
			return err
		}
	}
	return <-done
}

// waitForInput polls until the script has printed something new and then gone quiet while
// still running (it's waiting for input), or until it exits, or until runDurationMax passes.
func waitForInput(output *lockedBuffer, done chan error) (bool, error, bool) {
	start := len(output.String())
	seen, quietSince := start, time.Now()
	polls := int(runDurationMax / pollInterval)
	for poll := 0; poll < polls; poll++ {
		select {
		case err := <-done:
			return false, err, true
		case <-time.After(pollInterval):
		}
		length := len(output.String())
		if length != seen {
			seen, quietSince = length, time.Now()
			continue
		}
		if seen > start && time.Since(quietSince) >= quietDurationMin {
			return true, nil, false
		}
	}
	return false, nil, false
}

// plainText removes ANSI colour and cursor codes and carriage returns: a pseudo-terminal ends
// lines with \r\n, and the kit colours its output in a terminal.
// Ref: https://en.wikipedia.org/wiki/ANSI_escape_code#CSI_(Control_Sequence_Introducer)_sequences
func plainText(text string) string {
	codes := regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*\x07|\x1b[()][0-9A-B]`)
	return strings.ReplaceAll(codes.ReplaceAllString(text, ""), "\r", "")
}

// shellQuote quotes text for bash with single quotes, so any character in it is literal.
// Ref: https://www.gnu.org/software/bash/manual/bash.html#Single-Quotes
func shellQuote(text string) string {
	return "'" + strings.ReplaceAll(text, "'", `'\''`) + "'"
}
