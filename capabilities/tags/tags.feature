Feature: Tags
  As a developer, or an agent, marking scenarios
  I want kinds, timeouts and groups written as Gherkin tags
  So that a test runner can filter on the same tags Givens uses

  Search and `givens list --tags` take the same tag expressions as Cucumber runners, so a
  filter that works in one works in the other. Each tag is a view: in the TUI, the Tags tab
  lists them, and Enter on one lists its scenarios. Cucumber's tag expressions:
  https://cucumber.io/docs/cucumber/api/#tag-expressions

  Scenario: An error state
    Given "Card declined" is tagged @error in checkout.feature
    When a developer selects the "Error states" view
    Then "Card declined" is listed, marked as an error state

  Scenario: A tag view
    Given "Card declined" and "Pay with saved card" are both tagged @payments
    When a developer selects "payments" on the Tags tab
    Then only those two scenarios are listed

  Scenario: A tag on the Feature
    Given the Feature: Checkout line is tagged @payments
    When a developer selects "payments" on the Tags tab
    Then every Checkout scenario is listed

  Scenario: A runner filters on the same tags
    Given a developer has the sample shop project
    When they run "npx cucumber-js --dry-run --tags @error capabilities/**/*.feature"
    Then cucumber-js finds the same scenarios the Error states view lists

  Scenario: Search by tag
    Given "Card declined" and "Pay with saved card" are both tagged @payments
    When a developer searches for "@payments"
    Then only those two scenarios are listed under Scenarios
    And a scenario that gets @payments from its Feature line is listed too

  Scenario: Search with a tag expression
    Given "Card declined" is tagged @error and "3-D Secure failed" is tagged @error and @slow
    When a developer searches for "@error and not @slow"
    Then "Card declined" is listed
    And "3-D Secure failed" isn't

  Scenario: A tag and words together
    Given "Card declined" and "Pay with saved card" are both tagged @payments
    When a developer searches for "@payments declined"
    Then only "Card declined" is listed, with "declined" marked as the match

  @error
  Scenario: A tag expression that doesn't parse
    Given a developer has the sample shop project open in Givens
    When they search for "@error and"
    Then they see "Not a tag expression: expected a tag after 'and'"
    And no results are listed until the expression is complete

  # @todo flags a scenario that needs work: not built yet, failing, or not testable. Givens
  # flags it wherever it shows it, and like any other tag it's a view. A runner can skip
  # flagged scenarios with "not @todo". `givens todo` and `givens done` flag and unflag one
  # from the terminal.
  Scenario: A scenario flagged to do
    Given "Empty cart" is tagged @todo in checkout.feature
    When a developer selects "Checkout"
    Then "Empty cart" is shown flagged to do
    And "todo" is listed on the Tags tab

  Scenario: Flag a scenario to do
    Given a developer has the sample shop project open in Givens
    And "Pay with saved card" is selected
    When they choose "Flag as to do"
    Then "Pay with saved card" is tagged @todo in checkout.feature, and nothing else changes
    And it's listed when they select "todo" on the Tags tab

  Scenario: Remove a to do flag
    Given "Empty cart" is tagged @todo in checkout.feature
    And "Empty cart" is selected
    When a developer chooses "Remove to do flag"
    Then "Empty cart" isn't tagged @todo in checkout.feature, and nothing else changes
    And it isn't shown flagged to do
