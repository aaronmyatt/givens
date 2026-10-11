package usegivensfromtheterminal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"givens/acceptance/world"
)

// Limits for driving an interactive setup. A setup that hangs fails the step at the deadline.
const (
	terminalDeadline = 30 * time.Second       // a whole interactive run
	stepAppears      = 10 * time.Second       // waiting for the next step to print
	stillWaiting     = 500 * time.Millisecond // how long a step must stay alone on screen
	pollInterval     = 25 * time.Millisecond
)

// payInAppCommand is the command both markdown scenarios run, from their When step.
const payInAppCommand = "givens run checkout/pay-in-app"

// markdownPath is the markdown setup the common Given step writes.
const markdownPath = "capabilities/checkout/scripts/pay-in-app.md"

// codeBlock is the block a Given step indents under a step of the markdown setup.
const codeBlock = "xcrun simctl openurl booted \"shop://sign-in?email=sam@example.com\"\n" +
	"open -a Simulator"

// lockedBuffer collects a process's output while a step reads it.
type lockedBuffer struct {
	mutex  sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.buffer.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.buffer.String()
}

// terminal is a command running with a keyboard: steps type into its stdin and read its
// output as it prints. Ref: https://pkg.go.dev/os/exec#Cmd.StdinPipe
type terminal struct {
	run    *exec.Cmd
	cancel context.CancelFunc
	stdin  io.WriteCloser
	output *lockedBuffer
	done   chan error
}

