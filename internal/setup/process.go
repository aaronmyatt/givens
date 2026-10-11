package setup

import (
	"strings"
	"syscall"
	"time"

	"givens/internal/assert"
)

// Output is a copy of the lines read so far.
func (p *Process) Output() []string {
	assert.That(p != nil, "output runs on a process")
	p.mutex.Lock()
	defer p.mutex.Unlock()
	lines := make([]string, len(p.lines))
	copy(lines, p.lines)
	assert.That(len(lines) <= OutputLinesMax, "output stays bounded")
	return lines
}

// Dropped counts the lines read after OutputLinesMax were kept.
func (p *Process) Dropped() int {
	assert.That(p != nil, "dropped runs on a process")
	p.mutex.Lock()
	defer p.mutex.Unlock()
	assert.That(p.dropped >= 0, "a count is never negative")
	return p.dropped
}

// Done is closed once the script has ended and its output is read.
func (p *Process) Done() <-chan struct{} {
	assert.That(p != nil, "done runs on a process")
	assert.That(p.done != nil, "the process was started")
	return p.done
}

// Wait waits up to limit for the script to end. It returns the Result and true when it has,
// or false when it's still running.
func (p *Process) Wait(limit time.Duration) (Result, bool) {
	assert.That(p != nil, "wait runs on a process")
	assert.That(limit > 0, "a wait has a limit")
	select {
	case <-p.done:
		p.mutex.Lock()
		defer p.mutex.Unlock()
		return p.result, true
	case <-time.After(limit):
		return Result{}, false
	}
}

// Answer writes one line to the script's stdin, for a card that asks a question.
func (p *Process) Answer(text string) error {
	assert.That(p != nil, "answer runs on a process")
	assert.That(!strings.Contains(text, "\n"), "an answer is one line")
	_, err := p.stdin.WriteString(text + "\n")
	return err
}

// Stop ends the script and everything it started: SIGTERM to its process group, then SIGKILL
// after a grace period. Output read so far is kept. Stopping a finished script does nothing.
// Ref: https://man7.org/linux/man-pages/man2/kill.2.html (a negative pid names a group)
func (p *Process) Stop() error {
	assert.That(p != nil, "stop runs on a process")
	assert.That(p.command.Process != nil, "the process was started")
	select {
	case <-p.done:
		return nil
	default:
	}
	p.mutex.Lock()
	p.stopped = true
	p.mutex.Unlock()
	group := -p.command.Process.Pid
	if err := syscall.Kill(group, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		return err
	}
	select {
	case <-p.done:
		return nil
	case <-time.After(stopGrace):
	}
	if err := syscall.Kill(group, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		return err
	}
	<-p.done
	return nil
}
