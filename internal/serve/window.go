package serve

import (
	"fmt"
	"path/filepath"
	"time"

	"givens/internal/app"
	"givens/internal/assert"
)

// Window and session methods. Each checks what internal/app asserts, so a bad request is an
// error reply, never a crash.

func selectItem(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "select runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if given.Name == "" || len(given.Name) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: select needs a name", ErrBadRequest)
	}
	return nil, window.Select(given.Name)
}

func choose(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "choose runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if given.Action == "" || len(given.Action) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: choose needs an action", ErrBadRequest)
	}
	return nil, window.Choose(given.Action)
}

func typeText(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "type runs on a window")
	assert.That(len(given.Text) < LineLengthBytesMax, "typed text fits a line")
	return nil, window.Type(given.Text)
}

func press(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "press runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if given.Keys == "" || len(given.Keys) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: press needs keys", ErrBadRequest)
	}
	return nil, window.Press(given.Keys)
}

func capability(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "capability runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if given.Name == "" || len(given.Name) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: capability needs a name", ErrBadRequest)
	}
	return window.Capability(given.Name)
}

func scenario(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "scenario runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if given.Name == "" || len(given.Name) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: scenario needs a name", ErrBadRequest)
	}
	return window.Scenario(given.Name)
}

func dropIn(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "drop in runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if given.Scenario == "" || len(given.Scenario) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: drop in needs a scenario", ErrBadRequest)
	}
	_, err := window.DropIn(given.Scenario)
	return nil, err
}

func openRecent(window *app.Window, given params) (any, error) {
	assert.That(window != nil, "open recent runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	if !filepath.IsAbs(given.Path) || len(given.Path) > app.PathLengthMax {
		return nil, fmt.Errorf("%w: a recent project has an absolute path", ErrBadRequest)
	}
	_, err := window.OpenRecent(given.Path)
	return nil, err
}

func closeWindow(window *app.Window, _ params) (any, error) {
	assert.That(window != nil, "close runs on a window")
	assert.That(window.ID() != "", "a window has an id")
	return nil, window.CloseWindow()
}

func wait(session *app.Session, given params) (any, error) {
	assert.That(session != nil, "wait runs on a session")
	assert.That(session.Status() != "unknown", "a session has a known status")
	seconds, err := secondsOf(given.Seconds, waitSecondsMax)
	if err != nil {
		return nil, err
	}
	session.Wait(time.Duration(seconds) * time.Second)
	return nil, nil
}

func mark(session *app.Session, given params) (any, error) {
	assert.That(session != nil, "mark runs on a session")
	assert.That(session.Status() != "unknown", "a session has a known status")
	if given.Mark != app.MarkDone && given.Mark != app.MarkPass && given.Mark != app.MarkFail &&
		given.Mark != app.MarkClear {
		return nil, fmt.Errorf("%w: a mark is done, pass, fail or clear", ErrBadRequest)
	}
	return nil, session.Mark(given.Item, given.Mark)
}

func recordRun(session *app.Session, given params) (any, error) {
	assert.That(session != nil, "record run runs on a session")
	assert.That(session.Status() != "unknown", "a session has a known status")
	if len(given.Note) > app.NameLengthMax {
		return nil, fmt.Errorf("%w: a note fits %d bytes", ErrBadRequest, app.NameLengthMax)
	}
	return nil, session.RecordRun(given.Note)
}