func (s *state) startTerminal(command string) (*terminal, error) {
	ctx, cancel := context.WithTimeout(context.Background(), terminalDeadline)
	run := exec.CommandContext(ctx, "bash", "-c", command)
	run.Dir = s.Cwd
	if run.Dir == "" {
		run.Dir = s.Project
	}
	run.Env = append(os.Environ(),
		"PATH="+s.Bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	output := &lockedBuffer{}
	run.Stdout, run.Stderr = output, output
	stdin, err := run.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := run.Start(); err != nil {
		cancel()
		return nil, err
	}
	done := make(chan error, 1)
	go func() { done <- run.Wait() }()
	return &terminal{run: run, cancel: cancel, stdin: stdin, output: output, done: done}, nil
}

// waitFor waits until text has printed, failing if the command exits first or never prints it.
func (t *terminal) waitFor(text string) error {
	ticks := int(stepAppears / pollInterval)
	for tick := 0; tick < ticks; tick++ {
		if strings.Contains(t.output.String(), text) {
			return nil
		}
		if t.exited() {
			return fmt.Errorf("it exited before printing %q:\n%s", text, t.output.String())
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("it didn't print %q within %s:\n%s", text, stepAppears, t.output.String())
}

// holds expects the command to still be running, without having printed next, after a pause.
func (t *terminal) holds(next string) error {
	time.Sleep(stillWaiting)
	if strings.Contains(t.output.String(), next) {
		return fmt.Errorf("it printed %q without waiting for a key:\n%s", next, t.output.String())
	}
	if t.exited() {
		return fmt.Errorf("it exited instead of waiting for a key:\n%s", t.output.String())
	}
	return nil
}

// exited reads done's length rather than run.ProcessState, which Wait sets on another goroutine.
func (t *terminal) exited() bool {
	return len(t.done) > 0
}

func (t *terminal) press(keys string) error {
	_, err := io.WriteString(t.stdin, keys)
	return err
}

// finish waits for the command to exit and returns its exit code.
func (t *terminal) finish() (int, error) {
	defer t.cancel()
	err := <-t.done
	if exitErr := (*exec.ExitError)(nil); errors.As(err, &exitErr) {
		if exitErr.ProcessState.ExitCode() < 0 {
			return -1, fmt.Errorf("it was stopped at the %s deadline:\n%s",
				terminalDeadline, t.output.String())
		}
		return exitErr.ProcessState.ExitCode(), nil
	}
	return 0, err
}

// markdownSteps reads the numbered steps of the markdown setup as the plain text a terminal
// should show: links as their text, inline code without backticks. A link may print its URL
// after its text, so each step is cut at the end of its first link's text.
func (s *state) markdownSteps() ([]string, error) {
	markdown, err := s.ReadFile(markdownPath)
	if err != nil {
		return nil, err
	}
	numbered := regexp.MustCompile(`(?m)^\d+\. (.+)$`)
	link := regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	plain := make([]string, 0, 8)
	for _, match := range numbered.FindAllStringSubmatch(markdown, -1) {
		text := strings.ReplaceAll(match[1], "`", "")
		if found := link.FindStringSubmatchIndex(text); found != nil {
			text = text[:found[0]] + text[found[2]:found[3]]
		}
		plain = append(plain, text)
	}
	if len(plain) == 0 {
		return nil, fmt.Errorf("%s has no numbered steps", markdownPath)
	}
	return plain, nil
}

// walk runs the markdown setup, pressing Enter after each step appears, and keeps the output.
func (s *state) walk() (string, error) {
	if s.walked != "" {
		return s.walked, nil
	}
	plain, err := s.markdownSteps()
	if err != nil {
		return "", err
	}
	run, err := s.startTerminal(payInAppCommand)
	if err != nil {
		return "", err
	}
	for _, step := range plain {
		if err := run.waitFor(step); err != nil {
			run.cancel()
			return "", err
		}
		if err := run.press("\n"); err != nil {
			run.cancel()
			return "", err
		}
	}
	code, err := run.finish()
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("%q exited %d after every step was done:\n%s",
			payInAppCommand, code, run.output.String())
	}
	s.walked = run.output.String()
	return s.walked, nil
}

// codeBlockUnderStep indents a fenced code block under a numbered step of the markdown setup.
// Ref: https://spec.commonmark.org/0.31.2/#fenced-code-blocks
func (s *state) codeBlockUnderStep(step int) error {
	markdown, err := s.ReadFile(markdownPath)
	if err != nil {
		return err
	}
	lines := strings.Split(markdown, "\n")
	at := indexWithPrefix(lines, fmt.Sprintf("%d. ", step))
	if at < 0 {
		return fmt.Errorf("%s has no step %d", markdownPath, step)
	}
	block := []string{"", "   ```sh"}
	for _, line := range strings.Split(codeBlock, "\n") {
		block = append(block, "   "+line)
	}
	block = append(block, "   ```")
	edited := append(append(append([]string{}, lines[:at+1]...), block...), lines[at+1:]...)
	s.codeBlock = strings.Split(codeBlock, "\n")
	return s.WriteFile(markdownPath, strings.Join(edited, "\n"))
}

func indexWithPrefix(lines []string, prefix string) int {
	for index, line := range lines {
		if strings.HasPrefix(line, prefix) {
			return index
		}
	}
	return -1
}

// stepsAsPlainText expects every step's plain text, and none of markdown's punctuation.
func (s *state) stepsAsPlainText() error {
	output, err := s.walk()
	if err != nil {
		return err
	}
	for _, markup := range []string{"`", "](", "![", "**"} {
		if err := world.ExpectNot([]string{output}, markup, "the output"); err != nil {
			return err
		}
	}
	return nil
}

// waitsForEnter checks each step stays alone until Enter, then runs again and answers f at
// the first step: setup stops there, with a non-zero exit code.
func (s *state) waitsForEnter() error {
	plain, err := s.markdownSteps()
	if err != nil {
		return err
	}
	if err := s.holdsAtEachStep(plain); err != nil {
		return err
	}
	run, err := s.startTerminal(payInAppCommand)
	if err != nil {
		return err
	}
	if err := run.waitFor(plain[0]); err != nil {
		run.cancel()
		return err
	}
	if err := run.press("f\n"); err != nil {
		run.cancel()
		return err
	}
	code, err := run.finish()
	if err != nil {
		return err
	}
	if code == 0 || (len(plain) > 1 && strings.Contains(run.output.String(), plain[1])) {
		return fmt.Errorf("answering f at the first step exited %d and printed:\n%s",
			code, run.output.String())
	}
	return nil
}

func (s *state) holdsAtEachStep(plain []string) error {
	run, err := s.startTerminal(payInAppCommand)
	if err != nil {
		return err
	}
	defer run.cancel()
	for index, step := range plain {
		if err := run.waitFor(step); err != nil {
			return err
		}
		if index+1 < len(plain) {
			if err := run.holds(plain[index+1]); err != nil {
				return err
			}
		}
		if err := run.press("\n"); err != nil {
			return err
		}
	}
	code, err := run.finish()
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("pressing Enter at every step exited %d:\n%s", code, run.output.String())
	}
	return nil
}

// codeBlockOnOwnLines expects each line of the block as a whole output line: no fence, no
// indent, nothing beside it.
func (s *state) codeBlockOnOwnLines() error {
	output, err := s.walk()
	if err != nil {
		return err
	}
	if len(s.codeBlock) == 0 {
		return fmt.Errorf("no step added a code block to %s", markdownPath)
	}
	lines := strings.Split(output, "\n")
	for _, want := range s.codeBlock {
		if !hasLine(lines, want) {
			return fmt.Errorf("no output line is exactly %q:\n%s", want, output)
		}
	}
	return world.ExpectNot(lines, "```", "the output")
}

// codeBlockPastable expects the block's lines together, in order, with nothing between them,
// so selecting them copies the block as it was written.
func (s *state) codeBlockPastable() error {
	output, err := s.walk()
	if err != nil {
		return err
	}
	block := "\n" + strings.Join(s.codeBlock, "\n") + "\n"
	if len(s.codeBlock) == 0 || !strings.Contains(output, block) {
		return fmt.Errorf("the output doesn't hold the code block %q as written:\n%s",
			block, output)
	}
	return nil
}

func hasLine(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}
