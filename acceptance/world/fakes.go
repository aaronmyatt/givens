package world

import (
	"os"
	"path/filepath"
	"time"

	"givens/internal/assert"
)

// Clipboard is a fake system clipboard that holds one text.
type Clipboard struct {
	text string
}

// WriteText replaces the clipboard's text.
func (c *Clipboard) WriteText(text string) error {
	assert.That(c != nil, "write runs on a clipboard")
	assert.That(len(text) < OutputLineMax*64, "clipboard text stays bounded")
	c.text = text
	return nil
}

// ReadText returns the clipboard's text.
func (c *Clipboard) ReadText() (string, error) {
	assert.That(c != nil, "read runs on a clipboard")
	assert.That(len(c.text) < OutputLineMax*64, "clipboard text stays bounded")
	return c.text, nil
}

// Trash is a fake Finder Trash: a folder inside the scenario's folder.
type Trash struct {
	Dir   string
	Moved []string // original paths, in the order they were trashed
}

// Move moves path into the trash folder.
func (t *Trash) Move(path string) error {
	assert.That(t != nil, "move runs on a trash")
	assert.That(t.Dir != "", "the trash has a folder")
	if err := os.MkdirAll(t.Dir, 0o755); err != nil {
		return err
	}
	if err := os.Rename(path, filepath.Join(t.Dir, filepath.Base(path))); err != nil {
		return err
	}
	t.Moved = append(t.Moved, path)
	return nil
}

// Clock is a fake clock, fixed at 09:00 UTC on 10 October 2026 unless a step moves it.
type Clock struct {
	now time.Time
}

// NewClock returns a clock at the scenario's fixed start time.
func NewClock() *Clock {
	start := time.Date(2026, time.October, 10, 9, 0, 0, 0, time.UTC)
	assert.That(!start.IsZero(), "the start time is set")
	assert.That(start.Location() == time.UTC, "the clock runs in UTC")
	return &Clock{now: start}
}

// Now returns the clock's time.
func (c *Clock) Now() time.Time {
	assert.That(c != nil, "now runs on a clock")
	assert.That(!c.now.IsZero(), "the clock was started with NewClock")
	return c.now
}

// Advance moves the clock forward by step.
func (c *Clock) Advance(step time.Duration) {
	assert.That(c != nil, "advance runs on a clock")
	assert.That(step > 0, "time only moves forward")
	c.now = c.now.Add(step)
}
