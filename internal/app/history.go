package app

import (
	"sort"
	"strings"
	"time"

	"givens/internal/assert"
	"givens/internal/project"
	"givens/internal/runs"
)

// RunsShownMax bounds the runs a scenario's page lists, newest first.
const RunsShownMax = 20

// RunLine is one recorded run as a scenario's page lists it, so what's been tried shows
// before dropping in. Records.swift's RunRecord.
type RunLine struct {
	Result string `json:"result"` // runs.ResultPass or runs.ResultFail
	Text   string `json:"text"`   // "Passed today at 14:05 by Sam", " · card=expired" for a row
	Note   string `json:"note"`   // what was written with the run, or ""
}

// History lists a scenario's recorded runs, newest first, up to RunsShownMax; an outline's
// rows all together. Kept, like statuses, until a file changes; the day is part of the key,
// so "today" turns into "yesterday" on time.
func (w *Window) History(scenario project.Scenario) []RunLine {
	assert.That(w.project != nil, "history is read in a loaded project")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	now := w.app.config.Clock.Now()
	key := now.Format(time.DateOnly) + "|" + scenario.Capability + "/" + scenario.Name
	if kept, ok := w.histories[key]; ok {
		return kept
	}
	if w.histories == nil {
		w.histories = make(map[string][]RunLine, 16)
	}
	lines := []RunLine{}
	if recorded, err := runs.Read(w.project.Root); err == nil && scenario.Slug != "" {
		lines = historyOf(recorded, scenario.Key(), now)
	}
	w.histories[key] = lines
	return lines
}

// historyOf picks out one scenario's runs, by its run key or a row's key under it, newest
// first.
func historyOf(recorded []runs.Run, key string, now time.Time) []RunLine {
	assert.That(strings.Count(key, "/") == 1, "a scenario key is capability/slug")
	assert.That(!now.IsZero(), "now is a time")
	mine := make([]runs.Run, 0, 8)
	for _, run := range recorded {
		if run.Key == key || strings.HasPrefix(run.Key, key+"/") {
			mine = append(mine, run)
		}
	}
	// Ref: https://pkg.go.dev/sort#SliceStable
	sort.SliceStable(mine, func(a, b int) bool { return mine[a].Time.After(mine[b].Time) })
	lines := make([]RunLine, 0, min(len(mine), RunsShownMax))
	for _, run := range mine[:min(len(mine), RunsShownMax)] {
		lines = append(lines, RunLine{Result: run.Result, Text: runText(run, now), Note: run.Note})
	}
	return lines
}

// runText says one run as a person would: "Failed yesterday at 09:12 by Sam · card=expired".
func runText(run runs.Run, now time.Time) string {
	assert.That(!run.Time.IsZero(), "a run has a time")
	assert.That(len(run.Values) <= runs.FieldsMax, "a row stays bounded")
	text := "Failed"
	if run.Result == runs.ResultPass {
		text = "Passed"
	}
	when := run.Time.In(now.Location())
	day := "on " + when.Format(time.DateOnly)
	switch when.Format(time.DateOnly) {
	case now.Format(time.DateOnly):
		day = "today"
	case now.AddDate(0, 0, -1).Format(time.DateOnly):
		day = "yesterday"
	}
	text += " " + day + " at " + when.Format("15:04")
	if run.Author != "" {
		text += " by " + run.Author
	}
	values := make([]string, 0, len(run.Values))
	for _, pair := range run.Values {
		values = append(values, pair.Name+"="+pair.Value)
	}
	if len(values) > 0 {
		text += " · " + strings.Join(values, ", ")
	}
	return text
}

// shownHistory is the runs of the scenario on show, or none.
func (w *Window) shownHistory() []RunLine {
	assert.That(w != nil, "history runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	if w.project == nil || w.selectedScenario == "" {
		return []RunLine{}
	}
	scenario, ok := w.findScenario(w.selectedScenario)
	if !ok {
		return []RunLine{}
	}
	return w.History(scenario)
}
