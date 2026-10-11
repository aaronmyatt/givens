package project

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// TestLoadSampleShop loads testdata/sample-shop, the project most scenarios start from.
func TestLoadSampleShop(t *testing.T) {
	loaded, err := Load(filepath.Join("..", "..", "testdata", "sample-shop", "capabilities"))
	if err != nil {
		t.Fatal(err)
	}
	slugs := make([]string, 0, len(loaded.Capabilities))
	for _, capability := range loaded.Capabilities {
		slugs = append(slugs, capability.Slug)
	}
	if want := []string{"checkout", "invites", "sign-in"}; !slices.Equal(slugs, want) {
		t.Fatalf("capabilities = %q, want %q", slugs, want)
	}
	checkout := loaded.Capabilities[0]
	if checkout.Name != "Checkout" || len(checkout.Story) != 3 || checkout.About == "" {
		t.Errorf("checkout = %q, story %q, about %q", checkout.Name, checkout.Story, checkout.About)
	}
	if len(checkout.Helpers) != 2 || len(loaded.Helpers) == 0 {
		t.Errorf("helpers: checkout %q, root %q", checkout.Helpers, loaded.Helpers)
	}
	declined, err := loaded.Find("checkout/card-declined")
	if err != nil {
		t.Fatal(err)
	}
	if !declined.Error || declined.StillRunning != 5*time.Minute || declined.Setup == "" {
		t.Errorf("card declined = %+v", declined)
	}
	if declined.Key() != "checkout/card-declined" || len(declined.Steps) != 4 {
		t.Errorf("card declined key %q, steps %+v", declined.Key(), declined.Steps)
	}
	if _, err := loaded.Find("checkout/pay-with-card"); err == nil {
		t.Error("found a scenario that doesn't exist")
	}
	if len(loaded.Problems) != 0 {
		t.Errorf("the sample shop has problems: %v", loaded.Problems)
	}
}

const rulesFeature = `@billing
Feature: Billing
  As a shop owner
  I want invoices
  So that I get paid

  Background:
    Given a shop

  Scenario: Plain
    Given nothing
    When it runs
    Then it works

  @slow
  Rule: Refunds
    Background:
      Given a paid order

    @timeout:90s @error
    Scenario: Refund twice
      When they refund it twice
      Then they see "Already refunded"
        """
        a doc string
        """

    Scenario Outline: Refund <amount>
      When they refund <amount>
      Then the balance is <left>

      Examples: Small
        | amount | left |
        | 1      | 9    |

    @timeout:1h
    Scenario: Bad timeout
      Then nothing
`

// TestLoadRulesBackgroundsAndOutlines covers what the sample shop doesn't use.
func TestLoadRulesBackgroundsAndOutlines(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "billing", "billing.feature"), rulesFeature)
	write(t, filepath.Join(root, "billing", "scripts", "plain.md"), "Do it by hand.\n")
	write(t, filepath.Join(root, "notes", "readme.txt"), "not a capability\n")
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Capabilities) != 1 {
		t.Fatalf("capabilities = %+v, want only billing", loaded.Capabilities)
	}
	scenarios := loaded.Capabilities[0].Scenarios
	if len(scenarios) != 4 {
		t.Fatalf("scenarios = %+v, want 4", scenarios)
	}
	plain, twice, outline, bad := scenarios[0], scenarios[1], scenarios[2], scenarios[3]
	if len(plain.Background) != 1 || plain.SetupCards == "" || plain.Setup != "" {
		t.Errorf("plain = %+v", plain)
	}
	if len(twice.Background) != 2 || !twice.Error || twice.StillRunning != 90*time.Second {
		t.Errorf("refund twice = %+v", twice)
	}
	if want := []string{"@timeout:90s", "@error", "@slow", "@billing"}; !slices.Equal(
		twice.Tags, want) {
		t.Errorf("refund twice tags = %q, want %q", twice.Tags, want)
	}
	if twice.Steps[1].DocString != "a doc string" {
		t.Errorf("doc string = %q", twice.Steps[1].DocString)
	}
	if !outline.Outline || outline.Slug != "refund-amount" || outline.Examples[0].Slug != "small" {
		t.Errorf("outline = %+v", outline)
	}
	if got := outline.Examples[0].Rows; len(got) != 1 || got[0][1] != "9" {
		t.Errorf("outline rows = %q", got)
	}
	if bad.StillRunning != 0 {
		t.Errorf("an unreadable timeout gave %v, want 0", bad.StillRunning)
	}
}

// TestLoadBrokenFeature keeps a capability whose file isn't valid Gherkin, with why.
func TestLoadBrokenFeature(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "broken", "broken.feature"), "Scenario: Orphan\n  Given oops\n")
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Capabilities) != 1 || loaded.Capabilities[0].ParseError == "" {
		t.Fatalf("capabilities = %+v, want broken with a parse error", loaded.Capabilities)
	}
	if len(loaded.Problems) == 0 {
		t.Error("a broken file gave no problems")
	}
}

// write writes content to path, making its folders.
func write(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
