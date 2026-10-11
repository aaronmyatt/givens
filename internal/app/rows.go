package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"givens/internal/assert"
	"givens/internal/project"
	"givens/internal/runs"
)

// Row actions and the question before leaving a row with marks.
const (
	OfferSwitchRow = "Switch to this row"
	OfferLeave     = "Leave"
)

// outlineRow is one Examples row: its block's slug and its values in column order. The zero
// value is "not an outline row".
type outlineRow struct {
	examples string
	values   []runs.Pair
}

// rowsOf lists an outline's rows, block by block, in file order.
func rowsOf(scenario project.Scenario) []outlineRow {
	assert.That(scenario.Name != "", "the scenario has a name")
	assert.That(len(scenario.Examples) <= project.HelperFilesMax, "blocks stay bounded")
	rows := make([]outlineRow, 0, 4)
	for _, block := range scenario.Examples {
		for _, cells := range block.Rows {
			values := make([]runs.Pair, 0, len(block.Header))
			for column, name := range block.Header {
				if column < len(cells) {
					values = append(values, runs.Pair{Name: name, Value: cells[column]})
				}
			}
			rows = append(rows, outlineRow{examples: block.Slug, values: values})
		}
	}
	return rows
}

// fillPlaceholders puts a row's values in place of its <column> placeholders.
func fillPlaceholders(text string, row []runs.Pair) string {
	assert.That(len(row) <= runs.FieldsMax, "a row stays bounded")
	assert.That(len(text) <= NameLengthMax*64, "a step stays bounded")
	for _, value := range row {
		text = strings.ReplaceAll(text, "<"+value.Name+">", value.Value)
	}
	return text
}

// rowNamed finds the selected outline's row with value among its values, or "column=value".
func (w *Window) rowNamed(name string) (project.Scenario, outlineRow, bool) {
	assert.That(w != nil, "row named runs on a window")
	assert.That(name != "", "a row is named by a value")
	scenario, ok := w.findScenario(w.selectedScenario)
	if w.selectedScenario == "" || !ok || !scenario.Outline {
		return project.Scenario{}, outlineRow{}, false
	}
	for _, row := range rowsOf(scenario) {
		for _, value := range row.values {
			if value.Value == name || value.Name+"="+value.Value == name {
				return scenario, row, true
			}
		}
	}
	return project.Scenario{}, outlineRow{}, false
}

// DropInRows lists the Examples rows of the selected outline's Drop in page, one after
// another, in file order. Only one row is live at a time; the others are greyed out, with
// "Switch to this row" in place of "Drop in". Needed by the outline scenarios.
func (w *Window) DropInRows() []DropInRow {
	assert.That(w != nil, "drop in rows runs on a window")
	assert.That(w.app != nil, "a window belongs to an app")
	scenario, ok := w.findScenario(w.selectedScenario)
	if w.selectedScenario == "" || !ok || !scenario.Outline {
		return []DropInRow{}
	}
	live := w.session != nil && w.session.scenario.Key() == scenario.Key() &&
		w.session.row != nil
	shown := make([]DropInRow, 0, 4)
	for _, row := range rowsOf(scenario) {
		entry := DropInRow{
			Examples: row.examples, Values: make(map[string]string, len(row.values)),
			Steps:  make([]string, 0, len(scenario.Steps)),
			Status: rowStatus(w.project, scenario, row, w.app.config.Clock.Now()),
			Offers: []string{OfferDropIn}, Disabled: []string{}, Checklist: []CheckItem{},
		}
		for _, value := range row.values {
			entry.Values[value.Name] = value.Value
		}
		for _, step := range scenario.Steps {
			entry.Steps = append(entry.Steps, step.Keyword+" "+fillPlaceholders(step.Text,
				row.values))
		}
		if live && slices.Equal(w.session.row, row.values) {
			entry.Live, entry.Checklist = true, w.session.Checklist()
		} else if live {
			entry.Greyed, entry.Offers = true, []string{OfferSwitchRow}
			entry.Disabled = []string{OfferDropIn}
		}
		shown = append(shown, entry)
	}
	return shown
}

// ChooseOnRow takes an action on one Examples row of the selected outline: "Drop in" or
// "Switch to this row". row is one of the row's values, like "expired". Switching away from a
// row with marks asks first, since its marks aren't recorded. Needed by "Drop in to one row"
// and "Switch to another row".
func (w *Window) ChooseOnRow(row string, action string) error {
	assert.That(w != nil, "choose on row runs on a window")
	assert.That(row != "", "a row is named by one of its values")
	scenario, found, ok := w.rowNamed(row)
	if !ok {
		return fmt.Errorf("the selected scenario has no row %q: %w", row, ErrNotFound)
	}
	switch action {
	case OfferDropIn:
		_, err := w.startSession(scenario, found, false)
		return err
	case OfferSwitchRow:
		if w.session == nil || !w.session.marked() {
			_, err := w.startSession(scenario, found, false)
			return err
		}
		question := fmt.Sprintf("Leave the %s row? Its marks aren't recorded",
			rowName(w.session.row))
		w.ask(question, OfferLeave, func() error {
			_, err := w.startSession(scenario, found, false)
			return err
		})
		return nil
	}
	return fmt.Errorf("%q isn't offered on a row", action)
}

// rowName says a row as a person would: its first value, like "expired".
func rowName(row []runs.Pair) string {
	assert.That(len(row) <= runs.FieldsMax, "a row stays bounded")
	if len(row) == 0 {
		return ""
	}
	assert.That(row[0].Name != "", "a value has a column")
	return row[0].Value
}

// marked is true once any checklist item has a mark.
func (s *Session) marked() bool {
	assert.That(s != nil, "marked runs on a session")
	assert.That(len(s.checklist) <= SetupCardsMax*4, "the checklist stays bounded")
	for _, item := range s.checklist {
		if item.Mark != "" {
			return true
		}
	}
	return false
}

// rowStatus says where one outline row stands, as StatusOf does for a scenario.
func rowStatus(loaded *project.Project, scenario project.Scenario, row outlineRow,
	now time.Time) string {
	assert.That(loaded != nil, "status is read in a loaded project")
	assert.That(scenario.Outline, "only an outline has rows")
	return statusFor(loaded, scenario, runs.Key(scenario.Key(), row.examples), row.values, now)
}

// outlineStatus sums up an outline's rows: "Never run" until one has run, "Verified …" once
// all are, and "1 of 2 rows verified" between.
func outlineStatus(loaded *project.Project, scenario project.Scenario, now time.Time) string {
	assert.That(loaded != nil, "status is read in a loaded project")
	assert.That(scenario.Outline, "only an outline has rows")
	rows := rowsOf(scenario)
	verifiedRows, everRun, last := 0, 0, StatusNeverRun
	for _, row := range rows {
		status := rowStatus(loaded, scenario, row, now)
		if status != StatusNeverRun {
			everRun++
		}
		if strings.HasPrefix(status, statusVerified) {
			verifiedRows, last = verifiedRows+1, status
		}
	}
	switch {
	case everRun == 0:
		return StatusNeverRun
	case verifiedRows == len(rows):
		return last
	}
	return fmt.Sprintf("%d of %d rows verified", verifiedRows, len(rows))
}
