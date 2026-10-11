package app

import (
	"time"

	"givens/internal/assert"
	"givens/internal/fingerprint"
	"givens/internal/project"
	"givens/internal/runs"
)

// Scenario statuses, as the library shows them.
const (
	StatusNeverRun      = "Never run"
	StatusLastRunFailed = "Last run failed"
	statusVerified      = "Verified"
	statusUnverified    = "Unverified: "
)

// statusOf says where a scenario stands, by the app's clock.
func (w *Window) statusOf(scenario project.Scenario) string {
	assert.That(w.project != nil, "status is read in a loaded project")
	assert.That(scenario.Capability != "", "the scenario belongs to a capability")
	return StatusOf(w.project, scenario, w.app.config.Clock.Now())
}

// StatusOf says where a scenario stands: verified while its latest run passed and nothing its
// setup depends on has changed since. The givens command uses it too. See capabilities/views
// and capabilities/share-setup-between-scenarios.
func StatusOf(loaded *project.Project, scenario project.Scenario, now time.Time) string {
	assert.That(loaded != nil, "status is read in a loaded project")
	assert.That(!now.IsZero(), "now is a time")
	if scenario.Slug == "" {
		return StatusNeverRun
	}
	recorded, err := runs.Read(loaded.Root)
	if err != nil {
		return StatusNeverRun
	}
	latest, ok := runs.Latest(recorded, scenario.Key(), nil)
	if !ok {
		return StatusNeverRun
	}
	current, err := fingerprint.Of(loaded, scenario, nil)
	if err != nil {
		return StatusNeverRun
	}
	if changed := fingerprint.Changed(latest.Fingerprint, current); changed != "" {
		return statusUnverified + changed + " changed since the last run"
	}
	if latest.Result != runs.ResultPass {
		return StatusLastRunFailed
	}
	return verified(latest, now)
}

// verified says when, and by whom, a passing run was recorded: "Verified today by Sam".
func verified(run runs.Run, now time.Time) string {
	assert.That(run.Result == runs.ResultPass, "only a passing run verifies")
	assert.That(!now.IsZero(), "now is a time")
	day := "on " + run.Time.Format(time.DateOnly)
	switch run.Time.In(now.Location()).Format(time.DateOnly) {
	case now.Format(time.DateOnly):
		day = "today"
	case now.AddDate(0, 0, -1).Format(time.DateOnly):
		day = "yesterday"
	}
	status := statusVerified + " " + day
	if run.Author != "" {
		status += " by " + run.Author
	}
	return status
}
