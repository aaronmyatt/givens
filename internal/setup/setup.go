// Package setup runs one setup script: bash, in its own process group, with GIVENS_*
// variables, its output read line by line, answers written to its stdin, and a stop that ends
// the script and everything it started.
//
// It decides nothing about what a scenario means; internal/app chooses the script, the
// variables and when to show "Still running".
package setup

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"givens/internal/assert"
)

// Limits. Tiger Style: put a limit on everything.
// Ref: https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md#safety
const (
	// DurationMax ends a setup nobody stopped. "Keep waiting" waits up to here.
	DurationMax = time.Hour
	// OutputLinesMax bounds the lines kept; later lines are counted, not kept.
	OutputLinesMax = 10000
	// LineLengthBytesMax bounds one output line; a longer line is cut.
	LineLengthBytesMax = 64 << 10
	// EnvMax bounds the variables a request adds.
	EnvMax = 256
	// stopGrace is how long Stop waits after SIGTERM before SIGKILL.
	stopGrace = 2 * time.Second
	// drainWait is how long output may stay open after the script exits, held by something it
	// left running in the background.
	drainWait = time.Second
)

// ErrStopped is the Result.Err of a setup ended by Stop or by DurationMax.
var ErrStopped = errors.New("setup was stopped")

// Request is one script to run.
type Request struct {
	Script string   // absolute path to a .sh file
	Dir    string   // where it runs: the project's folder
	Env    []string // NAME=value pairs added to Givens's own environment
}

// Result is how a finished setup ended.
type Result struct {
	ExitCode int   // the script's exit code; -1 when it was stopped or killed
	Err      error // nil, ErrStopped, or why it couldn't run to its end
}

// Process is one running setup.
type Process struct {
	command *exec.Cmd
	stdin   *os.File
	done    chan struct{}

	mutex   sync.Mutex
	lines   []string
	dropped int
	stopped bool
	result  Result
}

// Start runs request's script and returns at once. Read its progress with Output and Done.
func Start(request Request) (*Process, error) {
	assert.That(filepath.IsAbs(request.Script), "the script path is absolute")
	assert.That(filepath.IsAbs(request.Dir), "the folder is absolute")
	if len(request.Env) > EnvMax {
		return nil, fmt.Errorf("setup has %d variables, more than %d", len(request.Env), EnvMax)
	}
	command := exec.Command("bash", request.Script)
	command.Dir = request.Dir
	command.Env = append(os.Environ(), request.Env...)
	// Setpgid puts the script in a new process group, so Stop can signal everything it
	// started. Ref: https://pkg.go.dev/syscall#SysProcAttr
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	pipes, err := newPipes()
	if err != nil {
		return nil, err
	}
	command.Stdin, command.Stdout, command.Stderr = pipes.stdinRead, pipes.outWrite,
		pipes.outWrite
	if err := command.Start(); err != nil {
		pipes.closeAll()
		return nil, err
	}
	// The child holds its own copies now; closing ours lets the reader see the end of output.
	_ = pipes.stdinRead.Close()
	_ = pipes.outWrite.Close()
	process := &Process{
		command: command, stdin: pipes.stdinWrite, done: make(chan struct{}),
		lines: make([]string, 0, 64),
	}
	go process.run(pipes.outRead)
	assert.That(command.Process != nil, "a started command has a process")
	return process, nil
}

// pipes are the script's stdin and its combined stdout and stderr.
type pipes struct {
	stdinRead, stdinWrite, outRead, outWrite *os.File
}

// newPipes opens both pipes. Ref: https://pkg.go.dev/os#Pipe
func newPipes() (pipes, error) {
	stdinRead, stdinWrite, err := os.Pipe()
	if err != nil {
		return pipes{}, err
	}
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		_ = stdinRead.Close()
		_ = stdinWrite.Close()
		return pipes{}, err
	}
	assert.That(stdinRead != nil && stdinWrite != nil, "the stdin pipe is open")
	assert.That(outRead != nil && outWrite != nil, "the output pipe is open")
	return pipes{stdinRead, stdinWrite, outRead, outWrite}, nil
}

// closeAll closes every end, for a script that never started.
func (p pipes) closeAll() {
	assert.That(p.stdinRead != nil, "the pipes were opened")
	assert.That(p.outRead != nil, "the pipes were opened")
	for _, file := range []*os.File{p.stdinRead, p.stdinWrite, p.outRead, p.outWrite} {
		_ = file.Close()
	}
}

// run reads output to its end, waits for the script, and closes done.
func (p *Process) run(output *os.File) {
	assert.That(p != nil, "run runs on a process")
	assert.That(output != nil, "the output pipe is open")
	limit := time.AfterFunc(DurationMax, func() { _ = p.Stop() })
	defer limit.Stop()
	drained := make(chan struct{})
	go p.read(output, drained)
	err := p.command.Wait()
	select {
	case <-drained:
	case <-time.After(drainWait):
		// Something the script left running still holds the output open: end it.
		_ = syscall.Kill(-p.command.Process.Pid, syscall.SIGKILL)
		_ = output.Close()
		<-drained
	}
	_ = p.stdin.Close()
	p.finish(err)
}

// read appends each output line until the pipe closes.
func (p *Process) read(output *os.File, drained chan struct{}) {
	assert.That(p != nil, "read runs on a process")
	assert.That(drained != nil, "read signals when it's done")
	defer close(drained)
	defer func() { _ = output.Close() }()
	reader := bufio.NewReaderSize(output, 4096)
	// bound: ReadString returns an error at the end of output, which ends the loop.
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			p.add(strings.TrimRight(line, "\r\n"))
		}
		if err != nil {
			return
		}
	}
}

// add keeps one line, cut to LineLengthBytesMax, while fewer than OutputLinesMax are kept.
func (p *Process) add(line string) {
	assert.That(p != nil, "add runs on a process")
	if len(line) > LineLengthBytesMax {
		line = line[:LineLengthBytesMax]
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if len(p.lines) >= OutputLinesMax {
		p.dropped++
		return
	}
	p.lines = append(p.lines, line)
	assert.That(len(p.lines) <= OutputLinesMax, "kept lines stay bounded")
}

// finish records how the script ended and closes done.
func (p *Process) finish(err error) {
	assert.That(p != nil, "finish runs on a process")
	p.mutex.Lock()
	result := Result{ExitCode: p.command.ProcessState.ExitCode()}
	var exit *exec.ExitError
	switch {
	case p.stopped:
		result = Result{ExitCode: -1, Err: ErrStopped}
	case err != nil && !errors.As(err, &exit):
		result.Err = err
	}
	p.result = result
	p.mutex.Unlock()
	close(p.done)
	assert.That(result.Err != nil || result.ExitCode >= 0, "a script that ran has an exit code")
}
