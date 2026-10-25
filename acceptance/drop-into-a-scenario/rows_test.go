package dropintoascenario

import (
	"encoding/csv"
	"errors"
	"fmt"
	"slices"
	"strings"

	"givens/acceptance/world"
	"givens/internal/app"
	"givens/internal/app/model"
)

// hasOutline writes the outline, and the named plain scenario if one is given, with setups.
func (s *state) hasOutline(other string) error {
	if err := s.writeOutline(true); err != nil {
		return err
	}
	if other != "" {
		if err := s.EnsureScenario("checkout", other); err != nil {
			return err
		}
		if err := s.writeSetup(other, script(`ok "second card added"`)); err != nil {
			return err
		}
	}
	return s.prepare()
}

// rows is the selected outline's Drop in rows.
func (s *state) rows() ([]model.DropInRow, error) {
	window, err := s.window()
	if err != nil {
		return nil, err
	}
	rows := window.DropInRows()
	if len(rows) == 0 {
		return nil, fmt.Errorf("the Drop in page lists no rows; it shows %q", window.Visible())
	}
	return rows, nil
}

// row is the row whose card column is card.
func (s *state) row(card string) (model.DropInRow, error) {
	rows, err := s.rows()
	if err != nil {
		return model.DropInRow{}, err
	}
	for _, row := range rows {
		if row.Values["card"] == card {
			return row, nil
		}
	}
	return model.DropInRow{}, fmt.Errorf("no row has card=%s: %+v", card, rows)
}

func (s *state) rowsListed(first string, second string) error {
	rows, err := s.rows()
	if err != nil {
		return err
	}
	if len(rows) != 2 || rows[0].Values["card"] != first || rows[1].Values["card"] != second {
		return fmt.Errorf("the rows aren't %q then %q: %+v", first, second, rows)
	}
	return nil
}

func (s *state) rowsFilledIn(example string) error {
	rows, err := s.rows()
	if err != nil {
		return err
	}
	for _, row := range rows {
		for _, step := range row.Steps {
			if strings.Contains(step, "<card>") || strings.Contains(step, "<message>") {
				return fmt.Errorf("the %v row shows an unfilled step %q", row.Values, step)
			}
		}
		want := "Then they see \"" + rowMessages()[row.Values["card"]] + "\""
		if !slices.Contains(row.Steps, want) {
			return fmt.Errorf("the %v row doesn't show %q: %q", row.Values, want, row.Steps)
		}
	}
	expired, err := s.row("expired")
	if err != nil {
		return err
	}
	return world.Expect(expired.Steps, example, "the expired row's steps")
}

func (s *state) rowsHaveStatus() error {
	rows, err := s.rows()
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Status == "" || !slices.Contains(row.Offers, "Drop in") {
			return fmt.Errorf("the %v row has no status or no \"Drop in\": %+v", row.Values, row)
		}
	}
	return nil
}

func (s *state) outlineOpen(name string) error {
	if err := s.writeOutline(true); err != nil {
		return err
	}
	if err := s.prepare(); err != nil {
		return err
	}
	return s.Window.Select(name)
}

func (s *state) chooseOnRow(action string, row string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	if err := window.ChooseOnRow(row, action); err != nil {
		return err
	}
	if action != "Drop in" {
		return nil
	}
	_, err = s.settleTo(app.SessionReady)
	return err
}

func (s *state) exampleCardIs(card string) error {
	if _, err := s.settleTo(app.SessionReady); err != nil {
		return err
	}
	got, ran, err := s.shopFile("ex-card")
	if err != nil {
		return err
	}
	if !ran || got != card {
		return fmt.Errorf("setup ran with GIVENS_EX_card=%q, not %q", got, card)
	}
	return nil
}

func (s *state) rowChecklistOpen() error {
	return s.rowChecklistLive("expired")
}

