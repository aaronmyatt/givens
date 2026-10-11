package app

import (
	"errors"
	"fmt"
	"path/filepath"

	"givens/internal/assert"
	"givens/internal/fingerprint"
	"givens/internal/runs"
)

// Marks.
const (
	MarkDone = "done"
	MarkPass = "pass"
	MarkFail = "fail"
	// MarkClear takes a mark off, like unticking a When step.
	MarkClear = "clear"
)

// MessageMarkEachCheck is shown when "Record run" comes before every check is marked.
const MessageMarkEachCheck = "Mark each check first"

// Mark marks a checklist item: "done" for Givens and Whens, "pass" or "fail" for Thens.
func (s *Session) Mark(item string, mark string) error {
	assert.That(s != nil, "mark runs on a session")
	assert.That(mark == MarkDone || mark == MarkPass || mark == MarkFail || mark == MarkClear,
		"a mark is done, pass, fail or clear")
	s.poll()
	for index := range s.checklist {
		if s.checklist[index].Text != item {
			continue
		}
		if mark == MarkClear {
			s.checklist[index].Mark = ""
			return nil
		}
		then := s.checklist[index].Kind == "then"
		if then == (mark == MarkDone) {
			return fmt.Errorf("%q can't be marked %q", item, mark)
		}
		s.checklist[index].Mark = mark
		return nil
	}
	return fmt.Errorf("the checklist has no %q: %w", item, ErrNotFound)
}

// SetRunNote sets the note that "Record run" saves with the run. Needed by "Record a run".
func (s *Session) SetRunNote(note string) error {
	assert.That(s != nil, "set note runs on a session")
	assert.That(len(note) <= NameLengthMax, "a note fits NameLengthMax")
	s.note = note
	return nil
}

// RecordRun records the run with note, which may be "": one line in today's file in runs/,
// passing when every Then step passed. Until every check is marked it records nothing and
// shows MessageMarkEachCheck instead.
func (s *Session) RecordRun(note string) error {
	assert.That(s != nil, "record run runs on a session")
	assert.That(len(note) <= NameLengthMax, "a note fits NameLengthMax")
	if s.Status() != SessionReady {
		return errors.New("setup hasn't finished, so there's no run to record")
	}
	if warning := examplesWarning(s.scenario); warning != "" {
		return errors.New(warning)
	}
	result := runs.ResultPass
	for _, item := range s.checklist {
		if item.Mark == "" {
			s.message = MessageMarkEachCheck
			return nil
		}
		if item.Mark == MarkFail {
			result = runs.ResultFail
		}
	}
	if note == "" {
		note = s.note
	}
	window := s.window
	parts, err := fingerprint.Of(window.project, s.scenario, s.row)
	if err != nil {
		return err
	}
	run := runs.Run{
		Time: window.app.config.Clock.Now(), Key: s.key(), Result: result,
		Author: runs.Author(filepath.Dir(window.path)), Values: s.row,
		Fingerprint: parts, Note: note,
	}
	if err := runs.Append(window.path, run); err != nil {
		return err
	}
	s.message, s.recorded = "", true
	return nil
}

// key is the run key this Drop in records against.
func (s *Session) key() string {
	assert.That(s.scenario.Slug != "", "the scenario has a slug")
	assert.That(s.scenario.Capability != "", "the scenario belongs to a capability")
	return runs.Key(s.scenario.Key(), s.examples)
}
