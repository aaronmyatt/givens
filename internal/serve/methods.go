package serve

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"time"
	"unicode/utf8"

	"givens/internal/app"
	"givens/internal/assert"
)

// Limits on what one request may carry.
const (
	// waitSecondsMax bounds session.wait; the app waits in short steps.
	waitSecondsMax = 600
	// advanceSecondsMax bounds one test.advanceClock: a year.
	advanceSecondsMax = 366 * 24 * 60 * 60
)

// params holds every field any method takes. Swift sends most as strings.
type params struct {
	Window   string           `json:"window"`
	Path     string           `json:"path"`
	Paths    []string         `json:"paths"`
	BinDir   string           `json:"binDir"`
	Name     string           `json:"name"`
	Action   string           `json:"action"`
	Text     string           `json:"text"`
	Keys     string           `json:"keys"`
	Scenario string           `json:"scenario"`
	Item     string           `json:"item"`
	Mark     string           `json:"mark"`
	Note     string           `json:"note"`
	Seconds  json.RawMessage  `json:"seconds"` // a number, or a number in a string
	Releases []app.KitRelease `json:"releases"`
}

// table maps each method in ServeProtocol.swift to its internal/app call.
func (s *Server) table() map[string]handler {
	assert.That(s != nil, "table runs on a server")
	assert.That(s.core != nil, "the server has an app")
	return map[string]handler{
		"launch":                    s.launch,
		"open":                      s.open,
		"quit":                      s.quit,
		"openDropped":               s.openDropped,
		"installCommand":            s.installCommand,
		"window.select":             s.onWindow(selectItem),
		"window.choose":             s.onWindow(choose),
		"window.type":               s.onWindow(typeText),
		"window.press":              s.onWindow(press),
		"window.capability":         s.onWindow(capability),
		"window.scenario":           s.onWindow(scenario),
		"window.dropIn":             s.onWindow(dropIn),
		"window.openRecent":         s.onWindow(openRecent),
		"window.close":              s.onWindow(closeWindow),
		"session.wait":              s.onSession(wait),
		"session.mark":              s.onSession(mark),
		"session.recordRun":         s.onSession(recordRun),
		seamPrefix + "shipKits":     s.shipKits,
		seamPrefix + "advanceClock": s.advanceClock,
	}
}

// onWindow finds the window a request names, then runs call on it.
func (s *Server) onWindow(call func(*app.Window, params) (any, error)) handler {
	assert.That(s != nil, "on window runs on a server")
	assert.That(call != nil, "a window method has a call")
	return func(given params) (any, error) {
		for _, window := range s.core.Windows() {
			if window.ID() == given.Window {
				return call(window, given)
			}
		}
		return nil, fmt.Errorf("%w: no window %q", ErrBadRequest, given.Window)
	}
}

// onSession finds the Drop in open in the window a request names, then runs call on it.
func (s *Server) onSession(call func(*app.Session, params) (any, error)) handler {
	assert.That(s != nil, "on session runs on a server")
	assert.That(call != nil, "a session method has a call")
	return s.onWindow(func(window *app.Window, given params) (any, error) {
		session := window.Session()
		if session == nil {
			return nil, fmt.Errorf("%w: window %q has no Drop in open", ErrBadRequest,
				given.Window)
		}
		return call(session, given)
	})
}

func (s *Server) launch(params) (any, error) {
	assert.That(s != nil, "launch runs on a server")
	assert.That(s.core != nil, "the server has an app")
	return nil, s.core.Launch()
}

func (s *Server) quit(params) (any, error) {
	assert.That(s != nil, "quit runs on a server")
	assert.That(s.core != nil, "the server has an app")
	return nil, s.core.Quit()
}

func (s *Server) open(given params) (any, error) {
	assert.That(s != nil, "open runs on a server")
	assert.That(s.core != nil, "the server has an app")
	if err := absolute(given.Path); err != nil {
		return nil, err
	}
	_, err := s.core.Open(given.Path)
	return nil, err
}

func (s *Server) openDropped(given params) (any, error) {
	assert.That(s != nil, "open dropped runs on a server")
	assert.That(s.core != nil, "the server has an app")
	if len(given.Paths) == 0 || len(given.Paths) > app.DroppedPathsMax {
		return nil, fmt.Errorf("%w: a drop holds 1 to %d folders", ErrBadRequest,
			app.DroppedPathsMax)
	}
	for _, path := range given.Paths {
		if err := absolute(path); err != nil {
			return nil, err
		}
	}
	return nil, s.core.OpenDropped(given.Paths)
}

func (s *Server) installCommand(given params) (any, error) {
	assert.That(s != nil, "install runs on a server")
	assert.That(s.core != nil, "the server has an app")
	if err := absolute(given.BinDir); err != nil {
		return nil, err
	}
	return s.core.InstallCommand(given.BinDir)
}

func (s *Server) shipKits(given params) (any, error) {
	assert.That(s != nil, "ship kits runs on a server")
	assert.That(s.core != nil, "the server has an app")
	if len(given.Releases) == 0 || len(given.Releases) > app.KitReleasesMax {
		return nil, fmt.Errorf("%w: ship 1 to %d kits", ErrBadRequest, app.KitReleasesMax)
	}
	for _, release := range given.Releases {
		versioned := len(release.Text) <= app.KitLengthMax &&
			len(release.Text) >= len(app.KitVersionPrefix) &&
			release.Text[:len(app.KitVersionPrefix)] == app.KitVersionPrefix
		if release.Version < 1 || !versioned {
			return nil, fmt.Errorf("%w: kit %d isn't a versioned kit", ErrBadRequest,
				release.Version)
		}
	}
	return nil, s.core.ShipKits(given.Releases)
}

func (s *Server) advanceClock(given params) (any, error) {
	assert.That(s != nil, "advance runs on a server")
	assert.That(s.core != nil, "the server has an app")
	seconds, err := secondsOf(given.Seconds, advanceSecondsMax)
	if err != nil {
		return nil, err
	}
	s.clock.offset += time.Duration(seconds) * time.Second
	return nil, nil
}

// absolute rejects a path the app can't have sent: empty, relative or too long.
func absolute(path string) error {
	assert.That(app.PathLengthMax > 0, "paths have a limit")
	assert.That(len(path) < LineLengthBytesMax, "a path came in one request line")
	if !filepath.IsAbs(path) || len(path) > app.PathLengthMax || !utf8.ValidString(path) {
		return fmt.Errorf("%w: %q isn't an absolute path", ErrBadRequest, path)
	}
	return nil
}

// secondsOf reads a whole number of seconds, sent as a number or as a string, from 1 to max.
func secondsOf(raw json.RawMessage, max int) (int, error) {
	assert.That(max > 0, "seconds have a limit")
	text := string(raw)
	if unquoted, err := strconv.Unquote(text); err == nil {
		text = unquoted
	}
	seconds, err := strconv.Atoi(text)
	if err != nil || seconds < 1 || seconds > max {
		return 0, fmt.Errorf("%w: seconds must be 1 to %d, not %s", ErrBadRequest, max, raw)
	}
	assert.That(seconds >= 1 && seconds <= max, "seconds are in range")
	return seconds, nil
}
