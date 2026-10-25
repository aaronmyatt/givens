package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"givens/internal/assert"
	"givens/internal/helpers"
	"givens/internal/markdown"
	"givens/internal/setup"
)

// A setup card whose code block's info string is "bash run" or "js run" runs by itself when
// Drop in reaches it: a bash block with the kit loaded, a js block in the Givens browser
// through in_browser. Each runs as a script of Givens's own, so it's polled, stopped and
// timed like setup. See capabilities/mix-shell-browser-and-hand-steps.

// cardOutputLinesMax bounds what one card keeps of its blocks' output.
const cardOutputLinesMax = 1000

// blockRun is the run block running now, and those of its card still to run after it.
type blockRun struct {
	card    int   // the card's index in Session.cards
	waiting []int // the card's run blocks still to run, as indices into its Blocks
	process *setup.Process
	parsed  int // output lines already moved into the card
}

// runLanguage says how a run block runs: "bash", "js", or "" when it doesn't run.
func runLanguage(block markdown.Block) string {
	assert.That(len(block.Code) <= markdown.TextBytesMax, "a block stays bounded")
	assert.That(block.Line >= 0, "a block has a line")
	return setup.BlockLanguage(block.Language, block.Run, block.Closed)
}

// runBlocksIn reads a markdown setup's run blocks' code, a js block as the in_browser line it
// runs as, so Drop in's checks read them as a script's lines. Empty when nothing runs.
func runBlocksIn(path string) []string {
	assert.That(path == "" || filepath.IsAbs(path), "a setup file is absolute")
	assert.That(markdown.StepsMax > 0, "steps have a limit")
	texts := make([]string, 0, 2)
	if path == "" {
		return texts
	}
	text, err := helpers.ReadScript(path)
	if err != nil {
		return texts
	}
	parsed, err := markdown.Parse(text)
	if err != nil {
		return texts
	}
	for _, step := range parsed.Steps {
		for _, block := range step.Blocks {
			switch runLanguage(block) {
			case "bash":
				texts = append(texts, block.Code)
			case "js":
				texts = append(texts, "in_browser js")
			}
		}
	}
	return texts
}

// runNextCard moves on through the markdown cards: the first one not yet marked runs its
// blocks if it has any, or else setup waits on it. Once every card is marked, the checklist
// shows.
func (s *Session) runNextCard() error {
	assert.That(s.markdownFrom >= 0, "markdown cards were added")
	assert.That(s.block == nil, "one card runs at a time")
	for index := s.markdownFrom; index < len(s.cards); index++ {
		if s.cards[index].Mark != "" {
			continue
		}
		if waiting := s.cards[index].runBlocks; len(waiting) > 0 {
			s.resume()
			return s.startBlock(index, waiting)
		}
		if s.status != SessionWaiting {
			s.status, s.waitingSince = SessionWaiting, s.window.app.config.Clock.Now()
		}
		return nil
	}
	s.resume()
	s.status, s.checklist = SessionReady, checklistOf(s.scenario, s.row)
	return nil
}

// startBlock runs the first of a card's waiting run blocks.
func (s *Session) startBlock(card int, waiting []int) error {
	assert.That(card >= s.markdownFrom && card < len(s.cards), "the card is a markdown card")
	assert.That(len(waiting) > 0, "a block waits to run")
	script, err := s.blockScript(card, waiting[0])
	if err != nil {
		return err
	}
	// GIVENS_SCRIPT_AS makes the kit load the capability's own helpers, as for its .sh.
	stands := strings.TrimSuffix(s.scenario.SetupCards, ".md") + ".sh"
	process, err := setup.Start(setup.Request{
		Script: script, Dir: filepath.Dir(s.window.path),
		Env: append(append(s.env[:0:0], s.env...), setup.ScriptAsEnv+"="+stands),
	})
	if err != nil {
		s.fail("Setup couldn't start: " + err.Error())
		return nil
	}
	s.block = &blockRun{card: card, waiting: waiting[1:], process: process}
	return nil
}

// blockScript writes the script one run block runs as, beside the session's argument files.
func (s *Session) blockScript(card int, block int) (string, error) {
	assert.That(card >= 0 && card < len(s.cards), "the card exists")
	assert.That(block >= 0 && block < len(s.cards[card].Blocks), "the block exists")
	dir, err := s.scratchDir()
	if err != nil {
		return "", err
	}
	code := s.cards[card].Blocks[block]
	language := setup.BlockLanguage(code.Language, code.Run, code.Copyable)
	return setup.WriteBlock(dir, "card-"+strconv.Itoa(card)+"-block-"+strconv.Itoa(block),
		filepath.Join(s.window.path, "_shared", "givens.sh"), language, code.Code)
}

// pollBlock moves the running block's output into its card, and moves on once it ends: to
// the card's next block, to the next card, or to "Setup stopped at step N of M (exit X)".
func (s *Session) pollBlock() {
	assert.That(s.block != nil, "a block is running")
	assert.That(s.block.process != nil, "the block has a process")
	run := s.block
	result, ended := run.process.Wait(time.Millisecond)
	card := &s.cards[run.card]
	lines := run.process.Output()
	for _, line := range lines[run.parsed:] {
		if len(card.Output) < cardOutputLinesMax {
			card.Output = append(card.Output, line)
		}
	}
	run.parsed = len(lines)
	if !ended {
		return
	}
	s.block = nil
	step, steps := run.card-s.markdownFrom+1, len(s.cards)-s.markdownFrom
	switch {
	case errors.Is(result.Err, setup.ErrStopped):
		s.status = SessionStopped
	case result.Err != nil:
		s.fail("Setup failed: " + result.Err.Error())
	case result.ExitCode != 0:
		card.Mark, s.markdownFailed = "failed", true
		s.fail(fmt.Sprintf("Setup stopped at step %d of %d (exit %d)", step, steps,
			result.ExitCode))
	case len(run.waiting) > 0:
		if err := s.startBlock(run.card, run.waiting); err != nil {
			s.fail("Setup couldn't start: " + err.Error())
		}
	default:
		card.Mark = MarkDone
		if err := s.runNextCard(); err != nil {
			s.fail("Setup couldn't start: " + err.Error())
		}
	}
}

// stopBlock stops the running block, if there's one: "Stop", and leaving the Drop in.
func (s *Session) stopBlock() error {
	assert.That(s != nil, "stop block runs on a session")
	assert.That(s.block == nil || s.block.process != nil, "a running block has a process")
	if s.block == nil {
		return nil
	}
	return s.block.process.Stop()
}