// rowSetAside expects the row set aside (Greyed) while another is live: its steps still
// shown, and "Drop in" shown but turned off.
func (s *state) rowSetAside(card string) error {
	row, err := s.row(card)
	if err != nil {
		return err
	}
	if !row.Greyed || len(row.Steps) == 0 {
		return fmt.Errorf("the %s row isn't set aside with its steps shown: %+v", card, row)
	}
	if !slices.Contains(row.Disabled, "Drop in") || slices.Contains(row.Offers, "Drop in") {
		return fmt.Errorf("the %s row can still be dropped in to: %+v", card, row)
	}
	return nil
}

// rowMarkedOne drops in to one row of the outline and marks its first Then step "pass".
func (s *state) rowMarkedOne(card string) error {
	if err := s.outlineOpen(outlineName); err != nil {
		return err
	}
	if err := s.chooseOnRow("Drop in", card); err != nil {
		return err
	}
	session, err := s.session()
	if err != nil {
		return err
	}
	first, err := firstOfKind(session.Checklist(), "then")
	if err != nil {
		return err
	}
	return session.Mark(first.Text, "pass")
}

func (s *state) asked(question string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	if err := world.Expect(window.Visible(), question, "the window"); err != nil {
		return err
	}
	return world.Expect(window.Offers(), "Leave", "the offered actions")
}

func (s *state) leaveSwitches(from string, to string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	if err := window.Choose("Leave"); err != nil {
		return err
	}
	if err := s.exampleCardIs(to); err != nil {
		return err
	}
	left, err := s.row(from)
	if err != nil {
		return err
	}
	if left.Live || len(left.Checklist) > 0 {
		return fmt.Errorf("the %s checklist is still open: %+v", from, left)
	}
	return s.rowChecklistLive(to)
}

// rowChecklistLive expects the row to be live, with a checklist for its own values.
func (s *state) rowChecklistLive(card string) error {
	row, err := s.row(card)
	if err != nil {
		return err
	}
	texts := make([]string, 0, len(row.Checklist))
	for _, item := range row.Checklist {
		texts = append(texts, item.Text)
	}
	if !row.Live {
		return fmt.Errorf("the %s row isn't live: %+v", card, row)
	}
	return world.Expect(texts, rowMessages()[card], "the "+card+" row's checklist")
}

func (s *state) rowAllPassed(name string, card string) error {
	if err := s.outlineOpen(name); err != nil {
		return err
	}
	return s.rowPassed(card)
}

// rowPassed drops in to one row of the selected outline and marks every check.
func (s *state) rowPassed(card string) error {
	if err := s.chooseOnRow("Drop in", card); err != nil {
		return err
	}
	session, err := s.session()
	if err != nil {
		return err
	}
	return markAll(session)
}

func (s *state) runLineWithValue(key string, column string, value string) error {
	lines, err := s.runLines(key)
	if err != nil {
		return err
	}
	if fields := lines[len(lines)-1]; !hasField(fields, column+"="+value) {
		return fmt.Errorf("the run line for %s doesn't hold %s=%s: %q", key, column, value,
			fields)
	}
	return nil
}

func (s *state) scenarioShows(text string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	scenario, err := window.Scenario(outlineName)
	if err != nil {
		return err
	}
	return world.Expect([]string{scenario.Status}, text, outlineName+"'s status")
}

func (s *state) unverifiedLists(card string, status string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	if err := window.Select("Unverified"); err != nil {
		return err
	}
	for _, line := range window.Visible() {
		if strings.Contains(line, card) && strings.Contains(line, status) {
			return nil
		}
	}
	return fmt.Errorf("the Unverified view doesn't list %q as %q: %q", card, status,
		window.Visible())
}

// cartTable is the data table the Pay with saved card Given step gets.
const cartTable = `    Given a signed-in shopper with these items in the cart:
      | item   | price |
      | Mug    | 12.00 |
      | Poster | 8.50  |`

