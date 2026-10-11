Feature: Share setup between scenarios
  As a developer writing setup scripts
  I want helpers that every scenario, or every scenario in one capability, can call
  So that each setup script stays a few readable lines and shared setup changes in one place

  `givens.sh` loads `capabilities/_shared/*.sh` first, then the calling script's own
  `<capability>/_shared/*.sh`. A capability's helpers are only visible to that capability's
  scripts, and override a shared helper with the same name.

  The "Shared" section of the sidebar lists every helper documented with a `# @helper` comment,
  where it's defined and which scripts call it. A run's setup fingerprint covers the helpers a
  script calls, so changing a helper marks the scenarios that use it as unverified, and only
  those.

  @error
  Scenario: Calling another capability's helper
    Given a developer has the sample shop project open in Givens
    And the "Magic link" setup calls mock_payments, which only Checkout's helpers define
    When they drop in to "Magic link"
    Then setup fails with "mock_payments: command not found"
    And Givens points out that mock_payments is defined in checkout/_shared/payments.sh

  Scenario: Two helpers with the same name
    Given a developer has the sample shop project open in Givens
    And Checkout's helpers define a seed_shopper of their own
    When they drop in to "Card declined"
    Then Checkout's seed_shopper runs in place of the shared one
    And no warning is shown
    And the Shared list marks it "Overrides _shared/users.sh"

  @error
  Scenario: Two shared files define the same helper
    Given a developer has the sample shop project open in Givens
    And two files in _shared define seed_shopper
    When they drop in to "Card declined"
    Then the output warns "seed_shopper is defined in both _shared/users.sh and _shared/people.sh"
    And both definitions are flagged in the Shared list

  Scenario: Changing a helper unverifies the scenarios that call it
    Given "Card declined" and "Accept invite" both have a passing run recorded
    When seed_shopper changes on disk
    Then "Card declined" shows "Unverified: seed_shopper changed since the last run"
    And "Accept invite" still shows as verified

  # Accept invite doesn't call seed_shopper, so its setup fingerprint doesn't change.
  #
  # The fingerprint, a short code worked out from file contents, covers the scenario's own block
  # in its `.feature` file (tags, steps and its Examples row), its setup files (`.sh`, `.md` and
  # teardown), every setup it builds on through `from`, and every helper it calls, along with
  # the helpers those helpers call. It doesn't cover the Feature description, `about.md` or the
  # kit, so updating givens.sh unverifies nothing.
  Scenario: Changing a step unverifies the scenario
    Given "Card declined" has a passing run recorded
    When "the order stays pending" is changed to "the order is marked failed"
    Then "Card declined" shows "Unverified: steps changed since the last run"

  Scenario: Changing a setup unverifies the scenarios built on it
    Given "3-D Secure challenge" and "3-D Secure failed" both have passing runs recorded
    When the "Pay with saved card" setup script changes
    Then both show "Unverified: checkout/pay-with-saved-card setup changed since the last run"

  Scenario: Changing a nested helper unverifies its callers
    Given seed_shopper calls seed_address
    And "Card declined" has a passing run recorded
    When seed_address changes on disk
    Then "Card declined" shows "Unverified: seed_address changed since the last run"

  Scenario: Updating the kit keeps scenarios verified
    Given "Card declined" has a passing run recorded
    When the sample shop's kit is updated from version 1 to 2
    Then "Card declined" still shows as verified

  Scenario: See what a scenario needs installed
    Given a developer has the sample shop project open in Givens
    And "Pay with saved card" calls seed_catalogue, which runs _shared/catalogue.clj with bb
    When they select "Pay with saved card"
    Then they see "Requires bb"
    And if bb isn't installed, it's marked "Not installed" with its install hint

  Scenario: Edit a helper
    Given a developer has the sample shop project open in Givens
    When they select seed_shopper under Shared and choose "Edit"
    Then _shared/users.sh opens in the editor at seed_shopper
    And the 5 scripts that call it are listed beside the editor

  Scenario: Add a helper
    Given a developer has the sample shop project open in Givens
    When they choose "New helper" in Checkout and name it "add_coupon"
    Then checkout/_shared/add_coupon.sh is created with a # @helper comment and an empty add_coupon function
    And add_coupon is listed under Shared as one of Checkout's helpers

  @error
  Scenario: Save a helper with a syntax error
    Given a developer is editing seed_shopper in Givens
    When they save it with its closing brace missing
    Then the line bash reports is marked with its syntax error
    And the file is saved anyway