// dataTable gives Pay with saved card's Given step a table of two cart items, and a setup that
// keeps a copy of GIVENS_ARG_1 and seeds a cart from it.
func (s *state) dataTable() error {
	if err := s.CopySampleShop(); err != nil {
		return err
	}
	text, err := s.ReadFile(checkoutFeature)
	if err != nil {
		return err
	}
	scenario := "  Scenario: " + payWithSavedCard + "\n"
	before, after, found := strings.Cut(text, scenario)
	given, rest, hasGiven := strings.Cut(after, "\n")
	if !found || !hasGiven || !strings.HasPrefix(strings.TrimSpace(given), "Given") {
		return fmt.Errorf("checkout.feature has no %q with a Given first", payWithSavedCard)
	}
	edited := before + scenario + cartTable + "\n" + rest
	if err := s.WriteFile(checkoutFeature, edited); err != nil {
		return err
	}
	err = s.writeSetup(payWithSavedCard, script(
		`printf '%s' "$GIVENS_ARG_1" > `+shopData+`/arg-1-path"`,
		`cp "$GIVENS_ARG_1" `+shopData+`/arg-1.csv"`,
		`tail -n +2 "$GIVENS_ARG_1" | while IFS=, read -r item _ || [ -n "$item" ]; do `+
			`echo "$item" >> `+shopData+`/cart"; done`,
		`ok "cart seeded"`))
	if err != nil {
		return err
	}
	return s.prepare()
}

func (s *state) argIsCSV() error {
	if _, err := s.settleTo(app.SessionReady); err != nil {
		return err
	}
	path, _, err := s.shopFile("arg-1-path")
	if err != nil || path == "" {
		return fmt.Errorf("GIVENS_ARG_1 was empty or unset (%v)", err)
	}
	copied, _, err := s.shopFile("arg-1.csv")
	if err != nil {
		return err
	}
	// encoding/csv reads RFC 4180 CSV. Ref: https://pkg.go.dev/encoding/csv
	records, err := csv.NewReader(strings.NewReader(copied)).ReadAll()
	if err != nil {
		return fmt.Errorf("%s isn't CSV: %w", path, err)
	}
	want := [][]string{{"item", "price"}, {"Mug", "12.00"}, {"Poster", "8.50"}}
	if !slices.EqualFunc(records, want, slices.Equal[[]string]) {
		return fmt.Errorf("%s holds %q, not %q", path, records, want)
	}
	return nil
}

func (s *state) cartSeeded() error {
	cart, _, err := s.shopFile("cart")
	if err != nil {
		return err
	}
	if cart != "Mug\nPoster\n" {
		return fmt.Errorf("the setup seeded the cart with %q, not Mug and Poster", cart)
	}
	return nil
}

func (s *state) unnamedExamples(name string) error {
	if name != outlineName {
		return fmt.Errorf("only %q has Examples blocks to unname, not %q", outlineName, name)
	}
	if err := s.writeOutline(false); err != nil {
		return err
	}
	return s.prepare()
}

func (s *state) listedWithWarning(text string) error {
	window, err := s.window()
	if err != nil {
		return err
	}
	scenario, err := window.Scenario(outlineName)
	if err != nil {
		return err
	}
	return world.Expect([]string{scenario.Warning}, text, outlineName+"'s warning")
}

// recordOffUntilNamed drops in to a row and marks every check, expecting no "Record run";
// then names the Examples blocks and expects the warning to clear and "Record run" to return.
func (s *state) recordOffUntilNamed() error {
	window, err := s.window()
	if err != nil {
		return err
	}
	if err := window.Select(outlineName); err != nil {
		return err
	}
	if err := s.rowPassed("expired"); err != nil {
		return err
	}
	if slices.Contains(window.Offers(), "Record run") {
		return fmt.Errorf("\"Record run\" is offered with the Examples blocks unnamed: %q",
			window.Offers())
	}
	if err := s.writeOutline(true); err != nil {
		return err
	}
	named := eventually(func() bool {
		scenario, err := window.Scenario(outlineName)
		return err == nil && scenario.Warning == ""
	})
	if !named {
		return errors.New("the warning stayed after the Examples blocks were named")
	}
	if err := s.rowPassed("expired"); err != nil {
		return err
	}
	return world.Expect(window.Offers(), "Record run", "the offered actions once named")
}
